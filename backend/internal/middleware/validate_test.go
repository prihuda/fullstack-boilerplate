package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rhuda/fullstack-boilerplate/backend/internal/model"
)

// testValidateHandler wraps ValidateRequest in an HTTP handler so we can
// observe both the parsed result and any error written to the response.
func testValidateHandler(w http.ResponseWriter, r *http.Request) {
	req := ValidateRequest[model.LoginRequest](w, r)
	if req == nil {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"email":    req.Email,
		"password": req.Password,
	})
}

func TestValidateRequest(t *testing.T) {
	t.Parallel()

	t.Run("ValidBody", func(t *testing.T) {
		t.Parallel()

		body := `{"email":"user@example.com","password":"secret123"}`
		req := httptest.NewRequest("POST", "/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		testValidateHandler(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var data map[string]string
		err := json.Unmarshal(rec.Body.Bytes(), &data)
		require.NoError(t, err)
		assert.Equal(t, "user@example.com", data["email"])
		assert.Equal(t, "secret123", data["password"])
	})

	t.Run("InvalidEmail", func(t *testing.T) {
		t.Parallel()

		body := `{"email":"not-an-email","password":"secret123"}`
		req := httptest.NewRequest("POST", "/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		testValidateHandler(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)

		var resp map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, false, resp["success"])

		errObj := resp["error"].(map[string]any)
		assert.Equal(t, "VALIDATION_ERROR", errObj["code"])
		assert.Contains(t, errObj, "details")

		details := errObj["details"].(map[string]any)
		assert.Contains(t, details, "email")
	})

	t.Run("MissingRequiredField", func(t *testing.T) {
		t.Parallel()

		body := `{"email":"user@example.com"}`
		req := httptest.NewRequest("POST", "/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		testValidateHandler(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)

		var resp map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, false, resp["success"])

		errObj := resp["error"].(map[string]any)
		assert.Equal(t, "VALIDATION_ERROR", errObj["code"])

		details := errObj["details"].(map[string]any)
		assert.Contains(t, details, "password")
	})

	t.Run("EmptyBody", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest("POST", "/login", nil)
		rec := httptest.NewRecorder()

		testValidateHandler(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)

		var resp map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, false, resp["success"])

		errObj := resp["error"].(map[string]any)
		assert.Equal(t, "INVALID_REQUEST", errObj["code"])
	})

	t.Run("OversizedBody", func(t *testing.T) {
		t.Parallel()

		// MaxBytesReader limit is 4KB (4<<10). Create a body larger than that.
		longPassword := strings.Repeat("a", 5000)
		body := `{"email":"user@example.com","password":"` + longPassword + `"}`
		req := httptest.NewRequest("POST", "/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		testValidateHandler(rec, req)

		// MaxBytesReader causes Decode to fail → 400 (not 413).
		assert.Equal(t, http.StatusBadRequest, rec.Code)

		var resp map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, false, resp["success"])
	})
}

func TestToSnakeCase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"Email", "email"},
		{"Password", "password"},
		{"UserID", "user_id"},
		{"CreatedAt", "created_at"},
		{"EntryDate", "entry_date"},
		{"HTMLParser", "html_parser"},
		{"IPAddress", "ip_address"},
		{"ID", "id"},
		{"simple", "simple"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, toSnakeCase(tc.input))
		})
	}
}
