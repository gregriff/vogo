package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gregriff/vogo/server/internal/schemas"
	"github.com/gregriff/vogo/server/internal/state"
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

type Message struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type MessageHandler func(ru *state.RoomUser, msg json.RawMessage) error

// TODO: this doesnt need to be a func. recv from recvwithcontext in the top-level select?
func startMessageLoop(
	ctx context.Context,
	ws *websocket.Conn,
	ru *state.RoomUser,
	handlers map[string]MessageHandler,
) error {
	for {
		var msg Message
		if err := receiveWithContext(ctx, ws, &msg); err != nil {
			// TODO: caller needs to handle EOF, which would signal the client has closed the websocket.
			// OR, this func needs to handle it, and send that err to an abort chan thats passed as an arg for caller to select on
			return err
		}

		handler, ok := handlers[msg.Type]
		if !ok {
			log.Printf("unknown message type: %s", msg.Type)
			continue
		}

		handler(ru, msg.Data)
	}
}

var Handlers = map[string]MessageHandler{
	// todo: will add ConnectionMessage (offer for one existing user) once thats parallelized
	//
	// recv client's own ice candidate. send to answerer's chan or close if done
	"ice": func(ru *state.RoomUser, data json.RawMessage) error {
		var msg schemas.CandidateMessage
		json.Unmarshal(data, &msg)
		conn, err := ru.PendingConnections.Get(msg.UserId)
		if err != nil {
			return fmt.Errorf("unable to get conn for %s in ICEMessageHandler: ", msg.Username)
		}
		if msg.Candidate.Candidate == "" {
			close(conn.From.Candidates)
			log.Println("ice gather completed")
			return nil
		}
		conn.From.Candidates <- msg.Candidate
		return nil
	},
	"answer": func(ru *state.RoomUser, data json.RawMessage) error {
		var msg schemas.AnswerRequest
		json.Unmarshal(data, &msg)
		return nil
	},
}
