// This file is a skeleton for your project. You should replace this
// comment with true documentation.
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

func main() {
	var schema *jsonschema.Schema
	var server http.Server
	var port int
	var err error

	// Initialize flags
	portFlag := flag.Int("p", 3318, "Port number")
	schemaFlag := flag.String("s", "", "Schema file name")
	tokenFlag := flag.String("t", "", "Token file name")

	flag.Parse()

	port = *portFlag

	if *schemaFlag == "" {
		slog.Error("Missing schema", "error")
		return
	}

	schema, err = jsonschema.Compile(*schemaFlag)

	if err != nil {
		slog.Error("Invalid schema", "error", err)
	}

	fmt.Println(tokenFlag)
	fmt.Println(schema.Location)

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
