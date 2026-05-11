package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rhuda/fullstack-boilerplate/backend/internal/config"
	"github.com/rhuda/fullstack-boilerplate/backend/internal/middleware"
	"github.com/rhuda/fullstack-boilerplate/backend/internal/model"
	"github.com/rhuda/fullstack-boilerplate/backend/internal/service"
)

// ---------------------------------------------------------------------------
// Mock service
// ---------------------------------------------------------------------------

type mockAuthService struct {
	loginFn   func(ctx context.Context, email, password string) (*service.LoginResult, error)
	refreshFn func(ctx context.Context, rawToken string) (*service.RefreshResult, error)
	logoutFn  func(ctx context.Context, rawToken string) error
	getUserFn func(ctx context.Context, userID string) (*model.User, error)
}

func (m *mockAuthService) Login(ctx context.Context, email, password string) (*service.LoginResult, error) {
	return m.loginFn(ctx, email, password)
}

func (m *mockAuthService) Refresh(ctx context.Context, rawToken string) (*service.RefreshResult, error) {
	return m.refreshFn(ctx, rawToken)
}

func (m *mockAuthService) Logout(ctx context.Context, rawToken string) error {
	return m.logoutFn(ctx, rawToken)
}

func (m *mockAuthService) GetUser(ctx context.Context, userID string) (*model.User, error) {
	return m.getUserFn(ctx, userID)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const testJWTSecret = "test-jwt-secret-for-handler-tests"

func testConfig() *config.Config {
	return &config.Config{
		JWTSecret:   testJWTSecret,
		TokenType:   "Bearer",
		CookieSecure: false,
	}
}

func newTestHandler(svc service.AuthServicer) *AuthHandler {
	return NewAuthHandler(svc, testConfig())
}

func generateTestJWT(t *testing.T, userID, email string, expiresAt time.Time) string {
	t.Helper()
	claims := middleware.JWTClaims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := token.SignedString([]byte(testJWTSecret))
	require.NoError(t, err)
	return s
}

// decodeResponse decodes the JSON body into a map.
func decodeResponse(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var resp map[string]any
	err := json.Unmarshal(body, &resp)
	require.NoError(t, err)
	return resp
}

// ---------------------------------------------------------------------------
// Login handler tests
// ---------------------------------------------------------------------------

func TestLoginHandler(t *testing.T) {
	t.Parallel()

	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		now := time.Now().Add(15 * time.Minute)
		svc := &mockAuthService{
			loginFn: func(_ context.Context, _, _ string) (*service.LoginResult, error) {
				return &service.LoginResult{
					AccessToken:  "access-token-123",
					RefreshToken: "refresh-token-456",
					User: &model.User{
						ID:    "user-1",
						Email: "alice@example.com",
						Name:  "Alice",
					},
					ExpiresAt: now,
				}, nil
			},
		}
		h := newTestHandler(svc)
		router := h.Routes()

		body := `{"email":"alice@example.com","password":"password123"}`
		req := httptest.NewRequest("POST", "/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		resp := decodeResponse(t, rec.Body.Bytes())
		assert.Equal(t, true, resp["success"])

		data := resp["data"].(map[string]any)
		assert.Equal(t, "access-token-123", data["access_token"])
		assert.Equal(t, "Bearer", data["token_type"])
		assert.Equal(t, "refresh-token-456", data["refresh_token"])

		// Verify cookies are set.
		cookies := rec.Result().Cookies()
		cookieNames := make(map[string]bool)
		for _, c := range cookies {
			cookieNames[c.Name] = true
		}
		assert.True(t, cookieNames["access_token"], "access_token cookie should be set")
		assert.True(t, cookieNames["refresh_token"], "refresh_token cookie should be set")
	})

	t.Run("InvalidBody", func(t *testing.T) {
		t.Parallel()
		svc := &mockAuthService{}
		h := newTestHandler(svc)
		router := h.Routes()

		req := httptest.NewRequest("POST", "/login", strings.NewReader("not-json{{{"))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)

		resp := decodeResponse(t, rec.Body.Bytes())
		assert.Equal(t, false, resp["success"])
	})

	t.Run("WrongCredentials", func(t *testing.T) {
		t.Parallel()
		svc := &mockAuthService{
			loginFn: func(_ context.Context, _, _ string) (*service.LoginResult, error) {
				return nil, service.ErrInvalidCredentials
			},
		}
		h := newTestHandler(svc)
		router := h.Routes()

		body := `{"email":"alice@example.com","password":"wrong"}`
		req := httptest.NewRequest("POST", "/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)

		resp := decodeResponse(t, rec.Body.Bytes())
		assert.Equal(t, false, resp["success"])
		errObj := resp["error"].(map[string]any)
		assert.Equal(t, "UNAUTHORIZED", errObj["code"])
	})
}

