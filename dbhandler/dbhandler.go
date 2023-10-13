// Package dbhandler provides structs for handling
// HTTP requests according to the owldb specifications.
// Supports GET, PUT, POST, PATCH, DELETE, and OPTIONS
// requests.
package dbhandler

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/RICE-COMP318-FALL23/owldb-p1group20/document"
	"github.com/RICE-COMP318-FALL23/owldb-p1group20/options"
	"github.com/RICE-COMP318-FALL23/owldb-p1group20/subscribe"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

// Result code from getResourceFromPath()
const (
	RESOURCE_BLANK_PATHNAME = -104
	RESOURCE_PUT_BAD_NAME   = -103
	RESOURCE_INTERNAL       = -102
	RESOURCE_BAD_SLASH      = -101
	RESOURCE_NO_VERSION     = -100
	RESOURCE_NO_DB          = -RESOURCE_DB
	RESOURCE_NO_COLL        = -RESOURCE_COLL
	RESOURCE_NO_DOC         = -RESOURCE_DOC

	RESOURCE_NULL  = 0
	RESOURCE_DB    = 1
	RESOURCE_COLL  = 2
	RESOURCE_DOC   = 3
	RESOURCE_DB_PD = 4 // specifically for put and delete db w/o slash
)

// An authenticator is something which can validate a login token as one supported
// by a dbhandler or not.
type Authenticator interface {
	ValidateToken(w http.ResponseWriter, r *http.Request) (bool, string)
}

// A dbhandler is the highest level struct, holds all the
// base level databases as well as the schema and map of
// usernames to authentication tokens.
type Dbhandler struct {
	databases     *document.CollectionHolder
	schema        *jsonschema.Schema
	authenticator Authenticator
}

// Creates a new DBHandler
func New(testmode bool, schema *jsonschema.Schema, authenticator Authenticator) Dbhandler {
	newHolder := document.NewHolder()
	retval := Dbhandler{&newHolder, schema, authenticator}

	if testmode {
		slog.Info("Test mode enabled")

		// The current test cases will have
		// Need to be updated
		/*
			retval.databases.Store("db1", collection.New())
			retval.databases.Store("db2", collection.New())
			db1, _ := retval.databases.Load("db1")
			var collection = db1.(collection.Collection)
			docbody := make(map[string]interface{})
			docbody["test"] = 1
			docbody["jerry"] = "jingwu"
			doc := document.New("/doc", "charlie", docbody)
			collection.Documents.Store("doc", doc)
		*/
	}

	return retval
}

// The server implements the "handler" interface, it will recieve
// requests from the user and delegate them to the proper methods.
func (d *Dbhandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Set headers of response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Check if user is valid.
	if r.Method == http.MethodOptions {
		options.Options(w, r)
	} else {
		valid, username := d.authenticator.ValidateToken(w, r)
		if valid {
			switch r.Method {
			case http.MethodGet:
				d.get(w, r)
			case http.MethodPut:
				d.put(w, r, username)
			case http.MethodPost:
				d.post(w, r, username)
			case http.MethodPatch:
				d.patch(w, r, username)
			case http.MethodDelete:
				d.delete(w, r)
			default:
				// If user used method we do not support.
				slog.Info("User used unsupported method", "method", r.Method)
				msg := fmt.Sprintf("unsupported method: %s", r.Method)
				http.Error(w, msg, http.StatusBadRequest)
			}
		}
	}
}

// Top-level GET handler
//
// Handles GET document, GET database, or GET collection.
// On success, sends a response body of all document or a set of all documents.
func (d *Dbhandler) get(w http.ResponseWriter, r *http.Request) {
	// Check if we are in the subscribe mode
	mode := r.URL.Query().Get("mode")
	if mode == "subscribe" {
		subscribe.New().ServeHTTP(w, r)
		return
	}

	// Action fork for GET Database and GET Document
	coll, doc, resc := d.getResourceFromPath(r.URL.Path)
	switch resc {
	case RESOURCE_DB:
		d.DatabaseGet(w, r, coll)
	case RESOURCE_COLL:
		coll.CollectionGet(w, r)
	case RESOURCE_DOC:
		doc.DocumentGet(w, r)
	default:
		d.handlePathError(w, r, resc)
	}
}

