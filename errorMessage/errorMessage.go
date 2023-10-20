// Package errorMessage has a helper function that writes error response
// of JSON strings with the given input string and http code.
package errorMessage

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// Writes error response of JSON strings with the given input string and http code.
func ErrorResponse(w http.ResponseWriter, str string, statusCode int) {
	jsonData, err := json.Marshal(str)
	if err != nil {
		// This should never happen.
		slog.Error("error marshaling error response", "error", err)
		http.Error(w, `"error marshaling error response"`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(statusCode)
	w.Write(jsonData)
}
