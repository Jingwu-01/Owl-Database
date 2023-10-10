package authentication

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/RICE-COMP318-FALL23/owldb-p1group20/options"
)

// An authenticator implements methods to allow for users
// to log in and out of the owlDB and to verify users
// are allowed to use this db.
type Authenticator struct {
	sessions *sync.Map
}

// Creates a new authenticator object
func New() Authenticator {
	return Authenticator{&sync.Map{}}
}

func (a *Authenticator) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		a.Login(w, r)
	case http.MethodDelete:
		a.Logout(w, r)
	case http.MethodOptions:
		options.Options(w, r)
	default:
		// If user used method we do not support.
		slog.Info("User used unsupported method", "method", r.Method)
		msg := fmt.Sprintf("unsupported method: %s", r.Method)
		http.Error(w, msg, http.StatusBadRequest)
	}
}

type sessionInfo struct {
	username  string
	expiresAt time.Time
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

// ValidateToken tells if a token in a request is valid. Returns
// true if so, else writes an error to the input response writer.
func (a *Authenticator) ValidateToken(w http.ResponseWriter, r *http.Request) bool {
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
	userInfo, ok := a.sessions.Load(token)
	if ok {
		if !userInfo.(sessionInfo).expiresAt.After(time.Now()) {
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

// Handles login request.
func (a *Authenticator) Login(w http.ResponseWriter, r *http.Request) {
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

	a.sessions.Store(token, sessionInfo{username, time.Now().Add(1 * time.Hour)})

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
	slog.Info("Login: success", "user", username)
}

// Handles logout request.
func (a *Authenticator) Logout(w http.ResponseWriter, r *http.Request) {
	// Set headers of response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	isValidToken := a.ValidateToken(w, r)
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

		a.sessions.Delete(token)
		w.WriteHeader(http.StatusNoContent)

		slog.Info("Logout: user is successfully removed", "token", token)
		return
	}
}
