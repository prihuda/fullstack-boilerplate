package middleware

import (
	"log/slog"
	"net/http"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"
)

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
	written    int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.statusCode == 0 {
		r.statusCode = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.written += n
	return n, err
}

// Unwrap exposes the underlying ResponseWriter for compatibility with
// http.Flusher, http.Hijacker, etc.
func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

func Logger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}

			defer func() {
				duration := time.Since(start)
				msg := r.Method + " " + r.URL.Path

				attrs := []slog.Attr{
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.Int("status", rec.statusCode),
					slog.Int64("duration_ms", duration.Milliseconds()),
					slog.String("remote_addr", r.RemoteAddr),
					slog.Int("bytes_written", rec.written),
				}

				if reqID := chimw.GetReqID(r.Context()); reqID != "" {
					attrs = append(attrs, slog.String("request_id", reqID))
				}

				if userID := GetUserID(r); userID != "" {
					attrs = append(attrs, slog.String("user_id", userID))
				}

				switch {
				case rec.statusCode >= 500:
					logger.LogAttrs(r.Context(), slog.LevelError, msg, attrs...)
				case rec.statusCode >= 400:
					logger.LogAttrs(r.Context(), slog.LevelWarn, msg, attrs...)
				default:
					logger.LogAttrs(r.Context(), slog.LevelInfo, msg, attrs...)
				}
			}()

			next.ServeHTTP(rec, r)
		})
	}
}
