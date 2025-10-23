package internal

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gregriff/vogo/server/internal/db"
	"github.com/gregriff/vogo/server/internal/middleware"
	"github.com/gregriff/vogo/server/internal/routes"
)

func CreateAndListen(debug bool, host string, port int) {
	db := db.GetDB()
	defer db.Close()

	// Initialize handlers with dependencies
	h := routes.NewRouteHandler(db)

	mux := http.NewServeMux()
	createRoutes(mux, h)

	// apply middlewares
	var handler http.Handler
	if debug {
		handler = middleware.DebugLogging(mux)
	} else {
		handler = mux
	}
	handler = middleware.BasicAuth(mux, db)

	server := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", host, port),
		ReadHeaderTimeout: 500 * time.Millisecond,
		ReadTimeout:       500 * time.Millisecond,
		IdleTimeout:       500 * time.Millisecond,
		Handler:           http.TimeoutHandler(handler, 10*time.Second, ""),
	}

	// graceful shutdown channel
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// run server
	go func() {
		log.Printf("Starting server on %s", server.Addr)
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server error: %v", err)
		}
		log.Println("Stopped serving new connections.")
	}()

	// recieve stop signals
	<-sigChan

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("http shutdown error: %v", err)
	}
	log.Println("Graceful shutdown complete.")
}

// createRoutes creates the routing rules for the webserver
func createRoutes(mux *http.ServeMux, h *routes.RouteHandler) {
	mux.HandleFunc("POST /call", h.Call)
	mux.HandleFunc("POST /answer", h.Answer)
	mux.HandleFunc("GET /caller/{name}", h.Caller)

	mux.HandleFunc("POST /register", h.Register)
}
