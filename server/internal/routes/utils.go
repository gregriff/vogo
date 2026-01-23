package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"golang.org/x/net/websocket"
)

func writeJSON(w http.ResponseWriter, data any) {
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
}

// receiveWithContext reads json into v from ws in a new goroutine and cancels
// the read if ctx is cancelled. Param v should be a pointer.
func receiveWithContext(ctx context.Context, ws *websocket.Conn, v any) error {
	var (
		recv sync.WaitGroup
		done = make(chan error, 1)
	)
	defer recv.Wait()

	recv.Go(func() {
		done <- websocket.JSON.Receive(ws, v)
	})

	select {
	case <-ctx.Done():
		ws.SetReadDeadline(time.Now()) // interrupt the read
		return ctx.Err()
	case err := <-done:
		return err
	}
}
