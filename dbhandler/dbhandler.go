// A package implementing the highest level handling for
// the OwlDB project.
package dbhandler

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/RICE-COMP318-FALL23/owldb-p1group20/decoder"
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
type collection struct {
	documents *sync.Map
}

// Gets a collection
func (c collection) collectionGet(w http.ResponseWriter, r *http.Request) {
	returnDocs := make([]docoutput, 0)

	// Add each docoutput to the docoutputs list
	c.documents.Range(func(key, value interface{}) bool {
		returnDocs = append(returnDocs, value.(document).output)
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
func (c collection) documentPut(w http.ResponseWriter, r *http.Request, path string, schema *jsonschema.Schema) {

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
	existingDoc, exists := c.documents.Load(path)
	if exists {
		jsonResponse, err := json.Marshal(putoutput{r.URL.Path})
		if err != nil {
			// This should never happen
			slog.Error("Get: error marshaling", "error", err)
			http.Error(w, `"internal server error"`, http.StatusInternalServerError)
			return
		}
		// Need to modify metadata
		var modifiedDoc = existingDoc.(document)
		existingDocOutput := modifiedDoc.output
		existingDocOutput.Meta.LastModifiedAt = time.Now().UnixMilli()
		existingDocOutput.Meta.LastModifiedBy = "DUMMY USER"

		// Modify document contents
		existingDocOutput.Doc = docBody

		// Modify it again in the doc
		modifiedDoc.output = existingDocOutput
		c.documents.Store(path, modifiedDoc)

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
		var docOutput docoutput
		docOutput.Path = "/" + path
		docOutput.Doc = docBody
		docOutput.Meta = meta{"DUMMY USER", time.Now().UnixMilli(), "DUMMY USER", time.Now().UnixMilli()}

		c.documents.Store(path, document{docOutput, nil})
		slog.Info("Created new document", "path", path)
		w.Header().Set("Location", r.URL.Path)
		w.WriteHeader(http.StatusCreated)
		w.Write(jsonResponse)
	}

}

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
	Path string                 `json:"path"`
	Doc  map[string]interface{} `json:"doc"`
	Meta meta                   `json:"meta"`
}

// A document is a document plus a concurrent skip list of collections
type document struct {
	output   docoutput
	children *sync.Map
}

// Gets a document
func (d document) documentGet(w http.ResponseWriter, r *http.Request) {
	// Convert to JSON and send
	jsonDoc, err := json.Marshal(d.output)

	if err != nil {
		// This should never happen
		slog.Error("Get: error marshaling", "error", err)
		http.Error(w, `"internal server error"`, http.StatusInternalServerError)
		return
	}

	w.Write(jsonDoc)
	slog.Info("GET: success")
}

// A dbhandler is the highest level struct, holds all the collections and
// handles all the http requests.
type Dbhandler struct {
	databases *sync.Map
	schema    *jsonschema.Schema
}

// Creates a new DBHandler
func New(testmode bool, schema *jsonschema.Schema) Dbhandler {
	retval := Dbhandler{&sync.Map{}, schema}
	if testmode {
		slog.Info("Test mode enabled", "INFO", 0)

		// The current test cases will have
		retval.databases.Store("db1", collection{&sync.Map{}})
		retval.databases.Store("db2", collection{&sync.Map{}})
	}
	return retval
}

// The server implements the "handler" interface, it will recieve
// requests from the user and delegate them to the proper methods.
func (d *Dbhandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		d.Get(w, r)
	case http.MethodPut:
		d.Put(w, r)
	//case http.MethodPost:
	// Post handling
	//case http.MethodPatch:
	// Patch handling
	case http.MethodDelete:
		d.Delete(w, r)
	// Delete handling
	case http.MethodOptions:
		d.Options(w, r)
	default:
		// If user used method we do not support.
		slog.Info("User used unsupported method", "method", r.Method)
		msg := fmt.Sprintf("unsupported method: %s", r.Method)
		http.Error(w, msg, http.StatusBadRequest)
	}
}

// Handles case where we recieve a GET request.
func (d *Dbhandler) Get(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Check for version
	path, found := strings.CutPrefix(r.URL.Path, "/v1/")
	if !found {
		slog.Info("User path did not include version", "path", r.URL.Path)
		msg := fmt.Sprintf("path missing version: %s", r.URL.Path)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	// Action fork for GET Database and GET Document
	// Only compliant for check-in.
	// Checkpoint 1 heirarchy
	// dbhandler -> databases (map) -> collection -> documents (map) -> document -> docoutput (metadata, path, contents)
	splitpath := strings.SplitAfterN(path, "/", 2)
	if len(splitpath) == 1 {
		// Error, DB path does not end with "/"
		slog.Info("DB path did not end with '/'", "path", r.URL.Path)
		msg := fmt.Sprintf("path missing trailing '/': %s", r.URL.Path)
		http.Error(w, msg, http.StatusBadRequest)
		return
	} else if splitpath[1] == "" {
		// GET Database
		dbpath, _ := strings.CutSuffix(splitpath[0], "/")
		dbpath, err := decoder.PercentDecoding(dbpath)

		// Error messages printed in percentDecoding function
		if err != nil {
			msg := fmt.Sprintf("Error translating hex encoding: %s", err.Error())
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}

		// Access the database
		database, ok := d.databases.Load(dbpath)

		// Check to see if database exists
		if !ok {
			slog.Info("User attempted to access non-extant database", "db", dbpath)
			msg := fmt.Sprintf("Database does not exist")
			http.Error(w, msg, http.StatusNotFound)
			return
		}

		database.(collection).collectionGet(w, r)

	} else {
		// GET Document
		dbpath, _ := strings.CutSuffix(splitpath[0], "/")
		dbpath, err := decoder.PercentDecoding(dbpath)
		path = splitpath[1]

		// Error messages printed in percentDecoding function
		if err != nil {
			msg := fmt.Sprintf("Error translating hex encoding: %s", err.Error())
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}

		// Access the database
		database, ok := d.databases.Load(dbpath)

		// Check to see if database exists
		if !ok {
			slog.Info("User attempted to access non-extant database", "db", dbpath)
			msg := fmt.Sprintf("Document does not exist")
			http.Error(w, msg, http.StatusNotFound)
			return
		}

		// Get document
		// for checkpoint 1, we assume that path will always be a document name
		doc, ok := database.(collection).documents.Load(path)
		if !ok {
			slog.Info("User attempted to access non-extant document", "doc", path)
			msg := fmt.Sprintf("Document does not exist")
			http.Error(w, msg, http.StatusNotFound)
			return
		}

		doc.(document).documentGet(w, r)
	}
}

// Handles case where we have PUT request.
func (d *Dbhandler) Put(w http.ResponseWriter, r *http.Request) {
	// Set headers of response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	path, found := strings.CutPrefix(r.URL.Path, "/v1/")

	// Check for version
	if !found {
		slog.Info("User path did not include version", "path", path)
		msg := fmt.Sprintf("path missing version: %s", path)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	splitpath := strings.SplitAfterN(path, "/", 2)

	if len(splitpath) == 1 {
		// PUT database case
		dbpath, err := decoder.PercentDecoding(splitpath[0])

		// Error messages printed in percentDecoding function
		if err != nil {
			msg := fmt.Sprintf("Error translating hex encoding: %s", err.Error())
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}

		// Add a new database to dbhandler if it is not already there; otherwise, return error. (I assumed database and collection use the same struct).
		_, loaded := d.databases.LoadOrStore(dbpath, collection{&sync.Map{}})
		if loaded {
			slog.Error("Database already exists")
			http.Error(w, "Database already exists", http.StatusBadRequest)
			return
		} else {
			jsonResponse, err := json.Marshal(putoutput{r.URL.Path})
			if err != nil {
				// This should never happen
				slog.Error("Get: error marshaling", "error", err)
				http.Error(w, `"internal server error"`, http.StatusInternalServerError)
				return
			}
			slog.Info("Created Database", "path", dbpath)
			w.Header().Set("Location", r.URL.Path)
			w.WriteHeader(http.StatusCreated)
			w.Write(jsonResponse)
			return
		}

	} else {
		// PUT document or collection
		dbpath, _ := strings.CutSuffix(splitpath[0], "/")
		dbpath, err := decoder.PercentDecoding(dbpath)

		// Error messages printed in percentDecoding function
		if err != nil {
			msg := fmt.Sprintf("Error translating hex encoding: %s", err.Error())
			http.Error(w, msg, http.StatusBadRequest)
			return
		}

		// Access the database
		database, ok := d.databases.Load(dbpath)

		// Check to see if database exists
		if !ok {
			slog.Info("User attempted to access non-extant database", "db", dbpath)
			msg := fmt.Sprintf("Database does not exist")
			http.Error(w, msg, http.StatusNotFound)
			return
		}

		database.(collection).documentPut(w, r, splitpath[1], d.schema)
	}
}

// Handles case where we have OPTIONS request.
func (d *Dbhandler) Options(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Allow", "GET,PUT,POST,PATCH,DELETE,OPTIONS")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET,PUT,POST,PATCH,DELETE,OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "accept,Content-Type,Authorization")
	w.WriteHeader(http.StatusOK)
}

// Handles case where we have DELETE request.
func (d *Dbhandler) Delete(w http.ResponseWriter, r *http.Request) {
	// Set headers of response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	path, found := strings.CutPrefix(r.URL.Path, "/v1/")

	// Check for version
	if !found {
		slog.Info("User path did not include version", "path", path)
		msg := fmt.Sprintf("path missing version: %s", path)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	splitpath := strings.SplitAfterN(path, "/", 2)

	dbpath, err := decoder.PercentDecoding(splitpath[0])

	// Error messages printed in percentDecoding function
	if err != nil {
		msg := fmt.Sprintf("Error translating hex encoding: %s", err.Error())
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	// Check to see if database exists
	database, ok := d.databases.Load(dbpath)
	// If the database does not exist, return StatusNotFound error
	if !ok {
		slog.Info("User attempted to access non-extant database", "db", dbpath)
		msg := fmt.Sprintf("Database does not exist")
		http.Error(w, msg, http.StatusNotFound)
		return
	}

	if len(splitpath) == 1 {
		// DELETE database case
		d.databases.Delete(dbpath)
		slog.Info("Deleted Database", "path", dbpath)
		w.Header().Set("Location", r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
		return

	} else {
		// DELETE document case

		// Decode the document name
		docpath, _ := strings.CutSuffix(splitpath[1], "/")
		docpath, err1 := decoder.PercentDecoding(docpath)
		if err1 != nil {
			msg := fmt.Sprintf("Error translating hex encoding: %s", err.Error())
			http.Error(w, msg, http.StatusBadRequest)
			return
		}

		// Acess the document
		_, exist := database.(collection).documents.Load(docpath)
		if exist {
			// Document found
			database.(collection).documents.Delete(docpath)
			slog.Info("Deleted Document", "path", path)
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
}
