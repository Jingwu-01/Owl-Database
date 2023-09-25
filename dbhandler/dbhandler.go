// A package implementing the highest level handling for
// the OwlDB project.
package dbhandler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/RICE-COMP318-FALL23/owldb-p1group20/collection"
	"github.com/RICE-COMP318-FALL23/owldb-p1group20/document"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

// A putoutput stores the response to a put request.
type putoutput struct {
	Uri string `json:"uri"`
}

// A dbhandler is the highest level struct, holds all the collections and
// handles all the http requests.
type Dbhandler struct {
	databases *sync.Map
	schema    *jsonschema.Schema
	sessions  *sync.Map
}

// Creates a new DBHandler
func New(testmode bool, schema *jsonschema.Schema) Dbhandler {
	retval := Dbhandler{&sync.Map{}, schema, &sync.Map{}}

	if testmode {
		slog.Info("Test mode enabled", "INFO", 0)

		// The current test cases will have
		retval.databases.Store("db1", collection.New())
		retval.databases.Store("db2", collection.New())
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
	//case http.MethodPatch:
	// Patch handling
	case http.MethodDelete:
		d.Delete(w, r)
	case http.MethodOptions:
		d.Options(w, r)
	default:
		// If user used method we do not support.
		slog.Info("User used unsupported method", "method", r.Method)
		msg := fmt.Sprintf("unsupported method: %s", r.Method)
		http.Error(w, msg, http.StatusBadRequest)
	}
}

