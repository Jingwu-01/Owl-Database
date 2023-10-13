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
	"github.com/RICE-COMP318-FALL23/owldb-p1group20/relative"
	"github.com/RICE-COMP318-FALL23/owldb-p1group20/skiplist"
	"github.com/RICE-COMP318-FALL23/owldb-p1group20/subscribe"
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
	documents   *skiplist.SkipList[string, *Document]
	Subscribers []subscribe.Subscriber
}

// Creates a new collection.
func NewCollection() Collection {
	newSL := skiplist.New[string, *Document](skiplist.STRING_MIN, skiplist.STRING_MAX, skiplist.DEFAULT_LEVEL)
	return Collection{&newSL, make([]subscribe.Subscriber, 0)}
}

// Gets a collection of documents
func (c *Collection) CollectionGet(w http.ResponseWriter, r *http.Request) {
	// Get queries
	queries := r.URL.Query()
	interval := pathprocessor.GetInterval(queries.Get("interval"))

	// Build a list of document outputs
	returnDocs := make([]Docoutput, 0)

	// Make query on collection
	pairs, err := c.documents.Query(r.Context(), interval[0], interval[1])

	if err != nil {
		// TODO: type of error?
		slog.Info("Collection could not retrieve query in time")
		return
	}

	for _, pair := range pairs {
		// Just collect the value and get the docoutput from that
		returnDocs = append(returnDocs, pair.Value.Output)
	}

	// Subscribe mode
	mode := r.URL.Query().Get("mode")
	if mode == "subscribe" {
		subscriber := subscribe.New()
		c.Subscribers = append(c.Subscribers, subscriber)
		w.Header().Set("Content-Type", "text/event-stream")
		go subscriber.ServeHTTP(w, r)

		for _, output := range returnDocs {
			updateMSG, err := json.Marshal(output)
			if err != nil {
				// This should never happen
				slog.Error("Put: error marshaling", "error", err)
				http.Error(w, `"internal server error"`, http.StatusInternalServerError)
				return
			}
			subscriber.UpdateCh <- updateMSG
		}

		return
	}

	// Convert to JSON and send
	jsonToDo, err := json.Marshal(returnDocs)
	if err != nil {
		// This should never happen
		slog.Error("Get: error marshaling", "error", err)
		http.Error(w, `"internal server error"`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonToDo)
	slog.Info("Col/DB GET: success")
}

// Puts a document into a collection
func (c *Collection) DocumentPut(w http.ResponseWriter, r *http.Request, path string, schema *jsonschema.Schema, name string) {
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
	jsonResponse, err := json.Marshal(putoutput{r.URL.Path})
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
				return nil, errors.New("bad timestamp match")
			}

			// Modify metadata
			currValue.Overwrite(docBody, name)

			updateMSG, err := json.Marshal(currValue.Output)
			if err != nil {
				// This should never happen
				slog.Error("Put: error marshaling", "error", err)
				http.Error(w, `"internal server error"`, http.StatusInternalServerError)
				return nil, errors.New("marshalling error")
			}

			// Notify doc subscribers
			for _, sub := range currValue.Subscribers {
				sub.UpdateCh <- updateMSG
			}

			// Notify collection subscribers
			for _, sub := range c.Subscribers {
				sub.UpdateCh <- updateMSG
			}

			// Delete Children of this document
			newHolder := NewHolder()
			currValue.Children = &newHolder

			return currValue, nil
		} else {
			// Create new document
			newDoc := New(relative.GetRelativePathNonDB(r.URL.Path), name, docBody)

			updateMSG, err := json.Marshal(newDoc.Output)
			if err != nil {
				// This should never happen
				slog.Error("Put: error marshaling", "error", err)
				http.Error(w, `"internal server error"`, http.StatusInternalServerError)
				return nil, errors.New("marshalling error")
			}

			// Notify collection subscribers
			for _, sub := range c.Subscribers {
				sub.UpdateCh <- updateMSG
			}

			return &newDoc, nil
		}
	}

	updated, err := c.documents.Upsert(path, docUpsert)
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
	w.Header().Set("Location", r.URL.Path)
	slog.Info("", "DocumentCreate Location", r.URL.Path)
	if updated {
		slog.Info("Overwrote an old document", "path", path)
		w.WriteHeader(http.StatusOK)
	} else {
		slog.Info("Created new document", "path", path)
		w.WriteHeader(http.StatusCreated)
	}
	w.Write(jsonResponse)
}

