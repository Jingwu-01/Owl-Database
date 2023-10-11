package document

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/RICE-COMP318-FALL23/owldb-p1group20/skiplist"
)

/*
A collectionholder is a skiplist that holds or other collections
*/
type CollectionHolder struct {
	Collections *skiplist.SkipList[string, *Collection]
}

// Creates a new collectioon holder
func NewHolder() CollectionHolder {
	newSL := skiplist.New[string, *Collection](skiplist.STRING_MIN, skiplist.STRING_MAX, skiplist.DEFAULT_LEVEL)
	return CollectionHolder{&newSL}
}

func (c *CollectionHolder) CollectionPut(w http.ResponseWriter, r *http.Request, dbpath string) {
	// Add a new database to dbhandler if it is not already there; otherwise error
	// Define the upsert method - only create a new collection
	dbUpsert := func(key string, currValue *Collection, exists bool) (*Collection, error) {
		if exists {
			return nil, errors.New("Database already exists")
		} else {
			newColl := NewCollection()
			return &newColl, nil
		}
	}

	_, err := c.Collections.Upsert(dbpath, dbUpsert)
	// Handle errors
	if err != nil {
		slog.Error(err.Error())
		switch err.Error() {
		case "Database already exists":
			http.Error(w, err.Error(), http.StatusBadRequest)
		default:
			http.Error(w, "PUT() error "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Put success
	jsonResponse, err := json.Marshal(putoutput{r.URL.Path})
	if err != nil {
		// This should never happen
		slog.Error("PUT: marshal error", "error", err)
		http.Error(w, `"internal server error"`, http.StatusInternalServerError)
		return
	}
	slog.Info("Created Database", "path", dbpath)
	w.Header().Set("Location", r.URL.Path)
	w.WriteHeader(http.StatusCreated)
	w.Write(jsonResponse)
	return
}

func (c *CollectionHolder) CollectionDelete(w http.ResponseWriter, r *http.Request, dbpath string) {
	// Just request a delete on the specified element
	_, deleted := c.Collections.Remove(dbpath)

	// Handle response
	if !deleted {
		slog.Info("Collection does not exist", "path", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	slog.Info("Deleted Collection", "path", r.URL.Path)
	w.Header().Set("Location", r.URL.Path)
	w.WriteHeader(http.StatusNoContent)
}