// Handles case where we recieve a GET request.
func (d *Dbhandler) Get(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Check for version
	path, found := strings.CutPrefix(r.URL.Path, "/v1/")
	if !found {
		slog.Info("User path did not include version", "path", r.URL.Path)
		msg := fmt.Sprintf("path missing version: %s", r.URL.Path)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	// Action fork for GET Database and GET Document
	// Only compliant for check-in.
	// Checkpoint 1 heirarchy
	// dbhandler -> databases (map) -> collection -> documents (map) -> document -> docoutput (metadata, path, contents)
	splitpath := strings.SplitAfterN(path, "/", 2)
	if len(splitpath) == 1 {
		// Error, DB path does not end with "/"
		slog.Info("DB path did not end with '/'", "path", r.URL.Path)
		msg := fmt.Sprintf("path missing trailing '/': %s", r.URL.Path)
		http.Error(w, msg, http.StatusBadRequest)
		return
	} else if splitpath[1] == "" {
		// GET Database
		dbpath, _ := strings.CutSuffix(splitpath[0], "/")

		// Access the database
		database, ok := d.databases.Load(dbpath)

		// Check to see if database exists
		if !ok {
			slog.Info("User attempted to access non-extant database", "db", dbpath)
			msg := fmt.Sprintf("Database does not exist")
			http.Error(w, msg, http.StatusNotFound)
			return
		}

		database.(collection.Collection).CollectionGet(w, r)

	} else {
		// GET Document
		dbpath, _ := strings.CutSuffix(splitpath[0], "/")
		path = splitpath[1]

		// Access the database
		database, ok := d.databases.Load(dbpath)

		// Check to see if database exists
		if !ok {
			slog.Info("User attempted to access non-extant database", "db", dbpath)
			msg := fmt.Sprintf("Document does not exist")
			http.Error(w, msg, http.StatusNotFound)
			return
		}

		// Get document
		// for checkpoint 1, we assume that path will always be a document name
		doc, ok := database.(collection.Collection).Documents.Load(path)
		if !ok {
			slog.Info("User attempted to access non-extant document", "doc", path)
			msg := fmt.Sprintf("Document does not exist")
			http.Error(w, msg, http.StatusNotFound)
			return
		}

		doc.(document.Document).DocumentGet(w, r)
	}
}

// Handles case where we have PUT request.
func (d *Dbhandler) Put(w http.ResponseWriter, r *http.Request) {
	// Set headers of response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	path, found := strings.CutPrefix(r.URL.Path, "/v1/")

	// Check for version
	if !found {
		slog.Info("User path did not include version", "path", path)
		msg := fmt.Sprintf("path missing version: %s", path)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	splitpath := strings.SplitAfterN(path, "/", 2)

	if len(splitpath) == 1 {
		// PUT database case
		dbpath := splitpath[0]

		d.putDB(w, r, dbpath)
	} else {
		// PUT document or collection
		dbpath, _ := strings.CutSuffix(splitpath[0], "/")

		// Access the database
		database, ok := d.databases.Load(dbpath)

		// Check to see if database exists
		if !ok {
			slog.Info("User attempted to access non-extant database", "db", dbpath)
			msg := fmt.Sprintf("Database does not exist")
			http.Error(w, msg, http.StatusNotFound)
			return
		}

		database.(collection.Collection).DocumentPut(w, r, splitpath[1], d.schema)
	}
}

// Puts a new top level database into our handler
func (d *Dbhandler) putDB(w http.ResponseWriter, r *http.Request, dbpath string) {
	// Add a new database to dbhandler if it is not already there; otherwise, return error. (I assumed database and collection use the same struct).
	_, loaded := d.databases.LoadOrStore(dbpath, collection.New())
	if loaded {
		slog.Error("Database already exists")
		http.Error(w, "Database already exists", http.StatusBadRequest)
		return
	} else {
		jsonResponse, err := json.Marshal(putoutput{r.URL.Path})
		if err != nil {
			// This should never happen
			slog.Error("Get: error marshaling", "error", err)
			http.Error(w, `"internal server error"`, http.StatusInternalServerError)
			return
		}
		slog.Info("Created Database", "path", dbpath)
		w.Header().Set("Location", r.URL.Path)
		w.WriteHeader(http.StatusCreated)
		w.Write(jsonResponse)
		return
	}
}

// Handles case where we have DELETE request.
func (d *Dbhandler) Delete(w http.ResponseWriter, r *http.Request) {

	if strings.Contains(r.URL.Path, "auth") {
		// Logout case
		isValidToken := validateToken(d, w, r)
		if isValidToken {
			// Remove the corresponding userInfo from the sessions map
			d.sessions.Delete(r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusNoContent)
			slog.Info("user is successfully removed")
			return
		}
	} else {
		// Set headers of response
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		path, found := strings.CutPrefix(r.URL.Path, "/v1/")

		// Check for version
		if !found {
			slog.Info("User path did not include version", "path", path)
			msg := fmt.Sprintf("path missing version: %s", path)
			http.Error(w, msg, http.StatusBadRequest)
			return
		}

		splitpath := strings.SplitAfterN(path, "/", 2)

		dbpath := splitpath[0]

		// Check to see if database exists
		database, ok := d.databases.Load(dbpath)
		// If the database does not exist, return StatusNotFound error
		if !ok {
			slog.Info("User attempted to access non-extant database", "db", dbpath)
			msg := fmt.Sprintf("Database does not exist")
			http.Error(w, msg, http.StatusNotFound)
			return
		}

		if len(splitpath) == 1 {
			// DELETE database case
			d.databases.Delete(dbpath)
			slog.Info("Deleted Database", "path", dbpath)
			w.Header().Set("Location", r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
			return

		} else {
			// DELETE document case

			// Decode the document name
			docpath, _ := strings.CutSuffix(splitpath[1], "/")

			// Access the document
			_, exist := database.(collection.Collection).Documents.Load(docpath)
			if exist {
				// Document found
				database.(collection.Collection).Documents.Delete(docpath)
				slog.Info("Deleted Document", "path", path)
				w.Header().Set("Location", r.URL.Path)
				w.WriteHeader(http.StatusNoContent)
				return
			} else {
				// Document not found
				slog.Error("Document does not exist")
				http.Error(w, "Document does not exist", http.StatusNotFound)
				return
			}
		}
	}
}

// Handles case where we have POST request.
func (d *Dbhandler) Post(w http.ResponseWriter, r *http.Request) {

	if strings.Contains(r.URL.Path, "auth") {
		// Login request case

		// Set headers of response
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		// Read body of requests
		desc, err := io.ReadAll(r.Body)
		defer r.Body.Close()
		if err != nil {
			slog.Error("Login: error reading the request body", "error", err)
			http.Error(w, `"invalid login format"`, http.StatusBadRequest)
			return
		}

		// Read Body data
		var userInfo map[string]string
		err = json.Unmarshal(desc, &userInfo)
		if err != nil {
			slog.Error("Login: error unmarshaling request", "error", err)
			http.Error(w, `"invalid login format"`, http.StatusBadRequest)
			return
		}

		// Validate against schema
		err = d.schema.Validate(userInfo)
		if err != nil {
			slog.Error("Login: request body did not conform to schema", "error", err)
			http.Error(w, `"Login: request body did not conform to schema"`, http.StatusBadRequest)
			return
		}

		// Generate a secure, random token
		token, err := generateToken()
		if err != nil {
			slog.Error("Login: token not successfully generated", "error", err)
			http.Error(w, "Login: token not successfully generated", http.StatusInternalServerError)
			return
		}

		// Store username and token in a session map with expiration time
		username := userInfo["username"]
		d.sessions.Store(token, SessionInfo{Username: username, ExpiresAt: time.Now().Add(1 * time.Hour)})

		// Return the token to the user
		jsonToken, err := json.Marshal(map[string]string{"token": token})
		if err != nil {
			// This should never happen
			slog.Error("Login: error marshaling", "error", err)
			http.Error(w, `"internal server error"`, http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(jsonToken)
		slog.Info("Login: success")
	} else {
		// Handle other cases
	}
}

// generateToken is a helper function that generates a secure, random token
func generateToken() (string, error) {
	// 128 bits
	token := make([]byte, 16)
	// Fill the slide with cryptographically secure random bytes
	_, err := rand.Read(token)
	if err != nil {
		return "", err
	}
	// Convert the random bytes to a hexadecimal string
	return hex.EncodeToString(token), nil
}

// Handles case where we have OPTIONS request.
func (d *Dbhandler) Options(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Allow", "GET,PUT,POST,PATCH,DELETE,OPTIONS")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET,PUT,POST,PATCH,DELETE,OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "accept,Content-Type,Authorization")
	w.WriteHeader(http.StatusOK)
}

type SessionInfo struct {
	Username  string
	ExpiresAt time.Time
}

func validateToken(d *Dbhandler, w http.ResponseWriter, r *http.Request) bool {
	// check whether token is missing
	token := r.Header.Get("Authorization")
	if token == "" {
		slog.Info("token is missing", "token", token)
		http.Error(w, "Missing or invalid bearer token", http.StatusUnauthorized)
		return false
	}

	// Validate token and expiration in sessions map
	userInfo, ok := d.sessions.Load(token)
	if ok {
		if !userInfo.(SessionInfo).ExpiresAt.After(time.Now()) {
			// token has expired
			slog.Info("token has expired")
			http.Error(w, "Missing or invalid bearer token", http.StatusUnauthorized)
			return false
		} else {
			return true
		}
	} else {
		// token does not exist
		slog.Info("token does not exist")
		http.Error(w, "Missing or invalid bearer token", http.StatusUnauthorized)
		return false
	}
}
