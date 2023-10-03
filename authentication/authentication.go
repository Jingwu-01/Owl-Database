package authentication

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

type SessionInfo struct {
	Username  string
	ExpiresAt time.Time
}

// generateToken generates a cryptographically secure and random string token
func generateToken() (string, error) {
	// Generate a 16-byte or 128-bit token
	token := make([]byte, 16)
	// Fill the slide with cryptographically secure random bytes
	_, err := rand.Read(token)
	if err != nil {
		return "", err
	}
	// Convert the random bytes to a hexadecimal string
	return hex.EncodeToString(token), nil
}

// validateToken tells if a token in a request is valid
func validateToken(sessions *sync.Map, w http.ResponseWriter, r *http.Request) bool {
	// Check if the token is missing
	authValue := r.Header.Get("Authorization")
	parts := strings.Split(authValue, " ")

	if len(parts) != 2 || parts[0] != "Bearer" {
		// Missing or malformed bearer token
		slog.Info("validateToken: missing or malformed bearer token", "token", authValue)
		http.Error(w, "Missing or malformed bearer token", http.StatusUnauthorized)
		return false
	}
	token := parts[1]

	if token == "" {
		slog.Info("validateToken: token is missing", "token", token)
		http.Error(w, "Missing or invalid bearer token", http.StatusUnauthorized)
		return false
	}

	// Validate token and expiration in sessions map
	userInfo, ok := sessions.Load(token)
	if ok {
		if !userInfo.(SessionInfo).ExpiresAt.After(time.Now()) {
			// token has expired
			slog.Info("validateToken: token has expired")
			http.Error(w, "Missing or invalid bearer token", http.StatusUnauthorized)
			return false
		} else {
			// token is valid
			return true
		}
	} else {
		// token does not exist
		slog.Info("validateToken: token does not exist")
		http.Error(w, "Missing or invalid bearer token", http.StatusUnauthorized)
		return false
	}
}

// Login request
func Login(sessions *sync.Map, w http.ResponseWriter, r *http.Request) {
	// Set headers of response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Read body of requests
	desc, err := io.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		slog.Error("Login: error reading the request body", "error", err)
		http.Error(w, "invalid login format", http.StatusBadRequest)
		return
	}
	var userInfo map[string]string
	err = json.Unmarshal(desc, &userInfo)
	if err != nil {
		slog.Error("Login: error unmarshaling request", "error", err)
		http.Error(w, `"invalid login format"`, http.StatusBadRequest)
		return
	}

	// I think we should get the username with the JSON visitor model.
	// Store username and token in a session map with expiration time
	username := userInfo["username"]
	if username == "" {
		slog.Error("Login: no username in request body", "error", err)
		http.Error(w, "No username in request body", http.StatusBadRequest)
		return
	}

	// Generate a cryptographically secure, random token
	token, err := generateToken()
	if err != nil {
		slog.Error("Login: token not successfully generated", "error", err)
		http.Error(w, "Login: token not successfully generated", http.StatusInternalServerError)
		return
	}

	sessions.Store(token, SessionInfo{Username: username, ExpiresAt: time.Now().Add(1 * time.Hour)})

	// Return the token to the user
	jsonToken, err := json.Marshal(map[string]string{"token": token})
	// jsonToken, err := json.Marshal(sessions)
	if err != nil {
		// This should never happen
		slog.Error("Login: error marshaling", "error", err)
		http.Error(w, `"internal server error"`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(jsonToken)
	slog.Info("Login: success")
}

// Logout request
func Logout(sessions *sync.Map, w http.ResponseWriter, r *http.Request) {
	// Set headers of response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	isValidToken := validateToken(sessions, w, r)
	if isValidToken {
		// Remove the corresponding userInfo from the sessions map
		authValue := r.Header.Get("Authorization")
		parts := strings.Split(authValue, " ")

		if len(parts) != 2 || parts[0] != "Bearer" {
			// Missing or malformed bearer token
			slog.Info("Logout: missing or malformed bearer token", "token", authValue)
			http.Error(w, "Missing or malformed bearer token", http.StatusUnauthorized)
			return
		}
		token := parts[1]

		sessions.Delete(token)
		w.WriteHeader(http.StatusNoContent)

		slog.Info("Logout: user is successfully removed")
		return
	}
}
