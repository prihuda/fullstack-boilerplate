package middleware

import "net/http"

// SecurityHeaders returns a middleware that sets security-related HTTP headers.
func SecurityHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:;")
			w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
			next.ServeHTTP(w, r)
		})
	}
}
