package daemon

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
)

// GenerateAuthToken generates a secure random 32-byte hexadecimal token.
func GenerateAuthToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// RequireAuth wraps an http.HandlerFunc to enforce Bearer token authentication.
func RequireAuth(validToken string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// If no token configured, bypass
		if validToken == "" {
			next(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, `{"error":"unauthorized: missing Authorization header"}`, http.StatusUnauthorized)
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] != validToken {
			http.Error(w, `{"error":"unauthorized: invalid token"}`, http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}
