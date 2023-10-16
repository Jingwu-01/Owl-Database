// Package subscribe has structs and methods for
// supporting subscription to documents and collections
// as per the owlDB specification.
package subscribe

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"time"
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
	UpdateCh chan []byte
	DeleteCh chan string
}

// New creates a new subscriber.
func New() Subscriber {
	return Subscriber{
		UpdateCh: make(chan []byte),
		DeleteCh: make(chan string),
	}
}

// Send delete event when a document or collection is deleted.
func writeDelete(path string) string {
	// Create event
	var event bytes.Buffer
	now := time.Now()
	millisecondsSinceEpoch := now.UnixMilli()
	event.WriteString(fmt.Sprintf("event: delete\ndata: \"%s\"\nid: %d\n\n", path, millisecondsSinceEpoch))
	slog.Info("Sending", "msg", event.String())

	return event.String()
}

// Send update event when a document or collection is updated.
func writeUpdate(jsonObj []byte) string {
	// Create event
	var event bytes.Buffer
	now := time.Now()
	millisecondsSinceEpoch := now.UnixMilli()
	event.WriteString(fmt.Sprintf("event: update\ndata: %s\nid: %d\n\n", jsonObj, millisecondsSinceEpoch))
	slog.Info("Sending", "msg", event.String())

	return event.String()
}

// Send comment event to keep the server running.
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
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		slog.Info("Couldn't convert to write flusher")
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

	wf.Write([]byte(writeDelete("hello")))
	wf.Flush()

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
