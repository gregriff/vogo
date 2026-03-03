package internal

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gregriff/vogo/server/internal/db"
	"github.com/gregriff/vogo/server/internal/logging"
	"github.com/gregriff/vogo/server/internal/middleware"
	"github.com/gregriff/vogo/server/internal/routes"
	"golang.org/x/net/websocket"
)

func CreateAndListen(debug bool, host string, port int, logOpts logging.Opts) {
	log := logging.New(logOpts)

	db := db.GetDB()
	defer func() {
		if err := db.Close(); err != nil {
			log.Error("closing database", "err", err)
			os.Exit(1)
		}
	}()

	// Initialize handlers with dependencies
	h := routes.NewRouteHandler(db, logOpts)

	mux := http.NewServeMux()
	createRoutes(mux, h)

	httpLogger := logging.New(logOpts)

	// apply middlewares
	handler := middleware.Logging(mux, httpLogger)
	handler = middleware.BasicAuth(handler, db, logOpts)

	server := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", host, port),
		ReadHeaderTimeout: 500 * time.Millisecond,
		ReadTimeout:       500 * time.Millisecond,
		IdleTimeout:       500 * time.Millisecond,
		// Handler:           http.TimeoutHandler(handler, 30*time.Second, ""),
		Handler: handler,
	}

	// graceful shutdown channel
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// run server
	go func() {
		log.Info("starting server", "addr", server.Addr)
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server error", "err", err)
			os.Exit(1)
		}
		log.Info("stopped serving new connections")
	}()

	// recieve stop signals
	<-sigChan

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Error("http server shutdown error", "err", err)
		return
	}
	log.Info("graceful shutdown complete")
}

// createRoutes creates the routing rules for the webserver
func createRoutes(mux *http.ServeMux, h *routes.RouteHandler) {
	mux.HandleFunc("POST /register", h.Register)
	mux.HandleFunc("GET /status", h.Status)
	mux.HandleFunc("POST /friend", h.AddFriend)
	mux.HandleFunc("POST /channel", h.CreateChannel)
	mux.HandleFunc("POST /channel/invite", h.InviteFriend)

	callHandler := websocket.Server{
		Handshake: websocketHandshake,
		Handler:   h.Call,
	}
	answerHandler := websocket.Server{
		Handshake: websocketHandshake,
		Handler:   h.Answer,
	}
	joinHandler := websocket.Server{
		Handshake: websocketHandshake,
		Handler:   h.JoinRoom,
	}
	mux.Handle("GET /call", callHandler)
	mux.Handle("GET /answer/{name}", answerHandler)
	mux.Handle("GET /channel/join", joinHandler)
}

func websocketHandshake(_ *websocket.Config, _ *http.Request) error { return nil }
