package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecover(t *testing.T) {
	t.Parallel()

	t.Run("PanickingHandler", func(t *testing.T) {
		t.Parallel()

		logger := slog.Default()
		handler := Recover(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("something terrible happened")
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		// Should not panic — Recover catches it.
		assert.NotPanics(t, func() {
			handler.ServeHTTP(rec, req)
		})

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

		var resp map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		assert.Equal(t, false, resp["success"])

		errObj := resp["error"].(map[string]any)
		assert.Equal(t, "INTERNAL_ERROR", errObj["code"])
		assert.Equal(t, "An internal error occurred", errObj["message"])
	})

	t.Run("NonPanickingHandler_PassesThrough", func(t *testing.T) {
		t.Parallel()

		logger := slog.Default()
		handler := Recover(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("all good"))
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "all good", rec.Body.String())
	})

	t.Run("PanicWithNonString", func(t *testing.T) {
		t.Parallel()

		logger := slog.Default()
		handler := Recover(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic(42) // panic with non-string value
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		assert.NotPanics(t, func() {
			handler.ServeHTTP(rec, req)
		})

		assert.Equal(t, http.StatusInternalServerError, rec.Code)

		var resp map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, false, resp["success"])
	})
}
