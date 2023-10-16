// Package dbhandler provides structs for handling
// HTTP requests according to the owldb specifications.
// Supports GET, PUT, POST, PATCH, DELETE, and OPTIONS
// requests.
package dbhandler

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/RICE-COMP318-FALL23/owldb-p1group20/collection"
	"github.com/RICE-COMP318-FALL23/owldb-p1group20/collectionholder"
	"github.com/RICE-COMP318-FALL23/owldb-p1group20/document"
	"github.com/RICE-COMP318-FALL23/owldb-p1group20/interfaces"
	"github.com/RICE-COMP318-FALL23/owldb-p1group20/options"
	"github.com/RICE-COMP318-FALL23/owldb-p1group20/paths"
	"github.com/santhosh-tekuri/jsonschema/v5"
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
	databases     *collectionholder.CollectionHolder
	schema        *jsonschema.Schema
	authenticator Authenticator
}

// Creates a new DBHandler
func New(testmode bool, schema *jsonschema.Schema, authenticator Authenticator) Dbhandler {
	newHolder := collectionholder.New()
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
	slog.Info("Request being handled", "path", r.URL.Path)

	// Set headers of response
	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
	}
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

	// Action fork for GET Database and GET Document
	coll, doc, resc := paths.GetResourceFromPath(r.URL.Path, d.databases)
	switch resc {
	case paths.RESOURCE_DB:
		d.DatabaseGet(w, r, coll)
	case paths.RESOURCE_COLL:
		coll.CollectionGet(w, r)
	case paths.RESOURCE_DOC:
		doc.DocumentGet(w, r)
	default:
		paths.HandlePathError(w, r, resc)
	}
}

// Top-level PUT handler
//
// Handles PUT document, PUT database, or PUT collection.
// On success, puts the specified resource at the specified path.
func (d *Dbhandler) put(w http.ResponseWriter, r *http.Request, username string) {
	// Obtain parent resource to put to
	newRequest, newName, resc := paths.CutRequest(r.URL.Path)

	// PUT database
	if resc == paths.RESOURCE_DB_PD {
		d.DatabasePut(w, r, newName)
		return
	} else if resc == paths.RESOURCE_DB {
		paths.CustomPathError(w, r, "Bad syntax for PUT database (extra slash)", http.StatusBadRequest)
		return
	} else if resc <= 0 {
		paths.HandlePathError(w, r, resc)
		return
	}

	// Action fork for PUT Document and PUT Collection
	coll, doc, resc := paths.GetResourceFromPath(newRequest, d.databases)
	switch resc {
	case paths.RESOURCE_DB:
		// PUT document (in database)
		doc, err := d.createDocument(w, r, username)
		if err != nil {
			// handled in method
			return
		}
		coll.DocumentPut(w, r, newName, &doc)
	case paths.RESOURCE_COLL:
		// PUT document (in collection)
		doc, err := d.createDocument(w, r, username)
		if err != nil {
			// handled in method
			return
		}
		coll.DocumentPut(w, r, newName, &doc)
	case paths.RESOURCE_DOC:
		// PUT collection (in document)
		coll := collection.New()
		doc.CollectionPut(w, r, newName, &coll)
	default:
		paths.HandlePathError(w, r, resc)
	}
}

// Top-level delete resource handler
//
// Handles DELETE database, DELETE document, DELETE collection.
// On success, deletes the desired resource based on the specified path.
func (d *Dbhandler) delete(w http.ResponseWriter, r *http.Request) {
	// Obtain parent resource to delete the element from
	newRequest, newName, resc := paths.CutRequest(r.URL.Path)

	// DELETE database
	if resc == paths.RESOURCE_DB_PD {
		d.DatabaseDelete(w, r, newName)
		return
	} else if resc < 0 || resc == paths.RESOURCE_DB {
		paths.HandlePathError(w, r, resc)
		return
	}

	// Action fork for DELETE Document and DELETE Collection
	coll, doc, resc := paths.GetResourceFromPath(newRequest, d.databases)
	switch resc {
	case paths.RESOURCE_DB:
		// DELETE document (from database)
		coll.DocumentDelete(w, r, newName)
	case paths.RESOURCE_COLL:
		// delete a document from a collection
		coll.DocumentDelete(w, r, newName)
	case paths.RESOURCE_DOC:
		// delete a collection from a document
		doc.CollectionDelete(w, r, newName)
	default:
		paths.HandlePathError(w, r, resc)
	}
}