// Top-level PUT handler
//
// Handles PUT document, PUT database, or PUT collection.
// On success, puts the specified resource at the specified path.
func (d *Dbhandler) put(w http.ResponseWriter, r *http.Request, username string) {
	// Obtain parent resource to put to
	newRequest, newName, resc := cutRequest(r.URL.Path)

	// PUT database
	if resc == RESOURCE_DB_PD {
		d.DatabasePut(w, r, newName)
		return
	} else if resc < 0 || resc == RESOURCE_DB {
		d.handlePathError(w, r, resc)
		return
	}

	// Action fork for PUT Document and PUT Collection
	coll, doc, resc := d.getResourceFromPath(newRequest)
	switch resc {
	case RESOURCE_DB:
		// PUT document (in database)
		coll.DocumentPut(w, r, newName, d.schema, username)
	case RESOURCE_COLL:
		// PUT document (in collection)
		coll.DocumentPut(w, r, newName, d.schema, username)
	case RESOURCE_DOC:
		// PUT collection (in document)
		doc.Children.CollectionPut(w, r, newName)
	default:
		d.handlePathError(w, r, resc)
	}
}

// Top-level delete resource handler
//
// Handles DELETE database, DELETE document, DELETE collection.
// On success, deletes the desired resource based on the specified path.
func (d *Dbhandler) delete(w http.ResponseWriter, r *http.Request) {
	// Obtain parent resource to delete the element from
	newRequest, newName, resc := cutRequest(r.URL.Path)

	// DELETE database
	if resc == RESOURCE_DB_PD {
		d.DatabaseDelete(w, r, newName)
		return
	} else if resc < 0 || resc == RESOURCE_DB {
		d.handlePathError(w, r, resc)
		return
	}

	// Action fork for DELETE Document and DELETE Collection
	coll, doc, resc := d.getResourceFromPath(newRequest)
	switch resc {
	case RESOURCE_DB:
		// DELETE document (from database)
		coll.DocumentDelete(w, r, newName)
	case RESOURCE_COLL:
		// delete a document from a collection
		coll.DocumentDelete(w, r, newName)
	case RESOURCE_DOC:
		// delete a collection from a document
		doc.Children.CollectionDelete(w, r, newName)
	default:
		d.handlePathError(w, r, resc)
	}
}

// Top-level POST resource handler
//
// Handles POST Database, POST Collection.
// On success, adds the requested document with a randomly generated name
// to a database or collection.
func (d *Dbhandler) post(w http.ResponseWriter, r *http.Request, username string) {
	mode := r.URL.Query().Get("mode")
	if mode == "subscribe" {
		subscribe.New().ServeHTTP(w, r)
		return
	}
	// Action fork for POST Database and POST Collection
	coll, _, resc := d.getResourceFromPath(r.URL.Path)
	switch resc {
	case RESOURCE_DB:
		d.DatabasePost(w, r, coll, username)
	case RESOURCE_COLL:
		coll.DocumentPost(w, r, d.schema, username)
	default:
		d.handlePathError(w, r, resc)
	}
}

// Top-level PATCH handler
//
// Handles PATCH database, PATCH collection
// On success, applies the desired patches.
func (d *Dbhandler) patch(w http.ResponseWriter, r *http.Request, name string) {
	// Patch requires the parent resource
	newRequest, newName, resc := cutRequest(r.URL.Path)
	if resc < 0 {
		d.handlePathError(w, r, resc)
		return
	}

	// Action fork for PATCH document
	// Should go to parent first
	coll, _, resc := d.getResourceFromPath(newRequest)
	switch resc {
	case RESOURCE_DB:
		coll.DocumentPatch(w, r, newName, d.schema, name)
	case RESOURCE_COLL:
		coll.DocumentPatch(w, r, newName, d.schema, name)
	default:
		d.handlePathError(w, r, resc)
	}
}

// Specific handler for GET database
func (d *Dbhandler) DatabaseGet(w http.ResponseWriter, r *http.Request, coll *document.Collection) {
	// Same behavior as collection for now
	coll.CollectionGet(w, r)
}

// Specific handler for PUT database
func (d *Dbhandler) DatabasePut(w http.ResponseWriter, r *http.Request, dbpath string) {
	// Same behavior as collection for now
	d.databases.CollectionPut(w, r, dbpath)
}

// Specific handler for POST database
func (d *Dbhandler) DatabasePost(w http.ResponseWriter, r *http.Request, coll *document.Collection, name string) {
	// Same behavior as collection for now
	coll.DocumentPost(w, r, d.schema, name)
}

