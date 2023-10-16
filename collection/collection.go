package collection

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
	"strings"

	"github.com/RICE-COMP318-FALL23/owldb-p1group20/interfaces"
	"github.com/RICE-COMP318-FALL23/owldb-p1group20/patcher"
	"github.com/RICE-COMP318-FALL23/owldb-p1group20/skiplist"
	"github.com/RICE-COMP318-FALL23/owldb-p1group20/structs"
	"github.com/RICE-COMP318-FALL23/owldb-p1group20/subscribe"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

/*
A collection is a concurrent skip list of documents,
which is sorted by document name.
*/
type Collection struct {
	documents   *skiplist.SkipList[string, interfaces.IDocument]
	subscribers []subscribe.Subscriber
}

// Creates a new collection.
func New() Collection {
	newSL := skiplist.New[string, interfaces.IDocument](skiplist.STRING_MIN, skiplist.STRING_MAX, skiplist.DEFAULT_LEVEL)
	return Collection{&newSL, make([]subscribe.Subscriber, 0)}
}

// Gets a collection of documents
func (c *Collection) CollectionGet(w http.ResponseWriter, r *http.Request) {
	// Get queries
	queries := r.URL.Query()
	interval := getInterval(queries.Get("interval"))

	// Build a list of document outputs
	returnDocs := make([]interface{}, 0)

	// Make query on collection
	pairs, err := c.documents.Query(r.Context(), interval[0], interval[1])

	if err != nil {
		// TODO: type of error?
		slog.Info("Collection could not retrieve query in time")
		http.Error(w, `"Timeout while querying collection"`, http.StatusRequestTimeout)
		return
	}

	for _, pair := range pairs {
		// Just collect the value and get the docoutput from that
		returnDocs = append(returnDocs, pair.Value.GetRawBody())
	}

	// Subscribe mode
	mode := r.URL.Query().Get("mode")
	if mode == "subscribe" {
		subscriber := subscribe.New()
		c.subscribers = append(c.subscribers, subscriber)
		go subscriber.ServeSubscriber(w, r)

		for _, output := range returnDocs {
			jsonBody, err := json.Marshal(output)
			if err != nil {
				// This should never happen
				slog.Error("Get: error marshaling", "error", err)
				http.Error(w, `"internal server error"`, http.StatusInternalServerError)
				return
			}
			subscriber.UpdateCh <- jsonBody
		}
		return
	}

	// Convert to JSON and send
	jsonDocs, err := json.Marshal(returnDocs)
	if err != nil {
		// This should never happen
		slog.Error("Get: error marshaling", "error", err)
		http.Error(w, `"internal server error"`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonDocs)
	slog.Info("Col/DB GET: success")
}

// Puts a document into a collection
func (c *Collection) DocumentPut(w http.ResponseWriter, r *http.Request, path string, newDoc interfaces.IDocument) {

	// Marshal
	jsonResponse, err := json.Marshal(structs.PutOutput{Uri: r.URL.Path})
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
	docUpsert := func(key string, currValue interfaces.IDocument, exists bool) (interfaces.IDocument, error) {
		if exists {
			// Conditional put
			matchOld := timeStamp == currValue.GetLastModified()
			if timeStamp != -1 && !matchOld {
				return nil, errors.New("bad timestamp match")
			}

			// Modify metadata
			currValue.Overwrite(newDoc.GetDoc(), newDoc.GetOriginalAuthor())

			updateMSG, err := currValue.GetJSONBody()
			if err != nil {
				http.Error(w, `"internal server error"`, http.StatusInternalServerError)
				return nil, err
			}

			// Notify doc subscribers
			for _, sub := range currValue.GetSubscribers() {
				sub.UpdateCh <- updateMSG
			}

			// Notify collection subscribers
			for _, sub := range c.subscribers {
				sub.UpdateCh <- updateMSG
			}

			return currValue, nil
		} else {
			// Create new document
			updateMSG, err := newDoc.GetJSONBody()
			if err != nil {
				http.Error(w, `"internal server error"`, http.StatusInternalServerError)
				return nil, errors.New("marshalling error")
			}

			// Notify collection subscribers
			for _, sub := range c.subscribers {
				sub.UpdateCh <- updateMSG
			}

			return newDoc, nil
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
	for _, sub := range doc.GetSubscribers() {
		sub.DeleteCh <- r.URL.Path
	}

	// Notify collection subscribers
	for _, sub := range c.subscribers {
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

		updateMSG, err := doc.GetJSONBody()
		if err != nil {
			http.Error(w, `"internal server error"`, http.StatusInternalServerError)
			return
		}

		// Notify doc subscribers
		for _, sub := range doc.GetSubscribers() {
			sub.UpdateCh <- updateMSG
		}

		// Notify collection subscribers
		for _, sub := range c.subscribers {
			sub.UpdateCh <- updateMSG
		}

		// Upsert to reinsert
		patchUpsert := func(key string, currValue interfaces.IDocument, exists bool) (interfaces.IDocument, error) {
			if exists {
				// Delete Children of this document
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
func (c *Collection) DocumentPost(w http.ResponseWriter, r *http.Request, newDoc interfaces.IDocument) {

	// Upsert for post
	docUpsert := func(key string, currValue interfaces.IDocument, exists bool) (interfaces.IDocument, error) {
		if exists {
			// Return error
			return nil, errors.New("exists")
		} else {
			updateMSG, err := newDoc.GetJSONBody()
			if err != nil {
				http.Error(w, `"internal server error"`, http.StatusInternalServerError)
				return nil, errors.New("marshalling error")
			}

			// Notify collection subscribers
			for _, sub := range c.subscribers {
				sub.UpdateCh <- updateMSG
			}

			return newDoc, nil
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
	jsonResponse, err := json.Marshal(structs.PutOutput{Uri: r.URL.Path + path})
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
func (c *Collection) DocumentFind(resource string) (interfaces.IDocument, bool) {
	return c.documents.Find(resource)
}

// Get the subscribers to this collection.
func (c *Collection) GetSubscribers() []subscribe.Subscriber {
	return c.subscribers
}

/*
Convert a string representing string intervals into the elements inside the interval.

The interval must be of the format [x,y] where x and y are the min and max of
the interval. x and y may be optional where they are substituted for minima and maxima.
*/
func getInterval(intervalStr string) [2]string {
	interval := [2]string{skiplist.STRING_MIN, skiplist.STRING_MAX}
	// Must be in array form
	if !(len(intervalStr) > 2 && intervalStr[0] == '[' && intervalStr[len(intervalStr)-1] == ']') {
		slog.Info("GetInterval: Bad interval, non-array", "interval", intervalStr)
		return interval
	}

	// Get rid of array surrounders and split
	intervalStr = intervalStr[1 : len(intervalStr)-1]
	procArr := strings.Split(intervalStr, ",")

	if len(procArr) != 2 {
		// Too many args
		slog.Info("GetInterval: Bad interval, incorrect args", "interval", intervalStr)
		return interval
	}

	// Success
	interval[0] = procArr[0]
	interval[1] = procArr[1]

	if interval[1] == "" {
		interval[1] = skiplist.STRING_MAX
	}

	slog.Info("GetInterval: Good interval", "arg[0]", interval[0], "arg[1]", interval[1])
	return interval
}
