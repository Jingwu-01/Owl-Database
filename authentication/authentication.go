// Package authentication has structs and methods
// for enabling login and logout functionality per
// the owlDB specifications. Implements the handler
// interface, expects input urls to start with "/auth."
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

	"github.com/RICE-COMP318-FALL23/owldb-p1group20/errorMessage"
	"github.com/RICE-COMP318-FALL23/owldb-p1group20/options"
)

// An authenticator implements methods to allow for users
// to log in and out of the owlDB and to verify users
// are allowed to use this db.
type Authenticator struct {
	sessions *sync.Map
}

// A session info carries an individual users data
// about their session, including username and when
// their session expires.
type sessionInfo struct {
	username  string
	expiresAt time.Time
}

// Creates a new authenticator object
func New() Authenticator {
	return Authenticator{&sync.Map{}}
}

// Installs a map of usernames to login tokens into this
// authenticator. These user's sessions will last 24 hours.
func (a *Authenticator) InstallUsers(users map[string]string) {
	// Iterate over all user/token pairs.
	for user, token := range users {
		a.sessions.Store(token, sessionInfo{user, time.Now().Add(24 * time.Hour)})
	}
}

// Serves http requests with the path starting with /auth. Logs in
// on a post request, logs out on a delete, and sends options if requested.
func (a *Authenticator) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		a.login(w, r)
	case http.MethodDelete:
		a.logout(w, r)
	case http.MethodOptions:
		options.Options(w, r)
	default:
		// If user used method we do not support.
		slog.Info("User used unsupported method", "method", r.Method)
		msg := fmt.Sprintf("unsupported method: %s", r.Method)
		errorMessage.ErrorResponse(w, msg, http.StatusBadRequest)
	}
}

// generateToken generates a cryptographically secure and random string token.
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
// true and the corresponding username if so, else writes an error to the input response writer.
func (a *Authenticator) ValidateToken(w http.ResponseWriter, r *http.Request) (bool, string) {
	w.Header().Set("Content-Type", "application/json")

	// Check if the token is missing
	authValue := r.Header.Get("Authorization")
	parts := strings.Split(authValue, " ")
	slog.Info("Validate request", "full string", authValue)

	if len(parts) != 2 || parts[0] != "Bearer" || parts[1] == "" {
		// Missing or malformed bearer token
		slog.Info("ValidateToken: missing or malformed bearer token", "token", authValue)
		errorMessage.ErrorResponse(w, "Missing or malformed bearer token", http.StatusUnauthorized)
		return false, ""
	}

	token := parts[1]

	// Validate token and expiration in sessions map
	userInfo, ok := a.sessions.Load(token)
	if ok {
		if !userInfo.(sessionInfo).expiresAt.After(time.Now()) {
			// token has expired
			slog.Info("ValidateToken: token has expired")
			errorMessage.ErrorResponse(w, "Expired bearer token", http.StatusUnauthorized)
			return false, ""
		} else {
			// token is valid
			return true, userInfo.(sessionInfo).username
		}
	} else {
		// token does not exist
		slog.Info("ValidateToken: token does not exist")
		errorMessage.ErrorResponse(w, "Invalid bearer token", http.StatusUnauthorized)
		return false, ""
	}
}

// Logs the user who made the input request into the owlDB system.
func (a *Authenticator) login(w http.ResponseWriter, r *http.Request) {
	// Set headers of response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Read body of requests
	desc, err := io.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		slog.Error("Login: error reading the request body", "error", err)
		errorMessage.ErrorResponse(w, "error reading the login request body", http.StatusBadRequest)
		return
	}
	var userInfo map[string]string
	err = json.Unmarshal(desc, &userInfo)
	if err != nil {
		slog.Error("Login: error unmarshaling request", "error", err)
		errorMessage.ErrorResponse(w, "error unmarshaling login request", http.StatusBadRequest)
		return
	}

	// Store username and token in a sessions map with expiration time
	username := userInfo["username"]
	if username == "" {
		slog.Info("Login: no username in request body")
		errorMessage.ErrorResponse(w, "No username in request body", http.StatusBadRequest)
		return
	}

	// Generate a cryptographically secure, random token
	token, err := generateToken()
	if err != nil {
		// This should never happen
		slog.Error("Login: token not successfully generated", "error", err)
		errorMessage.ErrorResponse(w, "Login: token not successfully generated", http.StatusInternalServerError)
		return
	}

	// Add the user to the sessions map and its authorization expires one hour later
	a.sessions.Store(token, sessionInfo{username, time.Now().Add(1 * time.Hour)})

	// Return the token to the user
	jsonToken, err := json.Marshal(map[string]string{"token": token})
	if err != nil {
		// This should never happen
		slog.Error("Login: error marshaling", "error", err)
		errorMessage.ErrorResponse(w, "error marshaling token", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(jsonToken)
	slog.Info("Login: success", "user", username)
}

// Logs the user who made this request out of the owlDB system.
func (a *Authenticator) logout(w http.ResponseWriter, r *http.Request) {
	// Set headers of response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	isValidToken, _ := a.ValidateToken(w, r)
	if isValidToken {
		// Remove the corresponding userInfo from the sessions map
		authValue := r.Header.Get("Authorization")
		parts := strings.Split(authValue, " ")
		token := parts[1]
		a.sessions.Delete(token)

		w.WriteHeader(http.StatusNoContent)
		slog.Info("Logout: user is successfully removed", "token", token)
	}
}
