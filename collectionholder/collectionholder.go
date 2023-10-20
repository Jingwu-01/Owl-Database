package collectionholder

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/RICE-COMP318-FALL23/owldb-p1group20/errorMessage"
	"github.com/RICE-COMP318-FALL23/owldb-p1group20/interfaces"
	"github.com/RICE-COMP318-FALL23/owldb-p1group20/skiplist"
	"github.com/RICE-COMP318-FALL23/owldb-p1group20/structs"
)

/*
A collectionholder adds functionality to a concurrent skiplist
that holds collections, which is sorted by collection name.
*/
type CollectionHolder struct {
	collections *skiplist.SkipList[string, interfaces.ICollection] // The internal skiplist representation.
}

// Creates a new collection holder.
func New() CollectionHolder {
	newSL := skiplist.New[string, interfaces.ICollection](skiplist.STRING_MIN, skiplist.STRING_MAX, skiplist.DEFAULT_LEVEL)
	return CollectionHolder{&newSL}
}

// Create a new collection inside this CollectionHolder.
func (c *CollectionHolder) PutCollection(w http.ResponseWriter, r *http.Request, dbpath string, newColl interfaces.ICollection) {
	// Add a new database to dbhandler if it is not already there; otherwise error
	// Define the upsert method - only create a new collection
	dbUpsert := func(key string, currValue interfaces.ICollection, exists bool) (interfaces.ICollection, error) {
		if exists {
			return nil, errors.New("db exist")
		} else {
			return newColl, nil
		}
	}

	_, err := c.collections.Upsert(dbpath, dbUpsert)
	// Handle errors
	if err != nil {
		slog.Error(err.Error())
		switch err.Error() {
		case "db exist":
			errorMessage.ErrorResponse(w, "Database already exists", http.StatusBadRequest)
		default:
			errorMessage.ErrorResponse(w, "PUT() error "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Put success
	jsonResponse, err := json.Marshal(structs.PutOutput{Uri: r.URL.Path})
	if err != nil {
		// This should never happen
		slog.Error("PUT: marshal error", "error", err)
		errorMessage.ErrorResponse(w, "internal server error", http.StatusInternalServerError)
		return
	}
	slog.Info("Created Database", "path", dbpath)
	w.Header().Set("Location", r.URL.Path)
	w.WriteHeader(http.StatusCreated)
	w.Write(jsonResponse)
}

// Deletes a collection inside this CollectionHolder.
func (c *CollectionHolder) DeleteCollection(w http.ResponseWriter, r *http.Request, dbpath string) {
	// Just request a delete on the specified element
	col, deleted := c.collections.Remove(dbpath)

	// Handle response
	if !deleted {
		slog.Info("Collection does not exist", "path", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Notify subscribers
	// Notify collection subscribers
	// TODO: does not use interval?
	colsub, ok := interface{}(col).(interfaces.Subscribable)
	if ok {
		colsub.NotifySubscribersDelete(r.URL.Path, "")
	}

	slog.Info("Deleted Collection", "path", r.URL.Path)
	w.Header().Set("Location", r.URL.Path)
	w.WriteHeader(http.StatusNoContent)
}

// Find a collection in this collection holder.
func (c *CollectionHolder) GetCollection(resource string) (coll interfaces.ICollection, found bool) {
	return c.collections.Find(resource)
}
