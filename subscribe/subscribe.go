// Package subscribe has structs and methods for
// supporting subscription to documents and
// collections as per the owlDB specification.
package subscribe

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/RICE-COMP318-FALL23/owldb-p1group20/errorMessage"
)

// A write flusher is an interface to allow for
// casting response writers to SSE-supporting
// flushers.
type writeFlusher interface {
	http.ResponseWriter
	http.Flusher
}

// A subscriber has channels for supporting sending
// messages concurrently from documents and collections
// to a subscriber.
type Subscriber struct {
	UpdateCh chan []byte // A channel to which we write updates.
	DeleteCh chan string // A channel to which we write deletions.
}

// New creates a new subscriber.
func New() Subscriber {
	return Subscriber{
		UpdateCh: make(chan []byte),
		DeleteCh: make(chan string),
	}
}

// Writes delete event when a document or collection is deleted.
func writeDelete(path string) string {
	// Create event
	var event bytes.Buffer
	now := time.Now()
	millisecondsSinceEpoch := now.UnixMilli()
	event.WriteString(fmt.Sprintf("event: delete\ndata: \"%s\"\nid: %d\n\n", path, millisecondsSinceEpoch))
	slog.Info("Sending", "msg", event.String())

	return event.String()
}

// Writes update event when a document or collection is updated.
func writeUpdate(jsonObj []byte) string {
	// Create event
	var event bytes.Buffer
	now := time.Now()
	millisecondsSinceEpoch := now.UnixMilli()
	event.WriteString(fmt.Sprintf("event: update\ndata: %s\nid: %d\n\n", jsonObj, millisecondsSinceEpoch))
	slog.Info("Sending", "msg", event.String())

	return event.String()
}

// Writes comment event to keep the server running.
func writeComment() string {
	// Create event
	var event bytes.Buffer
	event.WriteString(": This is a comment event that keeps the server running\n\n")
	slog.Info("Sending", "msg", event.String())

	return event.String()
}

// ServeSubscriber runs in a goroutine sending
// events to a subscriber regarding the elements
// of a database that they have subscribed to.
func (s Subscriber) ServeSubscriber(w http.ResponseWriter, r *http.Request) {
	// Convert ResponseWriter to a writeFlusher
	wf, ok := w.(writeFlusher)
	if !ok {
		slog.Info("Couldn't convert to write flusher")
		errorMessage.ErrorResponse(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	slog.Info("Converted to writeFlusher")

	// Set up event stream connection
	wf.Header().Set("Content-Type", "text/event-stream")
	wf.Header().Set("Cache-Control", "no-cache")
	wf.Header().Set("Connection", "keep-alive")
	wf.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Last-Event-ID")
	wf.Header().Set("Access-Control-Allow-Origin", "*")
	wf.WriteHeader(http.StatusOK)
	wf.Flush()

	slog.Info("Sent headers")

	for {
		select {
		case <-r.Context().Done():
			// Client closed connection
			slog.Info("Subscribe: Client closed connection")
			return
		case <-time.After(15 * time.Second):
			// Send comments every 15 seconds to keep the connection
			wf.Write([]byte(writeComment()))
			wf.Flush()
		case jsonObj := <-s.UpdateCh:
			wf.Write([]byte(writeUpdate(jsonObj)))
			wf.Flush()
		case path := <-s.DeleteCh:
			wf.Write([]byte(writeDelete(path)))
			wf.Flush()
		}
	}
}