// Specific handler for DELETE database
func (d *Dbhandler) DatabaseDelete(w http.ResponseWriter, r *http.Request, name string) {
	// Same behavior as collection for now
	d.databases.CollectionDelete(w, r, name)
}

/*
Obtains resource from the specified path.

On success, returns a collection if the path leads to a collection or a database,
or a document if the path leads to a document. Returns a result code indicating
the type of resource returned.

On error, returns a resource error code indicating the type of error found.
*/
func (d *Dbhandler) getResourceFromPath(request string) (*document.Collection, *document.Document, int) {
	// Check version
	path, found := strings.CutPrefix(request, "/v1/")
	if !found {
		return nil, nil, RESOURCE_NO_VERSION
	}

	resources := strings.Split(path, "/")

	// Identify resource type
	finalRes := RESOURCE_NULL

	// Handle errors
	if len(resources) == 0 {
		// /v1/
		return nil, nil, RESOURCE_BAD_SLASH
	} else if len(resources)%2 == 1 {
		// Slash used for a document or end on a collection
		// /v1/db/doc/ or /v1/db/doc/col
		return nil, nil, RESOURCE_BAD_SLASH
	}

	// Identify the final resource
	// If the last element ends with a slash, then it must be a collection
	if len(resources) == 1 {
		// /v1/db
		finalRes = RESOURCE_DB_PD
	} else if len(resources) == 2 && resources[1] == "" {
		// /v1/db/
		finalRes = RESOURCE_DB
	} else if len(resources) > 2 && resources[len(resources)-1] == "" {
		finalRes = RESOURCE_COLL
	} else {
		finalRes = RESOURCE_DOC
	}

	// Iterate over path
	var lastColl *document.Collection = nil
	var lastDoc *document.Document = nil
	for i, resource := range resources {
		// Handle slash cases (blank)
		if resource == "" {
			if i != len(resources)-1 {
				// Not last; invalid resource name
				return nil, nil, RESOURCE_BLANK_PATHNAME
			}

			// Blank database put/delete
			if i == 0 {
				return nil, nil, RESOURCE_BAD_SLASH
			}

			// Error checking
			if lastColl == nil {
				slog.Error("GetResource: Returning NIL collection")
				return nil, nil, RESOURCE_INTERNAL
			}

			// Return a database or collection
			return lastColl, nil, finalRes
		}

		// Change behaviors depending on iteration
		if i == 0 {
			// Database
			lastColl, found = d.databases.CollectionFind(resource)
		} else if i%2 == 1 {
			// Document
			lastDoc, found = lastColl.DocumentFind(resource)
		} else if i > 0 && i%2 == 0 {
			// Collection
			lastColl, found = lastDoc.Children.CollectionFind(resource)
		}

		if !found {
			slog.Info("User could not find resource", "index", i, "resource", resource, "resources", resources)
			return nil, nil, -finalRes
		}
	}

	// End without a slash - either a db_pd or document
	if finalRes == RESOURCE_DB_PD {
		// Error check
		if lastColl == nil {
			slog.Error("GetResource: Returning NIL database")
			return nil, nil, RESOURCE_INTERNAL
		}

		return lastColl, nil, finalRes
	} else if finalRes == RESOURCE_DOC {
		// Error check
		if lastDoc == nil {
			slog.Error("GetResource: Returning NIL document")
			return nil, nil, RESOURCE_INTERNAL
		}

		return nil, lastDoc, finalRes
	} else {
		return nil, nil, RESOURCE_INTERNAL
	}

}

