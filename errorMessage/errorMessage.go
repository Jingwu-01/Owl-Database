// Package errorMessage has a helper function that writes error response
// of JSON strings with the given input string and http code.
package errorMessage

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// Writes error response of JSON strings with the given input string and http statusCode.
func ErrorResponse(w http.ResponseWriter, str string, statusCode int) {
	// Converts input string to JSON string
	jsonData, err := json.Marshal(str)
	if err != nil {
		// This should never happen.
		slog.Error("error marshaling error response", "error", err)
		http.Error(w, `"error marshaling error response"`, http.StatusInternalServerError)
		return
	}
	// Write the statusCode and error response
	w.WriteHeader(statusCode)
	w.Write(jsonData)
}