// ---------------------------------------------------------------------------
// Refresh handler tests
// ---------------------------------------------------------------------------

func TestRefreshHandler(t *testing.T) {
	t.Parallel()

	t.Run("Success_ViaCookie", func(t *testing.T) {
		t.Parallel()
		now := time.Now().Add(15 * time.Minute)
		svc := &mockAuthService{
			refreshFn: func(_ context.Context, _ string) (*service.RefreshResult, error) {
				return &service.RefreshResult{
					AccessToken:  "new-access-token",
					RefreshToken: "new-refresh-token",
					ExpiresAt:    now,
				}, nil
			},
		}
		h := newTestHandler(svc)
		router := h.Routes()

		req := httptest.NewRequest("POST", "/refresh", nil)
		req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "valid-refresh-token"})
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		resp := decodeResponse(t, rec.Body.Bytes())
		assert.Equal(t, true, resp["success"])
		data := resp["data"].(map[string]any)
		assert.Equal(t, "new-access-token", data["access_token"])
		assert.Equal(t, "new-refresh-token", data["refresh_token"])
	})

	t.Run("Success_ViaBody", func(t *testing.T) {
		t.Parallel()
		now := time.Now().Add(15 * time.Minute)
		svc := &mockAuthService{
			refreshFn: func(_ context.Context, rawToken string) (*service.RefreshResult, error) {
				assert.Equal(t, "body-refresh-token", rawToken)
				return &service.RefreshResult{
					AccessToken:  "new-access-token",
					RefreshToken: "new-refresh-token",
					ExpiresAt:    now,
				}, nil
			},
		}
		h := newTestHandler(svc)
		router := h.Routes()

		body := `{"refresh_token":"body-refresh-token"}`
		req := httptest.NewRequest("POST", "/refresh", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("NoToken", func(t *testing.T) {
		t.Parallel()
		svc := &mockAuthService{
			refreshFn: func(_ context.Context, _ string) (*service.RefreshResult, error) {
				t.Fatal("refresh should not be called")
				return nil, nil
			},
		}
		h := newTestHandler(svc)
		router := h.Routes()

		req := httptest.NewRequest("POST", "/refresh", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)

		resp := decodeResponse(t, rec.Body.Bytes())
		assert.Equal(t, false, resp["success"])
	})

	t.Run("TokenExpired", func(t *testing.T) {
		t.Parallel()
		svc := &mockAuthService{
			refreshFn: func(_ context.Context, _ string) (*service.RefreshResult, error) {
				return nil, service.ErrTokenExpired
			},
		}
		h := newTestHandler(svc)
		router := h.Routes()

		req := httptest.NewRequest("POST", "/refresh", nil)
		req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "expired-token"})
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)

		resp := decodeResponse(t, rec.Body.Bytes())
		errObj := resp["error"].(map[string]any)
		assert.Equal(t, "TOKEN_EXPIRED", errObj["code"])
	})

	t.Run("TokenReuse", func(t *testing.T) {
		t.Parallel()
		svc := &mockAuthService{
			refreshFn: func(_ context.Context, _ string) (*service.RefreshResult, error) {
				return nil, service.ErrTokenReuse
			},
		}
		h := newTestHandler(svc)
		router := h.Routes()

		req := httptest.NewRequest("POST", "/refresh", nil)
		req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "reused-token"})
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)

		resp := decodeResponse(t, rec.Body.Bytes())
		errObj := resp["error"].(map[string]any)
		assert.Equal(t, "TOKEN_REUSE", errObj["code"])

		// Verify cookies are cleared on reuse.
		cookies := rec.Result().Cookies()
		for _, c := range cookies {
			if c.Name == "access_token" || c.Name == "refresh_token" {
				assert.Equal(t, -1, c.MaxAge, "%s cookie should be cleared", c.Name)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// Me handler tests
// ---------------------------------------------------------------------------

func TestMeHandler(t *testing.T) {
	t.Parallel()

	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		svc := &mockAuthService{
			getUserFn: func(_ context.Context, userID string) (*model.User, error) {
				return &model.User{
					ID:        userID,
					Email:     "alice@example.com",
					Name:      "Alice",
					CreatedAt: time.Now().UTC().Truncate(time.Second),
				}, nil
			},
		}
		h := newTestHandler(svc)
		router := h.Routes()

		token := generateTestJWT(t, "user-1", "alice@example.com", time.Now().Add(15*time.Minute))
		req := httptest.NewRequest("GET", "/me", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		resp := decodeResponse(t, rec.Body.Bytes())
		assert.Equal(t, true, resp["success"])
		data := resp["data"].(map[string]any)
		assert.Equal(t, "user-1", data["id"])
		assert.Equal(t, "alice@example.com", data["email"])
		assert.Equal(t, "Alice", data["name"])
	})

	t.Run("NoAuth", func(t *testing.T) {
		t.Parallel()
		svc := &mockAuthService{
			getUserFn: func(_ context.Context, _ string) (*model.User, error) {
				t.Fatal("GetUser should not be called without auth")
				return nil, nil
			},
		}
		h := newTestHandler(svc)
		router := h.Routes()

		req := httptest.NewRequest("GET", "/me", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)

		resp := decodeResponse(t, rec.Body.Bytes())
		assert.Equal(t, false, resp["success"])
		errObj := resp["error"].(map[string]any)
		assert.Equal(t, "UNAUTHORIZED", errObj["code"])
	})

	t.Run("ExpiredToken", func(t *testing.T) {
		t.Parallel()
		svc := &mockAuthService{}
		h := newTestHandler(svc)
		router := h.Routes()

		token := generateTestJWT(t, "user-1", "alice@example.com", time.Now().Add(-1*time.Hour))
		req := httptest.NewRequest("GET", "/me", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("UserNotFound", func(t *testing.T) {
		t.Parallel()
		svc := &mockAuthService{
			getUserFn: func(_ context.Context, _ string) (*model.User, error) {
				return nil, service.ErrUserNotFound
			},
		}
		h := newTestHandler(svc)
		router := h.Routes()

		token := generateTestJWT(t, "deleted-user", "gone@example.com", time.Now().Add(15*time.Minute))
		req := httptest.NewRequest("GET", "/me", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

// ---------------------------------------------------------------------------
// Logout handler tests
// ---------------------------------------------------------------------------

func TestLogoutHandler(t *testing.T) {
	t.Parallel()

	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		logoutCalled := false
		svc := &mockAuthService{
			logoutFn: func(_ context.Context, rawToken string) error {
				logoutCalled = true
				assert.Equal(t, "my-refresh-token", rawToken)
				return nil
			},
		}
		h := newTestHandler(svc)
		router := h.Routes()

		req := httptest.NewRequest("POST", "/logout", nil)
		req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "my-refresh-token"})
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.True(t, logoutCalled, "Logout should be called")
		assert.Equal(t, http.StatusOK, rec.Code)

		resp := decodeResponse(t, rec.Body.Bytes())
		assert.Equal(t, true, resp["success"])

		// Verify cookies are cleared.
		cookies := rec.Result().Cookies()
		for _, c := range cookies {
			if c.Name == "access_token" || c.Name == "refresh_token" {
				assert.Equal(t, -1, c.MaxAge, "%s cookie should be cleared", c.Name)
			}
		}
	})

	t.Run("NoToken_StillReturns200", func(t *testing.T) {
		t.Parallel()
		svc := &mockAuthService{
			logoutFn: func(_ context.Context, _ string) error {
				t.Fatal("Logout should not be called when no token is provided")
				return nil
			},
		}
		h := newTestHandler(svc)
		router := h.Routes()

		req := httptest.NewRequest("POST", "/logout", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		// Logout is idempotent — returns 200 even without a token.
		assert.Equal(t, http.StatusOK, rec.Code)

		resp := decodeResponse(t, rec.Body.Bytes())
		assert.Equal(t, true, resp["success"])
	})
}
