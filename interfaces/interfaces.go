package interfaces

import (
	"net/http"

	"github.com/RICE-COMP318-FALL23/owldb-p1group20/patcher"
	"github.com/RICE-COMP318-FALL23/owldb-p1group20/structs"
	"github.com/RICE-COMP318-FALL23/owldb-p1group20/subscribe"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

type IDocument interface {
	DocumentGet(w http.ResponseWriter, r *http.Request)
	CollectionPut(w http.ResponseWriter, r *http.Request, newName string)
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

type ICollection interface {
}
