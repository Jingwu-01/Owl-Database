// Package interfaces contains interfaces of common data structures.
package interfaces

import (
	"net/http"

	"github.com/RICE-COMP318-FALL23/owldb-p1group20/patcher"
	"github.com/RICE-COMP318-FALL23/owldb-p1group20/structs"
	"github.com/RICE-COMP318-FALL23/owldb-p1group20/subscribe"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

// The interface of a document.
//
// A document represents a document in a database.
type IDocument interface {
	DocumentGet(w http.ResponseWriter, r *http.Request)
	CollectionPut(w http.ResponseWriter, r *http.Request, newName string, newColl ICollection)
	CollectionDelete(w http.ResponseWriter, r *http.Request, newName string)
	CollectionFind(resource string) (ICollection, bool)
	Overwrite(docBody interface{}, name string)
	ApplyPatches(patches []patcher.Patch, schema *jsonschema.Schema) (structs.PatchResponse, interface{})
	GetLastModified() int64
	GetOriginalAuthor() string
	GetJSONBody() ([]byte, error)
	GetRawBody() interface{}
	GetDoc() interface{}
	GetSubscribers() []subscribe.Subscriber
}

// The interface of a collection.
//
// A collection represents a collection of documents in a database.
type ICollection interface {
	CollectionGet(w http.ResponseWriter, r *http.Request)
	DocumentPut(w http.ResponseWriter, r *http.Request, path string, newDoc IDocument)
	DocumentDelete(w http.ResponseWriter, r *http.Request, docpath string)
	DocumentPatch(w http.ResponseWriter, r *http.Request, docpath string, schema *jsonschema.Schema, name string)
	DocumentPost(w http.ResponseWriter, r *http.Request, newDoc IDocument)
	DocumentFind(resource string) (IDocument, bool)
	GetSubscribers() []structs.CollSub
}

// The interface of a collection holder.
//
// A collection holder is used to store other collections.
type ICollectionHolder interface {
	CollectionPut(w http.ResponseWriter, r *http.Request, dbpath string, newColl ICollection)
	CollectionDelete(w http.ResponseWriter, r *http.Request, dbpath string)
	CollectionFind(resource string) (coll ICollection, found bool)
}
