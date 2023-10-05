// Package dbhandler provides structs for handling
// HTTP requests according to the owldb specifications.
// Supports GET, PUT, POST, PATCH, DELETE, and OPTIONS
// requests.
package dbhandler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/RICE-COMP318-FALL23/owldb-p1group20/authentication"
	"github.com/RICE-COMP318-FALL23/owldb-p1group20/collection"
	"github.com/RICE-COMP318-FALL23/owldb-p1group20/document"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

// A putoutput stores the response to a put request.
type putoutput struct {
	Uri string `json:"uri"`
}

// A dbhandler is the highest level struct, holds all the
// base level databases as well as the schema and map of
// usernames to authentication tokens.
type Dbhandler struct {
	databases *sync.Map
	schema    *jsonschema.Schema
	sessions  *sync.Map
}

// Creates a new DBHandler
func New(testmode bool, schema *jsonschema.Schema) Dbhandler {
	retval := Dbhandler{&sync.Map{}, schema, &sync.Map{}}

	if testmode {
		slog.Info("Test mode enabled")

		// The current test cases will have
		retval.databases.Store("db1", collection.New())
		retval.databases.Store("db2", collection.New())
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
	case http.MethodPost:
		d.Post(w, r)
	case http.MethodPatch:
		d.Patch(w, r)
	case http.MethodDelete:
		d.Delete(w, r)
	case http.MethodOptions:
		d.Options(w, r)
	default:
		// If user used method we do not support.
		slog.Info("User used unsupported method", "method", r.Method)
		msg := fmt.Sprintf("unsupported method: %s", r.Method)
		http.Error(w, msg, http.StatusBadRequest)
	}
}

// Handles GET request by either returning a
// document body or set of all documents in a collection.
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

		// Access the database
		database, ok := d.databases.Load(dbpath)

		// Check to see if database exists
		if !ok {
			slog.Info("User attempted to access non-extant database", "db", dbpath)
			msg := fmt.Sprintf("Database does not exist")
			http.Error(w, msg, http.StatusNotFound)
			return
		}

		database.(collection.Collection).CollectionGet(w, r)

	} else {
		// GET Document
		dbpath, _ := strings.CutSuffix(splitpath[0], "/")
		path = splitpath[1]

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
		doc, ok := database.(collection.Collection).Documents.Load(path)
		if !ok {
			slog.Info("User attempted to access non-extant document", "doc", path)
			msg := fmt.Sprintf("Document does not exist")
			http.Error(w, msg, http.StatusNotFound)
			return
		}

		doc.(document.Document).DocumentGet(w, r)
	}
}

// Handles case where we have PUT request by either
// putting a new document or database at the desired
// location.
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
		dbpath := splitpath[0]

		d.putDB(w, r, dbpath)
	} else {
		// PUT document or collection
		dbpath, _ := strings.CutSuffix(splitpath[0], "/")

		// Access the database
		database, ok := d.databases.Load(dbpath)

		// Check to see if database exists
		if !ok {
			slog.Info("User attempted to access non-extant database", "db", dbpath)
			msg := fmt.Sprintf("Database does not exist")
			http.Error(w, msg, http.StatusNotFound)
			return
		}
		var collection = database.(collection.Collection)
		collection.DocumentPut(w, r, splitpath[1], d.schema)
	}
}

// Puts a new top level database into our handler.
func (d *Dbhandler) putDB(w http.ResponseWriter, r *http.Request, dbpath string) {
	// Add a new database to dbhandler if it is not already there; otherwise, return error. (I assumed database and collection use the same struct).
	_, loaded := d.databases.LoadOrStore(dbpath, collection.New())
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
}

// Handles a DELETE request either by logging out the user
// if they use the /auth path, and otherwise by deleting
// the desired database or document.
func (d *Dbhandler) Delete(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/auth" {
		// logout
		authentication.Logout(d.sessions, w, r)
	} else {
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

		dbpath := splitpath[0]

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

			var collection = database.(collection.Collection)
			collection.DocumentDelete(w, r, docpath)
		}
	}
}

// Handles a POST request either by logging in the user or
// by adding a document to the desired top level db with a
// random name.
func (d *Dbhandler) Post(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/auth" {
		// login
		authentication.Login(d.sessions, w, r)
	} else {
		// Handle other cases
	}
}

// Handles a PATCH request by finding the proper document
// and applying the desired patches.
func (d *Dbhandler) Patch(w http.ResponseWriter, r *http.Request) {
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

	dbpath := splitpath[0]
	dbpath, _ = strings.CutSuffix(splitpath[0], "/")

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
		// Case where we do not point to a document
		slog.Info("User attempted to patch database", "db", dbpath)
		msg := fmt.Sprintf("Invalid patch target, %s", r.URL.Path)
		http.Error(w, msg, http.StatusNotFound)
		return
	} else {
		// Decode the document name
		docpath, _ := strings.CutSuffix(splitpath[1], "/")

		var collection = database.(collection.Collection)
		collection.DocumentPatch(w, r, docpath)
	}
}

// Handles OPTIONS request by sending the list of acceptable
// methods and headers to the client.
func (d *Dbhandler) Options(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Allow", "GET,PUT,POST,PATCH,DELETE,OPTIONS")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET,PUT,POST,PATCH,DELETE,OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "accept,Content-Type,Authorization")
	w.WriteHeader(http.StatusOK)
}
