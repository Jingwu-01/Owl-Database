// Package structs contains struct definitions for structs
// not associated with a file (not created by New).
package structs

import "github.com/RICE-COMP318-FALL23/owldb-p1group20/subscribe"

// A PatchResponse stores the response from a Patch operation
type PatchResponse struct {
	Uri         string `json:"uri"`
	PatchFailed bool   `json:"patchFailed"`
	Message     string `json:"message"`
}

// A PutOutput stores the response to a put request.
type PutOutput struct {
	Uri string `json:"uri"`
}

// A CollSub is a wrapper for a subscriber to a collection.
type CollSub struct {
	Subscriber    subscribe.Subscriber
	IntervalStart string
	IntervalEnd   string
}
