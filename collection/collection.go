package collection

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"sync"

	"github.com/RICE-COMP318-FALL23/owldb-p1group20/document"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

// A putoutput stores the response to a put request.
type putoutput struct {
	Uri string `json:"uri"`
}

/*
A collection is a concurrent skip list of documents,
which is sorted by document name.
*/
type Collection struct {
	Documents *sync.Map
}

// Creates a new collection.
func New() Collection {
	return Collection{&sync.Map{}}
}

// Gets a collection
func (c Collection) CollectionGet(w http.ResponseWriter, r *http.Request) {
	// we can use the query method once we've written
	returnDocs := make([]document.Docoutput, 0)

	// Add each docoutput to the docoutputs list
	c.Documents.Range(func(key, value interface{}) bool {
		returnDocs = append(returnDocs, value.(document.Document).Output)
		return true
	})

	// Convert to JSON and send
	jsonToDo, err := json.Marshal(returnDocs)
	if err != nil {
		// This should never happen
		slog.Error("Get: error marshaling", "error", err)
		http.Error(w, `"internal server error"`, http.StatusInternalServerError)
		return
	}
	w.Write(jsonToDo)
	slog.Info("GET: success")
}

// Puts a document into a collection
func (c Collection) DocumentPut(w http.ResponseWriter, r *http.Request, path string, schema *jsonschema.Schema) {

	// Read body of requests
	desc, err := io.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		slog.Error("Put document: error reading the document request body", "error", err)
		http.Error(w, `"invalid document format"`, http.StatusBadRequest)
		return
	}

	// Read Body data
	var docBody map[string]interface{}
	err = json.Unmarshal(desc, &docBody)
	if err != nil {
		slog.Error("createReplaceDocument: error unmarshaling Put document request", "error", err)
		http.Error(w, `"invalid Put document format"`, http.StatusBadRequest)
		return
	}

	// Validate against schema
	err = schema.Validate(docBody)
	if err != nil {
		slog.Error("Put document: document did not conform to schema", "error", err)
		http.Error(w, `"document did not conform to schema"`, http.StatusBadRequest)
		return
	}

	// Either modify or create a new document
	existingDoc, exists := c.Documents.Load(path)
	if exists {
		jsonResponse, err := json.Marshal(putoutput{r.URL.Path})
		if err != nil {
			// This should never happen
			slog.Error("Get: error marshaling", "error", err)
			http.Error(w, `"internal server error"`, http.StatusInternalServerError)
			return
		}
		// Need to modify metadata
		var modifiedDoc = existingDoc.(document.Document)
		modifiedDoc.Overwrite(docBody)
		c.Documents.Store(path, modifiedDoc)

		slog.Info("Overwrote an old document", "path", path)
		w.Header().Set("Location", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		w.Write(jsonResponse)
	} else {
		jsonResponse, err := json.Marshal(putoutput{r.URL.Path})
		if err != nil {
			// This should never happen
			slog.Error("Get: error marshaling", "error", err)
			http.Error(w, `"internal server error"`, http.StatusInternalServerError)
			return
		}
		// Create a new document
		doc := document.New("/"+path, "DUMMY USER", docBody)

		c.Documents.Store(path, doc)
		slog.Info("Created new document", "path", path)
		w.Header().Set("Location", r.URL.Path)
		w.WriteHeader(http.StatusCreated)
		w.Write(jsonResponse)
	}

}

// Deletes a document from this collection
func (c Collection) DocumentDelete(w http.ResponseWriter, r *http.Request, docpath string) {
	// Access the document
	_, exist := c.Documents.Load(docpath)
	if exist {
		// Document found
		c.Documents.Delete(docpath)
		slog.Info("Deleted Document", "path", r.URL.Path)
		w.Header().Set("Location", r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
		return
	} else {
		// Document not found
		slog.Error("Document does not exist")
		http.Error(w, "Document does not exist", http.StatusNotFound)
		return
	}
}
