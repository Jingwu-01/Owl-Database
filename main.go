/*
OwlDB stores documents in a hirearchical database structure.
The documents are JSON values encoded in UTF-8. The database
imposes a schema against which all documents are verified.

The DB stores a set of zero or more top-level databases, which
each point to zero or more documents. The documents contain
server-created metadata as well as user-supplied content. They
also point to collections, which are not part of the output to
the client, but contain further documents.

Usage:

	owldb [flags]

The flags are:

	-p
		A port number, an integer greater than 1024. If omitted,
		the server will listen to port 3318.
	-s
		A JSON schema file name, the file which contains the JSON
		schema that will be used to validate all the documents stored
		in the database. This flag must be included and the file must
		be a valid JSON schema file.
	-t
		A token file name, the file contains a JSON mapping user names
		to string tokens. These tokens will be installed on the system
		for 24 hours.

When a client connects to the OwlDB server, they will be given a unique
token which they will use on all future logins. They will have the power
to then access all of the databases, documents, and collections on the
server, as well as adding new ones, and subscribing to changes.
*/
package main

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

// A meta stores metadata about a document.
type meta struct {
	createdBy      string
	createdAt      int
	lastModifiedBy string
	lastModifiedAt int
}

/*
A collection is a concurrent skip list of documents,
which is sorted by document name.
*/
type collection struct {
	documents sync.Map
}

/*
A docoutput is a struct which represents the data to
be output when a user requests a given document.
*/
type docoutput struct {
	path     string
	meta     meta
	contents []byte
}

// A document is a document plus a concurrent skip list of collections
type document struct {
	output   docoutput
	children sync.Map
}

// A dbhandler is the highest level struct, holds all the collections and
// handles all the http requests.
type dbhandler struct {
	databases sync.Map
}

// Creates a new DBHandler
func NewDBHandler() dbhandler {
	return dbhandler{sync.Map{}}
}

