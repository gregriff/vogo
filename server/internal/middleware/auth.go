package middleware

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gregriff/vogo/server/internal/crypto"
	"github.com/gregriff/vogo/server/internal/dal"
	"github.com/gregriff/vogo/server/internal/schemas"
)

type contextKey string

const authKey contextKey = "authorization"

// BasicAuth is a middleware that mandates basic auth is present in the headers and validates
func BasicAuth(next http.Handler, db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// whitelisted endpoints
		if r.URL.Path == "/register" {
			next.ServeHTTP(w, r)
			return
		}

		username, password, ok := r.BasicAuth()
		username = strings.Trim(username, " ")
		if !ok {
			writeAuthError(w)
			return
		}

		user := &schemas.User{}
		user, err := dal.GetUserByUsername(db, username)
		if err != nil || crypto.CompareHashAndPassword(user.Password, password) != nil {
			log.Println(fmt.Errorf("auth error: %w", err))
			writeAuthError(w)
			return
		}

		ctx := context.WithValue(r.Context(), authKey, username)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func writeAuthError(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Basic realm="basic-client"`)
	http.Error(w, "Unauthorized", http.StatusUnauthorized)
}

// GetUsername is used in endpoint handlers to retrieve the username of the client that created the request.
// This should probably be a func in the route handler since its a dependency
func GetUsername(r *http.Request) string {
	username, _ := r.Context().Value(authKey).(string)
	return username
}
