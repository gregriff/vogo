package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"
)

const requestIdKey contextKey = "request_id"

func generateId() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b) // e.g. "a3f9c1d2"
}

// GetRequestId is used in endpoint handlers to retrieve the Id generated for this request.
func GetRequestId(r *http.Request) string {
	id, _ := r.Context().Value(requestIdKey).(string)
	return id
}

// Logging is a middleware that generates a request Id for each request and logs
// attributes about its request and response.
func Logging(next http.Handler, logger *slog.Logger) http.Handler {
	logCtx := context.Background()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		username := GetUsername(r)

		id := generateId()
		ctx := context.WithValue(r.Context(), requestIdKey, id)
		w.Header().Set("X-Request-ID", id)

		enabled := logger.Enabled(logCtx, slog.LevelInfo)
		mAttr, pAttr, idAttr, uAttr := slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("request_id", id),
			slog.String("username", username)

		if enabled {
			logger.LogAttrs(logCtx, slog.LevelInfo, "request",
				mAttr, pAttr, idAttr, uAttr,
			)
		}

		next.ServeHTTP(w, r.WithContext(ctx))

		// logger.Info("request completed",
		// 	"method", r.Method,
		// 	"path", r.URL.Path,
		// 	"request_id", id,
		// 	"duration_ms", time.Since(start).Milliseconds(),
		// )

		if enabled {
			logger.LogAttrs(logCtx, slog.LevelInfo, "response",
				mAttr, pAttr, idAttr, uAttr,
				slog.Duration("duration_ms", time.Since(start)),
			)
		}
	})
}
