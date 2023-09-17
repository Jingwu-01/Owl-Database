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
	-l
		An integer, logger output level, 1 for errors only, -1 for debug
		as well as all other info.
	-i
		A bool flag - include to have a default set of databases added to
		the server.

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
	"syscall"

	"github.com/RICE-COMP318-FALL23/owldb-p1group20/dbhandler"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

// Implements the main functionality for the OwlDB program.
func main() {
	// Declare variables for main method.
	var port int
	var schema *jsonschema.Schema
	var tknPath string
	var testMode bool
	var err error
	var server http.Server
	var handler dbhandler.Dbhandler

	// Initialize the user input variables.
	port, schema, tknPath, testMode, err = initialize()

	// Printing was handled in initialize.
	if err != nil {
		return
	}

	handler = dbhandler.New(testMode)
	server.Addr = fmt.Sprintf("localhost:%d", port)
	server.Handler = &handler

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