// Handle path errors returned from getResourceFromPath
func (d *Dbhandler) handlePathError(w http.ResponseWriter, r *http.Request, code int) {
	switch code {
	case RESOURCE_PUT_BAD_NAME:
		slog.Info("User used blank name", "path", r.URL.Path)
		msg := fmt.Sprintf("Blank name used for request: %s", r.URL.Path)
		http.Error(w, msg, http.StatusBadRequest)
	case RESOURCE_BAD_SLASH:
		// TODO: confirm this case (that it returns a bad request, not a not found)
		// /v1/a/b/ /v1/a /v1/a/b/c
		slog.Info("Missing collection or database slash", "path", r.URL.Path)
		msg := fmt.Sprintf("Bad slash: %s", r.URL.Path)
		http.Error(w, msg, http.StatusBadRequest)
	case RESOURCE_NO_VERSION:
		slog.Info("User path did not include version", "path", r.URL.Path)
		msg := fmt.Sprintf("path missing version: %s", r.URL.Path)
		http.Error(w, msg, http.StatusBadRequest)
	case RESOURCE_NO_DB:
		slog.Info("User attempted to access non-extant database", "path", r.URL.Path)
		msg := fmt.Sprintf("Database does not exist")
		http.Error(w, msg, http.StatusNotFound)
	case RESOURCE_NO_DOC:
		slog.Info("User attempted to access non-extant document", "path", r.URL.Path)
		msg := fmt.Sprintf("Document does not exist")
		http.Error(w, msg, http.StatusNotFound)
	case RESOURCE_NO_COLL:
		slog.Info("User attempted to access non-extant collection", "path", r.URL.Path)
		msg := fmt.Sprintf("Collection does not exist")
		http.Error(w, msg, http.StatusNotFound)
	case RESOURCE_DB:
		slog.Info("Invalid database resource for request", "path", r.URL.Path)
		msg := fmt.Sprintf("The method does not support database resource")
		http.Error(w, msg, http.StatusBadRequest)
	case RESOURCE_COLL:
		slog.Info("Invalid collection request for request", "path", r.URL.Path)
		msg := fmt.Sprintf("The method does not support collection resource")
		http.Error(w, msg, http.StatusBadRequest)
	case RESOURCE_DOC:
		slog.Info("Invalid document request for request", "path", r.URL.Path)
		msg := fmt.Sprintf("The method does not support document resource")
		http.Error(w, msg, http.StatusBadRequest)
	case RESOURCE_DB_PD:
		slog.Info("Invalid database (no slash) request for request", "path", r.URL.Path)
		msg := fmt.Sprintf("The method does not support database resource without slash")
		http.Error(w, msg, http.StatusBadRequest)
	case RESOURCE_BLANK_PATHNAME:
		slog.Info("Invalid path name (empty name for resource)", "path", r.URL.Path)
		msg := fmt.Sprintf("Invalid path name (empty name for resource)")
		http.Error(w, msg, http.StatusNotFound)
	default:
		slog.Info("Internal Error", "path", r.URL.Path)
		msg := fmt.Sprintf("ERROR: handlePath bad error code: %d", code)
		http.Error(w, msg, http.StatusInternalServerError)
	}
}

/*
Truncate a path's resource by one; that is, obtain the parent
of the specified resource.

On success, returns a new truncated path, the name of the resource
that was truncated, and the type of resource that was truncated.

On error, returns a resource error code.
*/
func cutRequest(request string) (truncatedRequest string, resourceName string, resourceType int) {
	// Check version
	path, found := strings.CutPrefix(request, "/v1/")
	if !found {
		return "", "", RESOURCE_NO_VERSION
	}

	resources := strings.Split(path, "/")

	// Identify resource type
	finalRes := RESOURCE_NULL

	// Handle errors and databases
	if len(resources) == 0 {
		// /v1/
		return "", "", RESOURCE_BAD_SLASH
	} else if len(resources) == 1 {
		// /v1/db
		return "", resources[0], RESOURCE_DB_PD
	} else if len(resources) == 2 && resources[1] == "" {
		// /v1/db/
		return "", resources[0], RESOURCE_DB
	} else if len(resources)%2 == 1 {
		// Slash used for a document or end on a collection
		// /v1/db/doc/ or /v1/db/doc/col
		return "", "", RESOURCE_BAD_SLASH
	}

	// Identify the final resource as a db or collection
	// If the last element ends with a slash, then it must be a collection
	li := strings.LastIndex(request, "/")
	resName := request[li+1:]
	if resources[len(resources)-1] == "" {
		// Collection - truncate by two
		// Goes to a document (do not include slash)
		li2 := strings.LastIndex(request[:li], "/")
		finalRes = RESOURCE_COLL
		resName = request[li2+1 : li]
		request = request[:li2]
	} else {
		// Document - truncate by one
		// Goes to collection (include slash)
		finalRes = RESOURCE_DOC
		request = request[:li+1]
	}
	slog.Info("Truncated resource path", "request", request, "resName", resName, "finalRes", finalRes)
	return request, resName, finalRes
}
