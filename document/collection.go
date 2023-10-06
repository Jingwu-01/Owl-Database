package document

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/RICE-COMP318-FALL23/owldb-p1group20/patcher"
	"github.com/RICE-COMP318-FALL23/owldb-p1group20/pathprocessor"
	"github.com/RICE-COMP318-FALL23/owldb-p1group20/skiplist"
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
	Documents *skiplist.SkipList[string, *Document]
}

// Creates a new collection.
// TODO: should this be a pointer instead?
func NewCollection() Collection {
	newSL := skiplist.New[string, *Document](skiplist.STRING_MIN, skiplist.STRING_MAX, skiplist.DEFAULT_LEVEL)
	return Collection{&newSL}
}

// Gets a collection of documents
func (c *Collection) CollectionGet(w http.ResponseWriter, r *http.Request) {
	// Get queries
	queries := r.URL.Query()
	interval := pathprocessor.GetInterval(queries.Get("interval"))

	// Build a list of document outputs
	returnDocs := make([]Docoutput, 0)

	// Make query on collection
	pairs, err := c.Documents.Query(r.Context(), interval[0], interval[1])

	if err != nil {
		// TODO: type of error?
		slog.Info("Collection could not retrieve query in time")
		return
	}

	for _, pair := range pairs {
		// Just collect the value and get the docoutput from that
		returnDocs = append(returnDocs, pair.Value.Output)
	}

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
func (c *Collection) DocumentPut(w http.ResponseWriter, r *http.Request, path string, schema *jsonschema.Schema) {
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

	// Marshal
	jsonResponse, err := json.Marshal(putoutput{path})
	if err != nil {
		// This should never happen
		slog.Error("Put: error marshaling", "error", err)
		http.Error(w, `"internal server error"`, http.StatusInternalServerError)
		return
	}

	// Conditional Put on timestamp
	timeStampStr := r.URL.Query().Get("timestamp")
	var timeStamp int64 = -1
	if timeStampStr != "" {
		val, err := strconv.Atoi(timeStampStr)
		if err != nil {
			slog.Error("Put: Bad timestamp", "error", err)
			http.Error(w, "Bad timestamp", http.StatusBadRequest)
			return
		}
		timeStamp = int64(val)
	}

	// Upsert for document; update if found, otherwise create new
	docUpsert := func(key string, currValue *Document, exists bool) (*Document, error) {
		if exists {
			// Conditional put
			matchOld := timeStamp == currValue.Output.Meta.LastModifiedAt || timeStamp == currValue.Output.Meta.CreatedAt
			if timeStamp != -1 && !matchOld {
				return nil, errors.New("Bad timestamp match")
			}

			// Modify data
			currValue.Overwrite(docBody)
			return currValue, nil
		} else {
			// Create new document
			newDoc := New(key, "DUMMY USER", docBody)
			return &newDoc, nil
		}
	}

	updated, err := c.Documents.Upsert(path, docUpsert)
	if err != nil {
		switch err.Error() {
		case "Bad timestamp":
			// TODO: error code for timestamp
			slog.Error(err.Error())
			http.Error(w, "PUT: bad timestamp", http.StatusNotFound)
		default:
			slog.Error(err.Error())
			http.Error(w, "PUT() error "+err.Error(), http.StatusInternalServerError)
		}
	}

	// Success: Construct response
	if updated {
		slog.Info("Overwrote an old document", "path", path)
		w.WriteHeader(http.StatusOK)
	} else {
		slog.Info("Created new document", "path", path)
		w.WriteHeader(http.StatusCreated)
	}
	w.Header().Set("Location", r.URL.Path)
	w.Write(jsonResponse)
}

// Deletes a document from this collection
func (c *Collection) DocumentDelete(w http.ResponseWriter, r *http.Request, docpath string) {
	// Just request a delete on the specified element
	_, deleted := c.Documents.Remove(docpath)

	// Handle response
	if !deleted {
		slog.Info("Document does not exist", "path", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	slog.Info("Deleted Document", "path", r.URL.Path)
	w.Header().Set("Location", r.URL.Path)
	w.WriteHeader(http.StatusNoContent)
}

func (c *Collection) DocumentPatch(w http.ResponseWriter, r *http.Request, docpath string) {
	// Patch document case
	// Retrieve document
	// TODO: maybe update this to use upsert?
	doc, ok := c.Documents.Find(docpath)

	// If document does not exist return error
	if !ok {
		slog.Info("User attempted to patch non-extant document", "doc", docpath)
		msg := fmt.Sprintf("Document, %s, does not exist", docpath)
		http.Error(w, msg, http.StatusNotFound)
		return
	}

	var patches []patcher.Patch

	// Read body of requests
	body, err := io.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		slog.Error("Patch document: error reading the patch request body", "error", err)
		http.Error(w, `"invalid request body"`, http.StatusBadRequest)
		return
	}

	// Unmarshal the body into an array of patches.
	json.Unmarshal(body, &patches)
	if err != nil {
		slog.Error("Patch document: error unmarshaling patch document request", "error", err)
		http.Error(w, `"invalid patch document format"`, http.StatusBadRequest)
		return
	}

	// Apply the patches to the document
	patchreply, newdoc := doc.ApplyPatches(patches)
	patchreply.Uri = r.URL.Path

	if !patchreply.PatchFailed {
		// Need to modify metadata
		doc.Overwrite(newdoc)

		// Upsert to reinsert
		patchUpsert := func(key string, currValue *Document, exists bool) (*Document, error) {
			if exists {
				return doc, nil
			} else {
				// We expect the document to already exist
				return nil, errors.New("Not found")
			}
		}

		updated, err := c.Documents.Upsert(docpath, patchUpsert)
		if !updated {
			// This shouldn't happen
			slog.Error("Patch: ", "error", err.Error())
			http.Error(w, `"internal server error"`, http.StatusInternalServerError)
			return
		}
	}

	// Marshal it into a json reply
	jsonResponse, err := json.Marshal(patchreply)
	if err != nil {
		// This should never happen
		slog.Error("Patch: error marshaling", "error", err)
		http.Error(w, `"internal server error"`, http.StatusInternalServerError)
		return
	}

	slog.Info("Patched a document", "path", r.URL.Path)
	w.Header().Set("Location", r.URL.Path)
	w.WriteHeader(http.StatusOK)
	w.Write(jsonResponse)
}

// note: a lot of shared logic with documentput, could refactor for shared method
func (c *Collection) DocumentPost(w http.ResponseWriter, r *http.Request, schema *jsonschema.Schema) {
	// Read body of requests
	desc, err := io.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		slog.Error("Post document: error reading the document request body", "error", err)
		http.Error(w, `"invalid document format"`, http.StatusBadRequest)
		return
	}

	// Read Body data
	var docBody map[string]interface{}
	err = json.Unmarshal(desc, &docBody)
	if err != nil {
		slog.Error("createReplaceDocument: error unmarshaling Post document request", "error", err)
		http.Error(w, `"invalid Post document format"`, http.StatusBadRequest)
		return
	}

	// Validate against schema
	err = schema.Validate(docBody)
	if err != nil {
		slog.Error("Post document: document did not conform to schema", "error", err)
		http.Error(w, `"document did not conform to schema"`, http.StatusBadRequest)
		return
	}

	// Upsert for post
	docUpsert := func(key string, currValue *Document, exists bool) (*Document, error) {
		if exists {
			// Return error
			return nil, errors.New("exists")
		} else {
			// Create new document
			newDoc := New(key, "DUMMY USER", docBody)
			return &newDoc, nil
		}
	}

	var path string
	for {
		// Same code as authenication.generateToken
		// Generate a 16-byte or 128-bit token
		token := make([]byte, 16)
		// Fill the slide with cryptographically secure random bytes
		_, err := rand.Read(token)
		if err != nil {
			slog.Error("Post document: could not generate random name", "error", err)
			http.Error(w, `"Could not generate random name`, http.StatusInternalServerError)
			return
		}

		// Convert the random bytes to a hexadecimal string
		randomName := hex.EncodeToString(token)
		_, upErr := c.Documents.Upsert(randomName, docUpsert)
		if upErr != nil {
			switch upErr.Error() {
			case "exists": // do nothing
			default:
				slog.Error(upErr.Error())
				http.Error(w, "POST() error "+upErr.Error(), http.StatusInternalServerError)
				return
			}

			// If "exists", then reloop
			continue
		}

		// No error: then stop
		path = randomName
		break
	}

	// Marshal
	jsonResponse, err := json.Marshal(putoutput{path})
	if err != nil {
		// This should never happen
		slog.Error("Post: error marshaling", "error", err)
		http.Error(w, `"internal server error"`, http.StatusInternalServerError)
		return
	}

	// Success: Construct response
	slog.Info("Created new document", "path", path)
	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Location", r.URL.Path)
	w.Write(jsonResponse)
}
