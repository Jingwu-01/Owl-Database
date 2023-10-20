// Package interfaces contains interfaces of common data structures.
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
	GetDocument(w http.ResponseWriter, r *http.Request)
	PutCollection(w http.ResponseWriter, r *http.Request, newName string, newColl ICollection)
	DeleteCollection(w http.ResponseWriter, r *http.Request, newName string)
	GetCollection(resource string) (ICollection, bool)
	OverwriteBody(docBody interface{}, name string)
	ApplyPatches(patches []patcher.Patch, schema *jsonschema.Schema) (structs.PatchResponse, interface{})
	GetLastModified() int64
	GetOriginalAuthor() string
	GetJSONBody() ([]byte, error)
	GetRawBody() interface{}
	GetDoc() interface{}
}

// The interface of a collection.
//
// A collection represents a collection of documents in a database.
type ICollection interface {
	GetCollection(w http.ResponseWriter, r *http.Request)
	PutDocument(w http.ResponseWriter, r *http.Request, path string, newDoc IDocument)
	DeleteDocument(w http.ResponseWriter, r *http.Request, docpath string)
	PatchDocument(w http.ResponseWriter, r *http.Request, docpath string, schema *jsonschema.Schema, name string)
	PostDocument(w http.ResponseWriter, r *http.Request, newDoc IDocument)
	FindDocument(resource string) (IDocument, bool)
}

// The interface of a collection holder.
//
// A collection holder is used to store other collections.
type ICollectionHolder interface {
	PutCollection(w http.ResponseWriter, r *http.Request, dbpath string, newColl ICollection)
	DeleteCollection(w http.ResponseWriter, r *http.Request, dbpath string)
	FindCollection(resource string) (coll ICollection, found bool)
}

// An authenticator is something which can validate a login token
// as a valid user of a dbhandler or not.
type Authenticator interface {
	ValidateToken(w http.ResponseWriter, r *http.Request) (bool, string)
}

// A subscribable object allows the sending of messages to subscribers.
type Subscribable interface {
	NotifySubscribersUpdate(msg []byte, intervalComp string)
	NotifySubscribersDelete(msg string, intervalComp string)
}
