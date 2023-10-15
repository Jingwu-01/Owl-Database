package structs

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