// The server implements the "handler" interface, it will recieve
// requests from the user and delegate them to the proper methods.
func (d *dbhandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		d.Get(w, r)
	case http.MethodPut:
		d.Put(w, r)
	//case http.MethodPost:
	// Post handling
	//case http.MethodPatch:
	// Patch handling
	//case http.MethodDelete:
	// Delete handling
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
func (d *dbhandler) Get(w http.ResponseWriter, r *http.Request) {
	path, found := strings.CutPrefix(r.URL.Path, "/v1/")

	// Check for version
	if !found {
		slog.Info("User path did not include version", "path", r.URL.Path)
		msg := fmt.Sprintf("path missing version: %s", r.URL.Path)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	splitpath := strings.SplitAfterN(path, "/", 2)

	if len(splitpath) == 1 {
		// Error, DB path does not end with "/"
		slog.Info("DB path did not end with \"/\"", "path", r.URL.Path)
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
			msg := fmt.Sprintf("Invalid database: %s", dbpath)
			http.Error(w, msg, http.StatusBadRequest)
			return
		}

		// Just to remove database not used error
		fmt.Println(database)
	} else {
		// GET Document or Collection
		dbpath, _ := strings.CutSuffix(splitpath[0], "/")
		path = splitpath[1]

		// Access the database
		database, ok := d.databases.Load(dbpath)

		// Check to see if database exists
		if !ok {
			slog.Info("User attempted to access non-extant database", "db", dbpath)
			msg := fmt.Sprintf("Invalid database: %s", dbpath)
			http.Error(w, msg, http.StatusBadRequest)
			return
		}

		// Just to remove database not used error
		fmt.Println(database)
	}
}

// Handles case where we have PUT request.
func (d *dbhandler) Put(w http.ResponseWriter, r *http.Request) {
	path, found := strings.CutPrefix(r.URL.Path, "/v1/")

	// Check for version
	if !found {
		slog.Info("User path did not include version", "path", path)
		msg := fmt.Sprintf("path missing version: %s", path)
		http.Error(w, msg, http.StatusBadRequest)
	}

	splitpath := strings.SplitAfterN(path, "/", 2)

	if len(splitpath) == 1 {
		// PUT database case
		dbpath := splitpath[0]
		fmt.Println(dbpath)
	} else {
		// PUT document or collection
		dbpath, _ := strings.CutSuffix(splitpath[0], "/")
		path = splitpath[1]

		// Access the database
		database, ok := d.databases.Load(dbpath)

		// Check to see if database exists
		if !ok {
			slog.Info("User attempted to access non-extant database", "db", dbpath)
			msg := fmt.Sprintf("Invalid database: %s", dbpath)
			http.Error(w, msg, http.StatusBadRequest)
			return
		}

		// Just to remove database not used error
		fmt.Println(database)
	}
}

// Handles case where we have OPTIONS request.
func (d *dbhandler) Options(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Allow", "GET,PUT,POST,PATCH,DELETE,OPTIONS")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET,PUT,POST,PATCH,DELETE,OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "accept,Content-Type,Authorization")
	w.WriteHeader(http.StatusOK)
}

// Implements the main functionality for the OwlDB program.
func main() {
	// Declare variables for main method.
	var port int
	var schema *jsonschema.Schema
	var tknPath string
	var testMode bool
	var err error
	var server http.Server
	var handler dbhandler

	// Initialize the user input variables.
	port, schema, tknPath, testMode, err = initialize()

	// Printing was handled in initialize.
	if err != nil {
		return
	}

	handler = NewDBHandler()
	server.Addr = fmt.Sprintf("localhost:%d", port)
	server.Handler = &handler

	if testMode {
		handler.databases.Store("db1", collection{})
		handler.databases.Store("db2", collection{})
	}

	// Remove errors of not used
	slog.Info("Input token path", "tokenpath", tknPath)
	slog.Info("Schema input", "schema", schema)

	// The following code should go last and remain unchanged.
	// Note that you must actually initialize 'server' and 'port'
	// before this.

	// signal.Notify requires the channel to be buffered
	ctrlc := make(chan os.Signal, 1)
	signal.Notify(ctrlc, os.Interrupt, syscall.SIGTERM)
	go func() {
		// Wait for Ctrl-C signal
		<-ctrlc
		server.Close()
	}()

	// Start server
	slog.Info("Listening", "port", port)
	err = server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		slog.Error("Server closed", "error", err)
	} else {
		slog.Info("Server closed", "error", err)
	}
}

// Initialize sets up flags for inputs and compiles
// the input schema file into a jsonschema.Schema object.
// Returns the port number, schema object, and the token
// file's path.
func initialize() (int, *jsonschema.Schema, string, bool, error) {
	// Initialize flags
	portFlag := flag.Int("p", 3318, "Port number")
	schemaFlag := flag.String("s", "", "Schema file name")
	tokenFlag := flag.String("t", "", "Token file name")
	loggerFlag := flag.Int("l", 0, "Logger output level, -1 for debug, 1 for only errors")
	testFlag := flag.Bool("i", false, "true to initialize a default database for testing")
	flag.Parse()

	// Ensure we got a schema file
	if *schemaFlag == "" {
		slog.Error("Missing schema", "error", errors.New("Missing schema"))
		return 0, nil, "", false, errors.New("Missing Schema")
	}

	// // Compile the schema
	// schema, err := jsonschema.Compile(*schemaFlag)

	// // Check for errors.
	// if err != nil {
	// 	slog.Error("Invalid schema", "error", err)
	// 	return 0, nil, "", false, errors.New("Invalid schema")
	// }

	// Set to debug and above
	if *loggerFlag == -1 {
		h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})
		slog.SetDefault(slog.New(h))
	}

	// Set to error only
	if *loggerFlag == 1 {
		h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})
		slog.SetDefault(slog.New(h))
	}

	return *portFlag, nil, *tokenFlag, *testFlag, nil
}
