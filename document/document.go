// Package document implements the document functionality
// as specified in the owlDB api. Includes several structs
// and methods for manipulating a document.
package document

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/RICE-COMP318-FALL23/owldb-p1group20/patcher"
)

// A meta stores metadata about a document.
type meta struct {
	CreatedBy      string `json:"createdBy"`
	CreatedAt      int64  `json:"createdAt"`
	LastModifiedBy string `json:"lastModifiedBy"`
	LastModifiedAt int64  `json:"lastModifiedAt"`
}

func newMeta(user string) meta {
	return meta{user, time.Now().UnixMilli(), user, time.Now().UnixMilli()}
}

/*
A docoutput is a struct which represents the data to
be output when a user requests a given document.
*/
type Docoutput struct {
	Path string                 `json:"path"`
	Doc  map[string]interface{} `json:"doc"`
	Meta meta                   `json:"meta"`
}

func newOutput(path, user string, docBody map[string]interface{}) Docoutput {
	return Docoutput{path, docBody, newMeta(user)}
}

// A document is a document plus a concurrent skip list of collections
type Document struct {
	Output   Docoutput
	Children *sync.Map
}

// Creates a new document.
func New(path, user string, docBody map[string]interface{}) Document {
	return Document{newOutput(path, user, docBody), &sync.Map{}}
}

// Overwrite the body of a document upon recieving a put.
func (d *Document) Overwrite(docBody map[string]interface{}) {
	existingDocOutput := d.Output
	existingDocOutput.Meta.LastModifiedAt = time.Now().UnixMilli()
	existingDocOutput.Meta.LastModifiedBy = "DUMMY USER"

	// Modify document contents
	existingDocOutput.Doc = docBody

	// Modify it again in the doc
	d.Output = existingDocOutput
}

// Gets a document
func (d Document) DocumentGet(w http.ResponseWriter, r *http.Request) {
	// Convert to JSON and send
	jsonDoc, err := json.Marshal(d.Output)

	if err != nil {
		// This should never happen
		slog.Error("Get: error marshaling", "error", err)
		http.Error(w, `"internal server error"`, http.StatusInternalServerError)
		return
	}

	w.Write(jsonDoc)
	slog.Info("GET: success")
}

type PatchResponse struct {
	Uri         string `json:"uri"`
	PatchFailed bool   `json:"patchFailed"`
	Message     string `json:"message"`
}

// Applys a slice of patches to this document.
// Returns a PatchResponse without the Uri field
// set, expecting it to be set by caller.
func (d *Document) ApplyPatches(patches []patcher.Patch) PatchResponse {
	var ret PatchResponse
	var ok bool
	var err error

	newdoc := d.Output.Doc
	for _, patch := range patches {
		newdoc, ok, err = patcher.ApplyPatch(newdoc, patch)
		if !ok {
			str := fmt.Sprintf("Error applying patches: %s", err.Error())
			ret.Message = str
			ret.PatchFailed = true
			return ret
		}
	}

	d.Output.Doc = newdoc

	// Successfully applied all the patches.
	ret.Message = "patch applied"
	ret.PatchFailed = false
	return ret
}
