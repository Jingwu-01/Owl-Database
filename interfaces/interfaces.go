// Package interfaces contains interfaces of common data structures
// and optional behaviors of those data structures.
package interfaces

import (
	"net/http"

	"github.com/RICE-COMP318-FALL23/owldb-p1group20/patcher"
	"github.com/RICE-COMP318-FALL23/owldb-p1group20/structs"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

// The interface of a document.
//
// A document represents a document in a database.
type IDocument interface {
	// Get the docoutput resource (jsondoc + metadta) from this object
	GetRawBody() interface{}

	// Get the json document (underlying json document) from this object
	GetJSONDoc() interface{}

	// HTTP handler for GETs on document paths
	GetDocument(w http.ResponseWriter, r *http.Request)
}

// The interface of a collection.
//
// A collection represents a collection of documents in a database.
type ICollection interface {
	// Get a resource from this object
	FindDocument(resource string) (IDocument, bool)

	// HTTP handler for GETs on collections (query collection)
	GetDocuments(w http.ResponseWriter, r *http.Request)

	// HTTP handler for PUTs on document paths
	PutDocument(w http.ResponseWriter, r *http.Request, path string, newDoc IDocument)

	// HTTP handler for DELETEs on document paths
	DeleteDocument(w http.ResponseWriter, r *http.Request, docpath string)

	// HTTP handler for PATCH on document paths
	PatchDocument(w http.ResponseWriter, r *http.Request, docpath string, schema *jsonschema.Schema, name string)

	// HTTP handler for POST on document paths
	PostDocument(w http.ResponseWriter, r *http.Request, newDoc IDocument)
}

// The interface of a collection holder.
//
// A collection holder is used to store other collections.
type ICollectionHolder interface {
	// HTTP handler for PUTs on collection paths (manage collections)
	PutCollection(w http.ResponseWriter, r *http.Request, dbpath string, newColl ICollection)

	// Get a resource from this object
	GetCollection(resource string) (coll ICollection, found bool)

	// HTTP handler for DELETEs on collections (manage collections)
	DeleteCollection(w http.ResponseWriter, r *http.Request, dbpath string)
}

// An authenticator is something which can validate a login token
// as a valid user of a dbhandler or not.
type Authenticator interface {
	// ValidateToken tells if a token in a request is valid. Returns
	// true and the corresponding username if so, else writes an error to the input response writer.
	ValidateToken(w http.ResponseWriter, r *http.Request) (bool, string)
}

// A subscribable object allows the sending of messages to subscribers.
type Subscribable interface {
	// Notifies subscribers of update messages.
	NotifySubscribersUpdate(msg []byte, intervalComp string)

	// Notifies subscribers of delete messages.
	NotifySubscribersDelete(msg string, intervalComp string)
}

// A HasMetadata object allows storage and public retrieval of metadata
type HasMetadata interface {
	// Gets the original author of this document
	GetOriginalAuthor() string

	// Gets the last modified at field from
	// this document for conditional put.
	GetLastModified() int64
}

// A Patchable object allows patching
type Patchable interface {
	// Applys a slice of patches to this document.
	ApplyPatches(patches []patcher.Patch, schema *jsonschema.Schema) (structs.PatchResponse, interface{})

	// Overwrite the body of a document upon recieving a put or patch.
	OverwriteBody(docBody interface{}, name string)
}

// A overwritable object allows being overwritten
type Overwriteable interface {
	// Overwrite the body of a document upon recieving a put or patch.
	OverwriteBody(docBody interface{}, name string)
}
