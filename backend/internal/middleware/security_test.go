package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSecurityHeaders(t *testing.T) {
	t.Parallel()

	t.Run("AllHeadersSet", func(t *testing.T) {
		t.Parallel()

		handler := SecurityHeaders()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		// Verify all security headers are present.
		tests := []struct {
			header    string
			expected  string
		}{
			{"X-Content-Type-Options", "nosniff"},
			{"X-Frame-Options", "DENY"},
			{"Referrer-Policy", "strict-origin-when-cross-origin"},
			{"Permissions-Policy", "geolocation=(), microphone=(), camera=()"},
			{"Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload"},
			{"Cache-Control", "no-store, no-cache, must-revalidate"},
			{"Pragma", "no-cache"},
		}

		for _, tc := range tests {
			t.Run(tc.header, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, tc.expected, rec.Header().Get(tc.header),
					"header %s should be set correctly", tc.header)
			})
		}
	})

	t.Run("HeadersSetBeforeHandler", func(t *testing.T) {
		t.Parallel()

		var receivedHeaders http.Header
		handler := SecurityHeaders()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedHeaders = w.Header().Clone()
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		// The downstream handler should see the headers already set.
		assert.Contains(t, receivedHeaders.Get("X-Content-Type-Options"), "nosniff")
		assert.Contains(t, receivedHeaders.Get("X-Frame-Options"), "DENY")
		assert.Contains(t, receivedHeaders.Get("Strict-Transport-Security"), "max-age")
	})
}
