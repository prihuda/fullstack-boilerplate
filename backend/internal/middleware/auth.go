package middleware

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const UserIDKey contextKey = "userID"
const userEmailKey contextKey = "userEmail"
const tokenExpiresAtKey contextKey = "tokenExpiresAt"

// JWTClaims represents the expected JWT claims structure.
type JWTClaims struct {
	UserID string `json:"sub"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

func GetUserID(r *http.Request) string {
	if id, ok := r.Context().Value(UserIDKey).(string); ok {
		return id
	}
	return ""
}

// WriteError writes a consistent JSON error response.
func WriteError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]any{
		"success": false,
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	}); err != nil {
		slog.ErrorContext(r.Context(), "failed to encode response", "error", err)
	}
}

// AuthMiddleware returns middleware that validates JWT tokens.
func AuthMiddleware(secret string) func(http.Handler) http.Handler {
	secretBytes := []byte(secret)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var tokenStr string

			// Check cookie FIRST — cookie is from our HttpOnly auth and cannot be
			// set by JavaScript (attacker-controlled). Authorization header might be
			// injected by XSS or third-party scripts.
			if cookie, err := r.Cookie("access_token"); err == nil {
				tokenStr = cookie.Value
			}

			// Fallback to Authorization: Bearer header
			if tokenStr == "" {
				authHeader := r.Header.Get("Authorization")
				if strings.HasPrefix(authHeader, "Bearer ") {
					tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
				}
			}

			if tokenStr == "" {
				WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "missing authentication token")
				return
			}

			// Parse with typed claims and explicit signing method validation
			claims := &JWTClaims{}
			token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
				// Explicitly validate signing method — reject none/RS*/ES* algorithms
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return secretBytes, nil
			})
			if err != nil || !token.Valid {
				WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "invalid or expired token")
				return
			}

			if claims.UserID == "" {
				WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "invalid token subject")
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, userEmailKey, claims.Email)
			ctx = context.WithValue(ctx, tokenExpiresAtKey, claims.ExpiresAt.Time)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
