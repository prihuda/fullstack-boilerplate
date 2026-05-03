package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime/debug"

	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/rhuda/fullstack-boilerplate/backend/internal/model"
)

// Recover returns middleware that recovers from panics and returns a JSON 500 response.
func Recover(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					attrs := []any{
						slog.Any("error", err),
						slog.String("stack", string(debug.Stack())),
					}
					if reqID := chimw.GetReqID(r.Context()); reqID != "" {
						attrs = append(attrs, slog.String("request_id", reqID))
					}
					logger.Error("panic recovered", attrs...)

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)

					resp := model.NewErrorResponse("INTERNAL_ERROR", "An internal error occurred")
					_ = json.NewEncoder(w).Encode(map[string]any{
						"success": false,
						"error":   resp,
					})
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