// Deletes a document from this collection
func (c *Collection) DocumentDelete(w http.ResponseWriter, r *http.Request, docpath string) {
	// Just request a delete on the specified element
	doc, deleted := c.documents.Remove(docpath)

	// Handle response
	if !deleted {
		slog.Info("Document does not exist", "path", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Notify doc subscribers
	for _, sub := range doc.Subscribers {
		sub.DeleteCh <- r.URL.Path
	}

	// Notify collection subscribers
	for _, sub := range c.Subscribers {
		sub.DeleteCh <- r.URL.Path
	}

	slog.Info("Deleted Document", "path", r.URL.Path)
	w.Header().Set("Location", r.URL.Path)
	w.WriteHeader(http.StatusNoContent)
}

// Patches a document in this collection
func (c *Collection) DocumentPatch(w http.ResponseWriter, r *http.Request, docpath string, schema *jsonschema.Schema, name string) {
	// Patch document case
	// Retrieve document
	doc, ok := c.documents.Find(docpath)

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
	err = json.Unmarshal(body, &patches)
	if err != nil {
		slog.Error("Patch document: error unmarshaling patch document request", "error", err)
		http.Error(w, `"invalid patch document format"`, http.StatusBadRequest)
		return
	}

	// Apply the patches to the document
	patchreply, newdoc := doc.ApplyPatches(patches, schema)
	patchreply.Uri = r.URL.Path

	// Marshal it into a json reply
	jsonResponse, err := json.Marshal(patchreply)
	if err != nil {
		// This should never happen
		slog.Error("Patch: error marshaling", "error", err)
		http.Error(w, `"internal server error"`, http.StatusInternalServerError)
		return
	}

	if !patchreply.PatchFailed {
		// Need to modify metadata
		doc.Overwrite(newdoc, name)

		updateMSG, err := json.Marshal(doc.Output)
		if err != nil {
			// This should never happen
			slog.Error("Put: error marshaling", "error", err)
			http.Error(w, `"internal server error"`, http.StatusInternalServerError)
			return
		}

		// Notify doc subscribers
		for _, sub := range doc.Subscribers {
			sub.UpdateCh <- updateMSG
		}

		// Notify collection subscribers
		for _, sub := range c.Subscribers {
			sub.UpdateCh <- updateMSG
		}

		// Upsert to reinsert
		patchUpsert := func(key string, currValue *Document, exists bool) (*Document, error) {
			if exists {
				return doc, nil
			} else {
				// We expect the document to already exist
				return nil, errors.New("not found")
			}
		}

		updated, err := c.documents.Upsert(docpath, patchUpsert)
		if !updated {
			// This shouldn't happen
			slog.Error("Patch: ", "error", err.Error())
			http.Error(w, `"internal server error"`, http.StatusInternalServerError)
			return
		}
		slog.Info("Patched a document", "path", r.URL.Path)
		w.Header().Set("Location", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	} else {
		slog.Info("Failed to patch a document", "path", r.URL.Path)
		w.WriteHeader(http.StatusBadRequest)
	}

	w.Write(jsonResponse)
}

// Posts a document in this collection
func (c *Collection) DocumentPost(w http.ResponseWriter, r *http.Request, schema *jsonschema.Schema, name string) {
	// note: a lot of shared logic with documentput, could refactor for shared method
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
			newDoc := New(key, name, docBody)

			updateMSG, err := json.Marshal(newDoc.Output)
			if err != nil {
				// This should never happen
				slog.Error("Post: error marshaling", "error", err)
				http.Error(w, `"internal server error"`, http.StatusInternalServerError)
				return nil, errors.New("marshalling error")
			}

			// Notify collection subscribers
			for _, sub := range c.Subscribers {
				sub.UpdateCh <- updateMSG
			}

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
		_, upErr := c.documents.Upsert(randomName, docUpsert)
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
	jsonResponse, err := json.Marshal(putoutput{r.URL.Path + path})
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

// Find a document in this collection
func (c *Collection) DocumentFind(resource string) (coll *Document, found bool) {
	return c.documents.Find(resource)
}