// Top-level POST resource handler
//
// Handles POST Database, POST Collection.
// On success, adds the requested document with a randomly generated name
// to a database or collection.
func (d *Dbhandler) post(w http.ResponseWriter, r *http.Request, username string) {
	// Action fork for POST Database and POST Collection
	coll, _, resc := paths.GetResourceFromPath(r.URL.Path, d.databases)
	switch resc {
	case paths.RESOURCE_DB:
		d.DatabasePost(w, r, coll, username)
	case paths.RESOURCE_COLL:
		doc, err := d.createDocument(w, r, username)
		if err != nil {
			// handled in method
			return
		}
		coll.DocumentPost(w, r, &doc)
	default:
		paths.HandlePathError(w, r, resc)
	}
}

// Top-level PATCH handler
//
// Handles PATCH database, PATCH collection
// On success, applies the desired patches.
func (d *Dbhandler) patch(w http.ResponseWriter, r *http.Request, name string) {
	// Patch requires the parent resource
	newRequest, newName, resc := paths.CutRequest(r.URL.Path)
	if resc < 0 {
		paths.HandlePathError(w, r, resc)
		return
	}

	// Action fork for PATCH document
	// Should go to parent first
	coll, _, resc := paths.GetResourceFromPath(newRequest, d.databases)
	switch resc {
	case paths.RESOURCE_DB:
		coll.DocumentPatch(w, r, newName, d.schema, name)
	case paths.RESOURCE_COLL:
		coll.DocumentPatch(w, r, newName, d.schema, name)
	default:
		paths.HandlePathError(w, r, resc)
	}
}

// Specific handler for GET database
func (d *Dbhandler) DatabaseGet(w http.ResponseWriter, r *http.Request, coll interfaces.ICollection) {
	// Same behavior as collection for now
	coll.CollectionGet(w, r)
}

// Specific handler for PUT database
func (d *Dbhandler) DatabasePut(w http.ResponseWriter, r *http.Request, dbpath string) {
	// Same behavior as collection for now
	coll := collection.New()
	d.databases.CollectionPut(w, r, dbpath, &coll)
}

// Specific handler for POST database
func (d *Dbhandler) DatabasePost(w http.ResponseWriter, r *http.Request, coll interfaces.ICollection, name string) {
	// Same behavior as collection for now
	doc, err := d.createDocument(w, r, name)
	if err != nil {
		// handled in method
		return
	}
	coll.DocumentPost(w, r, &doc)
}

// Specific handler for DELETE database
func (d *Dbhandler) DatabaseDelete(w http.ResponseWriter, r *http.Request, name string) {
	// Same behavior as collection for now
	d.databases.CollectionDelete(w, r, name)
}

// Creates a document object to insert into a collection.
func (d *Dbhandler) createDocument(w http.ResponseWriter, r *http.Request, name string) (document.Document, error) {
	var zero document.Document
	// Read body of requests
	desc, err := io.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		slog.Error("Post document: error reading the document request body", "error", err)
		http.Error(w, `"invalid document format"`, http.StatusBadRequest)
		return zero, err
	}

	// Read Body data
	var docBody map[string]interface{}
	err = json.Unmarshal(desc, &docBody)
	if err != nil {
		slog.Error("createReplaceDocument: error unmarshaling Post document request", "error", err)
		http.Error(w, `"invalid Post document format"`, http.StatusBadRequest)
		return zero, err
	}

	// Validate against schema
	err = d.schema.Validate(docBody)
	if err != nil {
		slog.Error("Post document: document did not conform to schema", "error", err)
		http.Error(w, `"document did not conform to schema"`, http.StatusBadRequest)
		return zero, err
	}

	return document.New(paths.GetRelativePathNonDB(r.URL.Path), name, docBody), nil
}
