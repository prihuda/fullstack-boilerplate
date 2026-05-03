package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const UserIDKey contextKey = "userID"
const UserEmailKey contextKey = "userEmail"
const TokenExpiresAtKey contextKey = "tokenExpiresAt"

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

func GetUserEmail(r *http.Request) string {
	if email, ok := r.Context().Value(UserEmailKey).(string); ok {
		return email
	}
	return ""
}

// WriteError writes a consistent JSON error response.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{
		"success": false,
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

// writeAuthError writes a consistent JSON error response from auth middleware.
func writeAuthError(w http.ResponseWriter, status int, code, message string) {
	WriteError(w, status, code, message)
}

func AuthMiddleware(secret string) func(http.Handler) http.Handler {
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
				writeAuthError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing authentication token")
				return
			}

			// Parse with typed claims and explicit signing method validation
			claims := &JWTClaims{}
			token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
				// Explicitly validate signing method — reject none/RS*/ES* algorithms
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(secret), nil
			})
			if err != nil || !token.Valid {
				writeAuthError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid or expired token")
				return
			}

			if claims.UserID == "" {
				writeAuthError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid token subject")
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, UserEmailKey, claims.Email)
			ctx = context.WithValue(ctx, TokenExpiresAtKey, claims.ExpiresAt.Time)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AuthOptional extracts user info if token is present but doesn't block unauthenticated requests.
func AuthOptional(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var tokenStr string

			// Cookie first, then header fallback
			if cookie, err := r.Cookie("access_token"); err == nil {
				tokenStr = cookie.Value
			}
			if tokenStr == "" {
				authHeader := r.Header.Get("Authorization")
				if strings.HasPrefix(authHeader, "Bearer ") {
					tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
				}
			}

			if tokenStr == "" {
				next.ServeHTTP(w, r)
				return
			}

			claims := &JWTClaims{}
			token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(secret), nil
			})
			if err != nil || !token.Valid {
				next.ServeHTTP(w, r)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, UserEmailKey, claims.Email)
			ctx = context.WithValue(ctx, TokenExpiresAtKey, claims.ExpiresAt.Time)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
