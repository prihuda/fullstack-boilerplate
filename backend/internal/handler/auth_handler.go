package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rhuda/fullstack-boilerplate/backend/internal/config"
	"github.com/rhuda/fullstack-boilerplate/backend/internal/middleware"
	"github.com/rhuda/fullstack-boilerplate/backend/internal/model"
	"github.com/rhuda/fullstack-boilerplate/backend/internal/service"
)

type AuthHandler struct {
	authService  *service.AuthService
	config       *config.Config
	cookieSecure bool
}

func NewAuthHandler(authService *service.AuthService, cfg *config.Config) *AuthHandler {
	return &AuthHandler{
		authService:  authService,
		config:       cfg,
		cookieSecure: cfg.CookieSecure,
	}
}

func (h *AuthHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/login", h.Login)
	r.Post("/refresh", h.Refresh)
	r.Post("/logout", h.Logout)
	r.With(middleware.AuthMiddleware(h.config.JWTSecret)).Get("/me", h.Me)
	return r
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	req, ok := middleware.ValidateRequest[model.LoginRequest](w, r)
	if !ok {
		return
	}

	result, err := h.authService.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid email or password")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "authentication failed")
		return
	}

	// Generate CSRF token
	csrfToken, _ := generateCSRFToken()

	// Set access token cookie — path /api/v1 so it's sent to all API routes
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    result.AccessToken,
		Path:     "/api/v1",
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   15 * 60, // 15 minutes — matches JWT expiry
	})

	// Set refresh token cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    result.RefreshToken,
		Path:     "/api/v1/auth/refresh",
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int((7 * 24 * time.Hour).Seconds()),
	})

	// Set CSRF token in cookie (non-HttpOnly so JS can read it)
	http.SetCookie(w, &http.Cookie{
		Name:     "csrf_token",
		Value:    csrfToken,
		Path:     "/",
		HttpOnly: false,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteStrictMode,
	})

	writeJSON(w, http.StatusOK, model.TokenResponse{
		User: model.UserResponse{
			ID:        result.User.ID,
			Email:     result.User.Email,
			Name:      result.User.Name,
			CreatedAt: result.User.CreatedAt.Format(time.RFC3339),
		},
		CSRFToken: csrfToken,
	})
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "refresh token not found")
		return
	}

	result, err := h.authService.Refresh(r.Context(), cookie.Value)
	if err != nil {
		if errors.Is(err, service.ErrTokenExpired) {
			writeError(w, http.StatusUnauthorized, "TOKEN_EXPIRED", "refresh token has expired")
			return
		}
		if errors.Is(err, service.ErrTokenInvalid) || errors.Is(err, service.ErrUserNotFound) {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid refresh token")
			return
		}
		if errors.Is(err, service.ErrTokenReuse) {
			writeError(w, http.StatusUnauthorized, "TOKEN_REUSE", "refresh token reuse detected")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "token refresh failed")
		return
	}

	// Set new access token cookie — path /api/v1 so it's sent to all API routes
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    result.AccessToken,
		Path:     "/api/v1",
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   15 * 60, // 15 minutes — matches JWT expiry
	})

	// Set new refresh token cookie (rotation)
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    result.RefreshToken,
		Path:     "/api/v1/auth/refresh",
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int((7 * 24 * time.Hour).Seconds()),
	})

	writeJSON(w, http.StatusOK, map[string]string{"message": "tokens refreshed"})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err == nil {
		_ = h.authService.Logout(r.Context(), cookie.Value)
	}

	// Clear access token cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    "",
		Path:     "/api/v1",
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})

	// Clear refresh token cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/api/v1/auth/refresh",
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})

	// Clear CSRF token cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "csrf_token",
		Value:    "",
		Path:     "/",
		HttpOnly: false,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})

	writeJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "user not found in context")
		return
	}

	user, err := h.authService.GetUser(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "user not found")
		return
	}

	writeJSON(w, http.StatusOK, model.UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		CreatedAt: user.CreatedAt.Format(time.RFC3339),
	})
}

// writeJSON writes a success response with {"success":true,"data":...} envelope.
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(model.APIResponse[any]{Success: true, Data: data})
}

// writeError writes an error response with {"success":false,"error":{...}} envelope.
func writeError(w http.ResponseWriter, status int, code, message string) {
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

func generateCSRFToken() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		// Fallback to time-based token
		return time.Now().Format("20060102150405.000000000"), nil
	}
	return hex.EncodeToString(b), nil
}
