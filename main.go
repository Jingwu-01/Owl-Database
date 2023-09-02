// This file is a skeleton for your project. You should replace this
// comment with true documentation.

package main

import (
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	var server http.Server
	var port int
	var err error

	// Your code goes here.

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
