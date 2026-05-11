package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const testAuthSecret = "test-auth-middleware-secret"

func generateTestToken(t *testing.T, secret, userID, email string, expiresAt time.Time) string {
	t.Helper()
	claims := JWTClaims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := token.SignedString([]byte(secret))
	require.NoError(t, err)
	return s
}

func decodeAuthResponse(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var resp map[string]any
	err := json.Unmarshal(body, &resp)
	require.NoError(t, err)
	return resp
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestAuthMiddleware(t *testing.T) {
	t.Parallel()

	// Success handler that returns the user ID from context.
	successHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := GetUserID(r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"user_id": userID})
	})

	mw := AuthMiddleware(testAuthSecret)

	t.Run("ValidToken_ViaHeader", func(t *testing.T) {
		t.Parallel()

		token := generateTestToken(t, testAuthSecret, "user-42", "alice@example.com", time.Now().Add(15*time.Minute))
		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		mw(successHandler).ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var data map[string]string
		err := json.Unmarshal(rec.Body.Bytes(), &data)
		require.NoError(t, err)
		assert.Equal(t, "user-42", data["user_id"])
	})

	t.Run("ValidToken_ViaCookie", func(t *testing.T) {
		t.Parallel()

		token := generateTestToken(t, testAuthSecret, "user-99", "bob@example.com", time.Now().Add(15*time.Minute))
		req := httptest.NewRequest("GET", "/protected", nil)
		req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
		rec := httptest.NewRecorder()

		mw(successHandler).ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var data map[string]string
		err := json.Unmarshal(rec.Body.Bytes(), &data)
		require.NoError(t, err)
		assert.Equal(t, "user-99", data["user_id"])
	})

	t.Run("CookieTakesPriority", func(t *testing.T) {
		t.Parallel()

		validToken := generateTestToken(t, testAuthSecret, "cookie-user", "c@example.com", time.Now().Add(15*time.Minute))
		invalidToken := generateTestToken(t, "wrong-secret", "header-user", "h@example.com", time.Now().Add(15*time.Minute))

		req := httptest.NewRequest("GET", "/protected", nil)
		req.AddCookie(&http.Cookie{Name: "access_token", Value: validToken})
		req.Header.Set("Authorization", "Bearer "+invalidToken)
		rec := httptest.NewRecorder()

		mw(successHandler).ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var data map[string]string
		err := json.Unmarshal(rec.Body.Bytes(), &data)
		require.NoError(t, err)
		// Cookie should take priority over header.
		assert.Equal(t, "cookie-user", data["user_id"])
	})

	t.Run("InvalidToken", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer this-is-not-a-valid-jwt")
		rec := httptest.NewRecorder()

		mw(successHandler).ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)

		resp := decodeAuthResponse(t, rec.Body.Bytes())
		assert.Equal(t, false, resp["success"])
		errObj := resp["error"].(map[string]any)
		assert.Equal(t, "UNAUTHORIZED", errObj["code"])
	})

	t.Run("ExpiredToken", func(t *testing.T) {
		t.Parallel()

		token := generateTestToken(t, testAuthSecret, "user-old", "old@example.com", time.Now().Add(-1*time.Hour))
		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		mw(successHandler).ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("MissingHeader", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest("GET", "/protected", nil)
		rec := httptest.NewRecorder()

		mw(successHandler).ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)

		resp := decodeAuthResponse(t, rec.Body.Bytes())
		assert.Equal(t, false, resp["success"])
		errObj := resp["error"].(map[string]any)
		assert.Equal(t, "UNAUTHORIZED", errObj["code"])
		assert.Equal(t, "missing authentication token", errObj["message"])
	})

	t.Run("WrongSigningMethod", func(t *testing.T) {
		t.Parallel()

		// Create a token signed with a different algorithm (none).
		claims := JWTClaims{
			UserID: "evil",
			Email:  "evil@example.com",
		}
		token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
		s, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
		require.NoError(t, err)

		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+s)
		rec := httptest.NewRecorder()

		mw(successHandler).ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

func TestGetUserID(t *testing.T) {
	t.Parallel()

	t.Run("ReturnsEmptyWithoutContext", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest("GET", "/", nil)
		assert.Empty(t, GetUserID(req))
	})
}
