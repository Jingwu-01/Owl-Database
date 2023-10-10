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

// Constants for getResourceFromPath
const (
	INVALID_OPERATION     = -8
	RESOURCE_PUT_BAD_NAME = -7
	RESOURCE_INTERNAL     = -6
	RESOURCE_BAD_SLASH    = -5
	RESOURCE_NO_VERSION   = -4
	RESOURCE_NO_DB        = -RESOURCE_DB
	RESOURCE_NO_COLL      = -RESOURCE_COLL
	RESOURCE_NO_DOC       = -RESOURCE_DOC

	RESOURCE_DB   = 1
	RESOURCE_COLL = 2
	RESOURCE_DOC  = 3
)

// A putoutput stores the response to a put request.
type putoutput struct {
	Uri string `json:"uri"`
}

// An authenticator is something which can validate a login token as one supported
// by a dbhandler or not.
type Authenticator interface {
	ValidateToken(w http.ResponseWriter, r *http.Request) bool
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
		options.Options(w, r)
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
	// Check if we are in the subscribe mode
	mode := r.URL.Query().Get("mode")
	if mode == "subscribe" {
		subscribe.New().ServeHTTP(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

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

// Handles case where we have PUT request by either
// putting a new document or database at the desired
// location.
func (d *Dbhandler) Put(w http.ResponseWriter, r *http.Request) {
	// Set headers of response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Obtain parent resource to put to
	newRequest, newName, resc := cutRequest(r.URL.Path)
	if resc == RESOURCE_DB {
		d.DatabasePut(w, r, newName)
		return
	} else if resc < 0 {
		d.handlePathError(w, r, resc)
		return
	}

	// Action fork for PUT Document and PUT Collection
	coll, doc, resc := d.getResourceFromPath(newRequest)
	switch resc {
	case RESOURCE_DB:
		// should never happen (already handled)
		d.handlePathError(w, r, RESOURCE_INTERNAL)
	case RESOURCE_COLL:
		// put a document to a collection
		coll.DocumentPut(w, r, newName, d.schema)
	case RESOURCE_DOC:
		// put a collection to a document
		doc.Children.CollectionPut(w, r, newName)
	default:
		d.handlePathError(w, r, resc)
	}
}

// Handles a DELETE request either by logging out the user
// if they use the /auth path, and otherwise by deleting
// the desired database or document.
func (d *Dbhandler) Delete(w http.ResponseWriter, r *http.Request) {
	// Set headers of response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Obtain parent resource to delete the element from
	newRequest, newName, resc := cutRequest(r.URL.Path)
	if resc == RESOURCE_DB {
		d.DatabaseDelete(w, r, newName)
		return
	} else if resc < 0 {
		d.handlePathError(w, r, resc)
		return
	}

	// Action fork for DELETE Document and DELETE Collection
	coll, doc, resc := d.getResourceFromPath(newRequest)
	switch resc {
	case RESOURCE_DB:
		// should never happen (already handled)
		d.handlePathError(w, r, RESOURCE_INTERNAL)
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

// Handles a POST request either by logging in the user or
// by adding a document to the desired top level db with a
// random name.
func (d *Dbhandler) Post(w http.ResponseWriter, r *http.Request) {
	// Set headers of response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Action fork for POST Database and POST Collection
	coll, _, resc := d.getResourceFromPath(r.URL.Path)
	switch resc {
	case RESOURCE_DB:
		d.DatabasePost(w, r, coll)
	case RESOURCE_COLL:
		coll.DocumentPost(w, r, d.schema)
	case RESOURCE_DOC:
		d.handlePathError(w, r, INVALID_OPERATION)
	default:
		d.handlePathError(w, r, resc)
	}
}

// Handles a PATCH request by finding the proper document
// and applying the desired patches.
func (d *Dbhandler) Patch(w http.ResponseWriter, r *http.Request) {
	// Set headers of response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

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
		coll.DocumentPatch(w, r, newName, d.schema)
	case RESOURCE_COLL:
		coll.DocumentPatch(w, r, newName, d.schema)
	case RESOURCE_DOC:
		d.handlePathError(w, r, INVALID_OPERATION)
	default:
		d.handlePathError(w, r, resc)
	}
}

// Handles top level database gets
func (d *Dbhandler) DatabaseGet(w http.ResponseWriter, r *http.Request, coll *document.Collection) {
	// Same behavior as collection for now
	coll.CollectionGet(w, r)
}

// Handles top level database puts
func (d *Dbhandler) DatabasePut(w http.ResponseWriter, r *http.Request, dbpath string) {
	// Same behavior as collection for now
	d.databases.CollectionPut(w, r, dbpath)
}

// Handles top level database posts
func (d *Dbhandler) DatabasePost(w http.ResponseWriter, r *http.Request, coll *document.Collection) {
	// Same behavior as collection for now
	coll.DocumentPost(w, r, d.schema)
}

// Delete a top level database
func (d *Dbhandler) DatabaseDelete(w http.ResponseWriter, r *http.Request, name string) {
	// Same behavior as collection for now
	d.databases.CollectionDelete(w, r, name)
}

// Obtains the last resource at the end of the path string
// if cut == true, then return a preceding resource
// returns a collection or document based on the last resource of the path
// from this path or a negative error code. This path must include the version
func (d *Dbhandler) getResourceFromPath(request string) (*document.Collection, *document.Document, int) {
	// Check version
	path, found := strings.CutPrefix(request, "/v1/")
	if !found {
		return nil, nil, RESOURCE_NO_VERSION
	}

	resources := strings.Split(path, "/")
	if len(resources) <= 1 {
		// /v1/ or /v1/a
		return nil, nil, RESOURCE_BAD_SLASH
	} else if len(resources)%2 == 1 {
		// Slash used for a document or end on a collection
		return nil, nil, RESOURCE_BAD_SLASH
	}

	// Identify the final resource
	// If the last element ends with a slash, then it must be a collection/database
	finalRes := RESOURCE_DOC
	if resources[len(resources)-1] == "" {
		if len(resources) == 2 {
			finalRes = RESOURCE_DB
		} else {
			finalRes = RESOURCE_COLL
		}
	}

	// Iterate over path
	var lastColl *document.Collection = nil
	var lastDoc *document.Document = nil
	for i, resource := range resources {
		// Handle slash cases (blank)
		if resource == "" {
			if i != len(resources)-1 {
				// Not last; invalid resource name
				return nil, nil, -finalRes
			} else {
				// Error checking
				if lastColl == nil {
					return nil, nil, RESOURCE_INTERNAL
				}

				// Return a database or collection
				return lastColl, nil, finalRes
			}
		}

		// Change behaviors depending on iteration
		if i == 0 {
			// Database
			lastColl, found = d.databases.Collections.Find(resource)
		} else if i%2 == 1 {
			// Document
			lastDoc, found = lastColl.Documents.Find(resource)
		} else if i > 0 && i%2 == 0 {
			// Collection
			lastColl, found = lastDoc.Children.Collections.Find(resource)
		}

		if found {
			return nil, nil, -finalRes
		}
	}

	if lastDoc == nil {
		return nil, nil, RESOURCE_INTERNAL
	}

	// This should always be a document
	return nil, lastDoc, finalRes
}

// Handle path errors returned from getResourceFromPath
// Note the error messages here reflect the type of request by the user,
// not the type of error from getResourceFromPath.
func (d *Dbhandler) handlePathError(w http.ResponseWriter, r *http.Request, code int) {
	switch code {
	case INVALID_OPERATION:
		slog.Info("Invalid operation for request", "operation", r.Method)
		msg := fmt.Sprintf("Invalid operation for request %s", r.Method)
		http.Error(w, msg, http.StatusBadRequest)
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
	default:
		slog.Info("Internal Error", "path", r.URL.Path)
		msg := fmt.Sprintf("ERROR: handlePath bad error code: %d", code)
		http.Error(w, msg, http.StatusInternalServerError)
	}
}

// Truncate a path's resource by one
func cutRequest(request string) (string, string, int) {
	// Check version
	path, found := strings.CutPrefix(request, "/v1/")
	if !found {
		return "", "", RESOURCE_NO_VERSION
	}

	resources := strings.Split(path, "/")
	if len(resources) <= 1 {
		// /v1/ or /v1/a
		return "", "", RESOURCE_BAD_SLASH
	} else if len(resources)%2 == 1 {
		// Slash used for a document or end on a collection
		return "", "", RESOURCE_BAD_SLASH
	}

	// Identify the final resource
	// If the last element ends with a slash, then it must be a collection/database
	finalRes := RESOURCE_DOC
	if resources[len(resources)-1] == "" {
		if len(resources) == 2 {
			finalRes = RESOURCE_DB
		} else {
			finalRes = RESOURCE_COLL
		}
	}

	// cut: obtain the preceding resource
	li := strings.LastIndex(request, "/")
	resName := request[li:]
	if finalRes == RESOURCE_DB {
		return "", resName, RESOURCE_DB
	} else if finalRes == RESOURCE_COLL {
		// Truncate by two
		li2 := strings.LastIndex(request[:li], "/")
		resName = request[li2+1 : li]
		request = request[:li2]
	} else if finalRes == RESOURCE_DOC {
		// Truncate by one
		request = request[:li]
	}
	return request, resName, finalRes
}
