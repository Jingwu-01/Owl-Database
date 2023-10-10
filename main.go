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
		A bool flag - include to have a test set of databases and documents
		added to the server.

When a client connects to the OwlDB server, they will be given a unique
token which they will use on all future logins. They will have the power
to then access all of the databases, documents, and collections on the
server, as well as adding new ones, and subscribing to changes.
*/
package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/RICE-COMP318-FALL23/owldb-p1group20/authentication"
	"github.com/RICE-COMP318-FALL23/owldb-p1group20/dbhandler"
	"github.com/RICE-COMP318-FALL23/owldb-p1group20/initialize"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

// Implements the main functionality for the OwlDB program.
func main() {
	// Declare variables for main method.
	var port int
	var schema *jsonschema.Schema
	var tokenmap map[string]string
	var testMode bool
	var err error
	var server http.Server
	var owlDB dbhandler.Dbhandler
	var authenticator authentication.Authenticator

	// Initialize the user input variables.
	port, schema, tokenmap, testMode, err = initialize.Initialize()

	// Printing was handled in initialize.
	if err != nil {
		return
	}

	authenticator = authentication.New()
	owlDB = dbhandler.New(testMode, schema, &authenticator)

	mux := http.NewServeMux()
	mux.Handle("/v1/", &owlDB)
	mux.Handle("/auth", &authenticator)

	server.Addr = fmt.Sprintf("localhost:%d", port)
	server.Handler = mux

	// Remove errors of not used
	slog.Info("Input token path", "tokenpath", tokenmap)

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
