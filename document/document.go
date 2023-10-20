// Package document implements the document functionality
// as specified in the owlDB api. Includes several structs
// and methods for manipulating a document.
package document

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/RICE-COMP318-FALL23/owldb-p1group20/collectionholder"
	"github.com/RICE-COMP318-FALL23/owldb-p1group20/errorMessage"
	"github.com/RICE-COMP318-FALL23/owldb-p1group20/interfaces"
	"github.com/RICE-COMP318-FALL23/owldb-p1group20/patcher"
	"github.com/RICE-COMP318-FALL23/owldb-p1group20/structs"
	"github.com/RICE-COMP318-FALL23/owldb-p1group20/subscribe"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

// A meta stores metadata about a document.
type meta struct {
	CreatedBy      string `json:"createdBy"`      // The user who created this JSON document.
	CreatedAt      int64  `json:"createdAt"`      // The time this JSON document was created.
	LastModifiedBy string `json:"lastModifiedBy"` // The last user who modified this JSON document.
	LastModifiedAt int64  `json:"lastModifiedAt"` // The last time that this JSON document was modified.
}

/*
A docoutput is a struct which represents the data to
be output when a user requests a given document.
*/
type docoutput struct {
	Path string      `json:"path"` // The relative path to this document.
	Doc  interface{} `json:"doc"`  // The actual JSON document represented by this object.
	Meta meta        `json:"meta"` // The metadata of this document.
}

// A document is a document plus a concurrent
// skip list of collections, and a slice of subscribers.
type Document struct {
	output      docoutput                          // The document held in this object with extra meta data.
	children    *collectionholder.CollectionHolder // The set of collections this document holds.
	subscribers []subscribe.Subscriber             // A slice of subscribers to this document.
}

// Creates a new document.
func New(path, user string, docBody interface{}) Document {
	newH := collectionholder.New()
	return Document{newOutput(path, user, docBody), &newH, make([]subscribe.Subscriber, 0)}
}

// Create a new docoutput
func newOutput(path, user string, docBody interface{}) docoutput {
	return docoutput{path, docBody, newMeta(user)}
}

// Create a new metadata
func newMeta(user string) meta {
	return meta{user, time.Now().UnixMilli(), user, time.Now().UnixMilli()}
}

// Handles a GET request that has a path pointing to this document.
func (d *Document) GetDocument(w http.ResponseWriter, r *http.Request) {
	// Convert to JSON and send
	jsonDoc, err := d.GetJSONBody()
	if err != nil {
		errorMessage.ErrorResponse(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Subscribe mode
	mode := r.URL.Query().Get("mode")
	if mode == "subscribe" {
		subscriber := subscribe.New()
		d.subscribers = append(d.subscribers, subscriber)
		go func() {
			d.NotifySubscribersUpdate(jsonDoc, "")
		}()
		subscriber.ServeSubscriber(w, r)
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonDoc)
		slog.Info("GET: success")
	}
}

// Handles a PUT request that has a path pointing to this document.
func (d *Document) PutCollection(w http.ResponseWriter, r *http.Request, newName string, newColl interfaces.ICollection) {
	d.children.PutCollection(w, r, newName, newColl)
}

// Handles a DELETE on a collection in this document.
func (d *Document) DeleteCollection(w http.ResponseWriter, r *http.Request, newName string) {
	d.children.DeleteCollection(w, r, newName)
}

// Finds a collection in this document for other methods.
func (d *Document) GetCollection(resource string) (interfaces.ICollection, bool) {
	return d.children.GetCollection(resource)
}

// Overwrite the body of a document upon recieving a put or patch.
func (d *Document) OverwriteBody(docBody interface{}, name string) {
	existingDocOutput := d.output
	existingDocOutput.Meta.LastModifiedAt = time.Now().UnixMilli()
	existingDocOutput.Meta.LastModifiedBy = name

	// Modify document contents
	existingDocOutput.Doc = docBody

	// Modify it again in the doc
	d.output = existingDocOutput

	// Wipes the children of this document
	newChildren := collectionholder.New()
	d.children = &newChildren
}

// Applys a slice of patches to this document.
// Returns a PatchResponse without the Uri field
// set, expecting it to be set by caller.
func (d *Document) ApplyPatches(patches []patcher.Patch, schema *jsonschema.Schema) (structs.PatchResponse, interface{}) {
	slog.Info("Applying patch to document", "path", d.output.Path)
	var ret structs.PatchResponse
	var err error

	// Iterate over slice of patches and apply them to the docbody each time.
	newdoc := d.output.Doc
	for i, patch := range patches {
		newdoc, err = patcher.ApplyPatch(newdoc, patch)

		slog.Debug("Patching", "patched doc", newdoc)

		if err != nil {
			slog.Info("Patch failed", "num", i)
			str := fmt.Sprintf("Error applying patches: %s", err.Error())
			ret.Message = str
			ret.PatchFailed = true
			return ret, nil
		}
	}

	// Validates document against schema after patches have been applied.
	err = schema.Validate(newdoc)
	if err != nil {
		slog.Error("Patch document: patched document did not conform to schema", "error", err)
		str := fmt.Sprintf("Patched document did not conform to schema: %s", err.Error())
		ret.Message = str
		ret.PatchFailed = true
		return ret, nil
	}

	// Successfully applied all the patches.
	ret.Message = "patches applied"
	ret.PatchFailed = false
	return ret, newdoc
}

// Gets the last modified at field from
// this document for conditional put.
func (d *Document) GetLastModified() int64 {
	return d.output.Meta.LastModifiedAt
}

// Gets the original author of this document
func (d *Document) GetOriginalAuthor() string {
	return d.output.Meta.CreatedBy
}

// Gets the JSON Object that this document stores.
func (d *Document) GetJSONBody() ([]byte, error) {
	jsonBody, err := json.Marshal(d.output)
	if err != nil {
		// This should never happen
		slog.Error("Error marshalling doc body", "error", err)
		return nil, err
	}

	return jsonBody, err
}

// Gets the JSON Object that this document stores.
func (d *Document) GetRawBody() interface{} {
	return d.output
}

// Gets the JSON Document that this document stores.
func (d *Document) GetJSONDoc() interface{} {
	return d.output.Doc
}

// Implements Subscribable method. Notifies subscribers of update messages.
// Does not use interval.
func (d *Document) NotifySubscribersUpdate(msg []byte, intervalComp string) {
	for _, sub := range d.subscribers {
		sub.UpdateCh <- msg
	}
}

// Implements Subscribable method. Notifies subscribers of update messages.
// Does not use interval.
func (d *Document) NotifySubscribersDelete(msg string, intervalComp string) {
	for _, sub := range d.subscribers {
		sub.DeleteCh <- msg
	}
}
