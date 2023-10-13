package subscribe

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

type writeFlusher interface {
	http.ResponseWriter
	http.Flusher
}

type subscriber struct {
	updateCh chan []byte
	deleteCh chan string
}

func New() subscriber {
	return subscriber{
		updateCh: make(chan []byte, 1000),
		deleteCh: make(chan string, 1000),
	}
}

// Send delete event when a document or collection is deleted
func (s subscriber) sendDelete(wf writeFlusher, path string) {
	// Create event
	var event bytes.Buffer
	now := time.Now()
	millisecondsSinceEpoch := now.UnixNano() / 1e6
	event.WriteString(fmt.Sprintf("event: delete\ndata: \"%s\"\nid: %d\n\n", path, millisecondsSinceEpoch))
	slog.Info("Sending", "msg", event.String())

	// Send event
	wf.Write(event.Bytes())
	wf.Flush()
}

// Send update event when a document or collection is updated
func (s subscriber) sendUpdate(wf writeFlusher, jsonObj []byte) {
	// Create event
	var event bytes.Buffer
	now := time.Now()
	millisecondsSinceEpoch := now.UnixNano() / 1e6
	event.WriteString(fmt.Sprintf("event: update\ndata: %s\nid: %d\n\n", jsonObj, millisecondsSinceEpoch))
	slog.Info("Sending", "msg", event.String())

	// Send event
	wf.Write(event.Bytes())
	wf.Flush()
}

// Send comment event to keep the server running
func (s subscriber) sendComment(wf writeFlusher) {
	// Create event
	var event bytes.Buffer
	event.WriteString(fmt.Sprintf(": This is a comment event that keeps the server running"))
	slog.Info("Sending", "msg", event.String())

	// Send event
	wf.Write(event.Bytes())
	wf.Flush()
}

func (s subscriber) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Convert ResponseWriter to a writeFlusher
	wf, ok := w.(writeFlusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
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
			s.sendComment(wf)
		case jsonObj := <-s.updateCh:
			s.sendUpdate(wf, jsonObj)
		case path := <-s.deleteCh:
			s.sendDelete(wf, path)
		}
	}
}
