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
	"github.com/RICE-COMP318-FALL23/owldb-p1group20/interfaces"
	"github.com/RICE-COMP318-FALL23/owldb-p1group20/patcher"
	"github.com/RICE-COMP318-FALL23/owldb-p1group20/structs"
	"github.com/RICE-COMP318-FALL23/owldb-p1group20/subscribe"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

// A meta stores metadata about a document.
type meta struct {
	CreatedBy      string `json:"createdBy"`
	CreatedAt      int64  `json:"createdAt"`
	LastModifiedBy string `json:"lastModifiedBy"`
	LastModifiedAt int64  `json:"lastModifiedAt"`
}

/*
A docoutput is a struct which represents the data to
be output when a user requests a given document.
*/
type docoutput struct {
	Path string      `json:"path"`
	Doc  interface{} `json:"doc"`
	Meta meta        `json:"meta"`
}

// A document is a document plus a concurrent skip list of collections
type Document struct {
	output      docoutput
	children    *collectionholder.CollectionHolder
	subscribers []subscribe.Subscriber
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

// Gets a document
func (d *Document) DocumentGet(w http.ResponseWriter, r *http.Request) {
	// Convert to JSON and send
	jsonDoc, err := d.GetJSONBody()
	if err != nil {
		http.Error(w, `"internal server error"`, http.StatusInternalServerError)
		return
	}

	// Subscribe mode
	mode := r.URL.Query().Get("mode")
	if mode == "subscribe" {
		subscriber := subscribe.New()
		d.subscribers = append(d.subscribers, subscriber)
		w.Header().Set("Content-Type", "text/event-stream")
		go subscriber.ServeHTTP(w, r)
		subscriber.UpdateCh <- jsonDoc
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonDoc)
	slog.Info("GET: success")
}

// Puts a new collection in this document.
func (d *Document) CollectionPut(w http.ResponseWriter, r *http.Request, newName string) {
	d.children.CollectionPut(w, r, newName)
}

// Deletes a collection in this document.
func (d *Document) CollectionDelete(w http.ResponseWriter, r *http.Request, newName string) {
	d.children.CollectionDelete(w, r, newName)
}

// Finds a collection in this document
func (d *Document) CollectionFind(resource string) (interfaces.ICollection, bool) {
	return d.children.CollectionFind(resource)
}

// Overwrite the body of a document upon recieving a put.
func (d *Document) Overwrite(docBody interface{}, name string) {
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
func (d *Document) GetDoc() interface{} {
	return d.output.Doc
}

// Gets the subscribers to this document.
func (d *Document) GetSubscribers() []subscribe.Subscriber {
	return d.subscribers
}
