package handler

import (
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
	authService         *service.AuthService
	config              *config.Config
	cookieSecure        bool
	refreshCookieMaxAge int
}

func NewAuthHandler(authService *service.AuthService, cfg *config.Config) *AuthHandler {
	return &AuthHandler{
		authService:         authService,
		config:              cfg,
		cookieSecure:        cfg.CookieSecure,
		refreshCookieMaxAge: int((7 * 24 * time.Hour).Seconds()),
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

// extractRefreshToken reads the refresh token from cookie first (web browsers),
// then falls back to JSON body (API clients: mobile, CLI, 3rd party).
func extractRefreshToken(r *http.Request) (string, error) {
	// 1. Try cookie first (web browsers — sent automatically)
	if cookie, err := r.Cookie("refresh_token"); err == nil && cookie.Value != "" {
		return cookie.Value, nil
	}

	// 2. Try JSON body (API clients without cookies)
	var req model.RefreshTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err == nil && req.RefreshToken != "" {
		return req.RefreshToken, nil
	}

	return "", errors.New("refresh token required")
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	req := middleware.ValidateRequest[model.LoginRequest](w, r)
	if req == nil {
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

	expiresIn := int(time.Until(result.ExpiresAt).Seconds())
	if expiresIn < 0 {
		expiresIn = 0
	}

	// Always set HttpOnly cookies (web browsers use these automatically)
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    result.AccessToken,
		Path:     "/api/v1",
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   expiresIn,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    result.RefreshToken,
		Path:     "/api/v1/auth/refresh",
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   h.refreshCookieMaxAge,
	})

	// Always return tokens in body (API clients: mobile, 3rd party, CLI)
	writeJSON(w, http.StatusOK, model.TokenResponse{
		AccessToken:  result.AccessToken,
		TokenType:    h.config.TokenType,
		RefreshToken: result.RefreshToken,
		ExpiresIn:    expiresIn,
		ExpiresAt:    result.ExpiresAt.Format(time.RFC3339),
	})
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	rawRefreshToken, err := extractRefreshToken(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "MISSING_TOKEN", "refresh token required")
		return
	}

	result, err := h.authService.Refresh(r.Context(), rawRefreshToken)
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
			h.clearAuthCookies(w)
			writeError(w, http.StatusUnauthorized, "TOKEN_REUSE", "refresh token reuse detected")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "token refresh failed")
		return
	}

	expiresIn := int(time.Until(result.ExpiresAt).Seconds())
	if expiresIn < 0 {
		expiresIn = 0
	}

	// Set new cookies (web browsers)
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    result.AccessToken,
		Path:     "/api/v1",
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   expiresIn,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    result.RefreshToken,
		Path:     "/api/v1/auth/refresh",
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   h.refreshCookieMaxAge,
	})

	// Always return new tokens in body (API clients)
	writeJSON(w, http.StatusOK, model.TokenResponse{
		AccessToken:  result.AccessToken,
		TokenType:    h.config.TokenType,
		RefreshToken: result.RefreshToken,
		ExpiresIn:    expiresIn,
		ExpiresAt:    result.ExpiresAt.Format(time.RFC3339),
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	rawRefreshToken, err := extractRefreshToken(r)
	if err == nil {
		_ = h.authService.Logout(r.Context(), rawRefreshToken)
	}

	h.clearAuthCookies(w)
	writeJSON(w, http.StatusOK, "logged out")
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

// clearAuthCookies sets expired cookies to effectively delete them.
func (h *AuthHandler) clearAuthCookies(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    "",
		Path:     "/api/v1",
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/api/v1/auth/refresh",
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}
