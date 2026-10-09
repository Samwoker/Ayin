package middleware

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recoverer recovers from panics and logs the stack trace cleanly with structured slog.
func Recoverer(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rvr := recover(); rvr != nil {
					reqID := GetRequestID(r.Context())
					stack := string(debug.Stack())

					logger.ErrorContext(r.Context(), "panic recovered",
						slog.String("request_id", reqID),
						slog.String("error", fmt.Sprintf("%v", rvr)),
						slog.String("stack", stack),
					)

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(w).Encode(map[string]any{
						"error":      "Internal Server Error",
						"request_id": reqID,
					})
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
