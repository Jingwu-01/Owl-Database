// Package structs contains struct definitions for structs
// not associated with a file (not created by New).
package structs

import "github.com/RICE-COMP318-FALL23/owldb-p1group20/subscribe"

// A PatchResponse stores the response from a Patch operation
type PatchResponse struct {
	Uri         string `json:"uri"`         // The URI at which this patch was applied.
	PatchFailed bool   `json:"patchFailed"` // A boolean indicating whether this patch failed.
	Message     string `json:"message"`     // A message indicating why a patch failed or "patches applied."
}

// A PutOutput stores the response to a put request.
type PutOutput struct {
	Uri string `json:"uri"` // The URI of the successful put operation.
}

// A CollSub is a wrapper for a subscriber to a collection.
type CollSub struct {
	Subscriber    subscribe.Subscriber // The subscriber object.
	IntervalStart string               // The start of the interval for this subscribers query.
	IntervalEnd   string               // The end of the interval for this subscribers query.
}
