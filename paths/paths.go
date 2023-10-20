// Package paths contains static utility methods for processing
// path name strings from requests.
package paths

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/RICE-COMP318-FALL23/owldb-p1group20/errorMessage"
	"github.com/RICE-COMP318-FALL23/owldb-p1group20/interfaces"
)

// Result codes from path resource operations.
//
// Indicates the type of resource obtained from a path or
// the type of error if there was an error obtaining a resource.
const (
	ERROR_BLANK_PATHNAME = -103
	ERROR_INTERNAL       = -102
	ERROR_BAD_SLASH      = -101
	ERROR_NO_VERSION     = -100
	ERROR_NO_DB          = -RESOURCE_DB
	ERROR_NO_COLL        = -RESOURCE_COLL
	ERROR_NO_DOC         = -RESOURCE_DOC

	RESOURCE_NULL  = 0
	RESOURCE_DB    = 1
	RESOURCE_COLL  = 2
	RESOURCE_DOC   = 3
	RESOURCE_DB_PD = 4 // specifically for put and delete db w/o slash
)

/*
Obtains resource from the specified path "request." Starts looking at the "root" collectioon holder.

On success, returns a collection if the path leads to a collection or a database,
or a document if the path leads to a document. Returns a result code indicating
the type of resource returned.

On error, returns a resource error code indicating the type of error found.
*/
func GetResourceFromPath(request string, root interfaces.ICollectionHolder) (interfaces.ICollection, interfaces.IDocument, int) {
	// Check version
	path, found := strings.CutPrefix(request, "/v1/")
	if !found {
		return nil, nil, ERROR_NO_VERSION
	}

	resources := strings.Split(path, "/")

	// Identify resource type
	finalRes := RESOURCE_NULL

	// Handle errors
	if len(resources) == 0 {
		// /v1/
		return nil, nil, ERROR_BAD_SLASH
	} else if len(resources)%2 == 1 {
		// Slash used for a document or end on a collection
		// /v1/db/doc/ or /v1/db/doc/col
		return nil, nil, ERROR_BAD_SLASH
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
	var lastColl interfaces.ICollection
	var lastDoc interfaces.IDocument
	for i, resource := range resources {
		// Handle slash cases (blank)
		if resource == "" {
			if i != len(resources)-1 {
				// Not last; invalid resource name
				return nil, nil, ERROR_BLANK_PATHNAME
			}

			// Blank database put/delete
			if i == 0 {
				return nil, nil, ERROR_BAD_SLASH
			}

			// Error checking
			if lastColl == nil {
				slog.Error("GetResource: Returning NIL collection")
				return nil, nil, ERROR_INTERNAL
			}

			// Return a database or collection
			return lastColl, nil, finalRes
		}

		// Change behaviors depending on iteration
		if i == 0 {
			// Database
			lastColl, found = root.FindCollection(resource)
		} else if i%2 == 1 {
			// Document
			lastDoc, found = lastColl.FindDocument(resource)
		} else if i > 0 && i%2 == 0 {
			// Collection
			lastColl, found = lastDoc.GetCollection(resource)
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
			return nil, nil, ERROR_INTERNAL
		}

		return lastColl, nil, finalRes
	} else if finalRes == RESOURCE_DOC {
		// Error check
		if lastDoc == nil {
			slog.Error("GetResource: Returning NIL document")
			return nil, nil, ERROR_INTERNAL
		}

		return nil, lastDoc, finalRes
	} else {
		return nil, nil, ERROR_INTERNAL
	}

}

/*
Truncate a path's resource by one; that is, obtain the parent
of the specified resource.

On success, returns a new truncated path, the name of the resource
that was truncated, and the type of resource that was truncated.

On error, returns a resource error code.
*/
func CutRequest(request string) (truncatedRequest string, resourceName string, resourceType int) {
	// Check version
	path, found := strings.CutPrefix(request, "/v1/")
	if !found {
		return "", "", ERROR_NO_VERSION
	}

	resources := strings.Split(path, "/")

	// Identify resource type
	finalRes := RESOURCE_NULL

	// Handle errors and databases
	if len(resources) == 0 {
		// /v1/
		return "", "", ERROR_BAD_SLASH
	} else if len(resources) == 1 {
		// /v1/db
		return "", resources[0], RESOURCE_DB_PD
	} else if len(resources) == 2 && resources[1] == "" {
		// /v1/db/
		return "", resources[0], RESOURCE_DB
	} else if len(resources)%2 == 1 {
		// Slash used for a document or end on a collection
		// /v1/db/doc/ or /v1/db/doc/col
		return "", "", ERROR_BAD_SLASH
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

// Generic path error handler.
//
// For a given result code, logs the error and sends a predefined message to the client.
func HandlePathError(w http.ResponseWriter, r *http.Request, code int) {
	switch code {
	case ERROR_BAD_SLASH:
		slog.Info("Invalid path: malformed slash.", "path", r.URL.Path)
		msg := fmt.Sprintf("Invalid path: malformed slash in pathname.")
		errorMessage.ErrorResponse(w, msg, http.StatusBadRequest)
	case ERROR_NO_VERSION:
		slog.Info("Invalid path: did not include version", "path", r.URL.Path)
		msg := fmt.Sprintf("Invalid path: did not include version.")
		errorMessage.ErrorResponse(w, msg, http.StatusBadRequest)
	case ERROR_NO_DB:
		slog.Info("User attempted to access non-extant database", "path", r.URL.Path)
		msg := fmt.Sprintf("Invalid path: could not find resource.")
		errorMessage.ErrorResponse(w, msg, http.StatusNotFound)
	case ERROR_NO_DOC:
		slog.Info("User attempted to access non-extant document", "path", r.URL.Path)
		msg := fmt.Sprintf("Invalid path: could not find resource.")
		errorMessage.ErrorResponse(w, msg, http.StatusNotFound)
	case ERROR_NO_COLL:
		slog.Info("User attempted to access non-extant collection", "path", r.URL.Path)
		msg := fmt.Sprintf("Invalid path: could not find resource.")
		errorMessage.ErrorResponse(w, msg, http.StatusNotFound)
	case RESOURCE_DB:
		slog.Info("Invalid database resource for request", "path", r.URL.Path)
		msg := fmt.Sprintf("Invalid request: request does not support databases")
		errorMessage.ErrorResponse(w, msg, http.StatusBadRequest)
	case RESOURCE_COLL:
		slog.Info("Invalid collection request for request", "path", r.URL.Path)
		msg := fmt.Sprintf("Invalid request: request does not support collections")
		errorMessage.ErrorResponse(w, msg, http.StatusBadRequest)
	case RESOURCE_DOC:
		slog.Info("Invalid document request for request", "path", r.URL.Path)
		msg := fmt.Sprintf("Invalid request: request does not support documents")
		errorMessage.ErrorResponse(w, msg, http.StatusBadRequest)
	case RESOURCE_DB_PD:
		slog.Info("Invalid database (no slash) request for request", "path", r.URL.Path)
		msg := fmt.Sprintf("Invalid request: invalid syntax for database or does not support database.")
		errorMessage.ErrorResponse(w, msg, http.StatusBadRequest)
	case ERROR_BLANK_PATHNAME:
		slog.Info("Invalid path name (empty name for resource)", "path", r.URL.Path)
		msg := fmt.Sprintf("Invalid path: empty name for resource")
		errorMessage.ErrorResponse(w, msg, http.StatusNotFound)
	default:
		slog.Info("Internal Error: unhandled error code", "path", r.URL.Path, "code", code)
		msg := fmt.Sprintf("ERROR: handlePath unhandled error code: %d", code)
		errorMessage.ErrorResponse(w, msg, http.StatusInternalServerError)
	}
}

// Takes a path with a /v1/db/<path> and removes
// the /v1/db/.
func GetRelativePathNonDB(path string) string {
	splitpath := strings.SplitAfterN(path, "/", 4)
	return "/" + splitpath[3]
}

// Takes a path with a /v1/db and removes
// the /v1.
func GetRelativePathDB(path string) string {
	trimmedpath := strings.TrimPrefix(path, "/v1")
	return trimmedpath
}
