// package wsock provides websocket utilities
package wsock

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"golang.org/x/net/websocket"
)

// MessageType defines the allowed message types to be sent over a vogo websocket.
type MessageType string

const (
	Offer     MessageType = "offer"
	Answer    MessageType = "answer"
	Connected MessageType = "connected"
	ICEOffer  MessageType = "ice-offer"
	ICEAnswer MessageType = "ice-answer"
)

func (t MessageType) IsICEMessage() bool {
	switch t {
	case ICEOffer, ICEAnswer:
		return true
	default:
		return false
	}
}

// Message is data tied to an event (Type) sent between client and server via
// websocket. Use Listen() and a switch statement to mux Messages from multiple senders at once.
type Message struct {
	Type MessageType     `json:"type"`
	Data json.RawMessage `json:"data"`
}

// Listen reads from ws until it is closed or errors, and sends the data it reads to ch.
// It assumes ws frames are structured according to the Message struct. It enables the caller to react
// to ws frames in an event-based manner.
func Listen(ctx context.Context, ws *websocket.Conn, ch chan<- Message) error {
	for {
		var msg Message
		if err := ReceiveJSON(ctx, ws, &msg); err != nil {
			return err
		}
		ch <- msg
	}
}

// ReceiveJSON reads json into v from ws in a new goroutine and cancels
// the read if ctx is cancelled, waiting for the spawned goroutine to finish.
func ReceiveJSON[T any](ctx context.Context, ws *websocket.Conn, v *T) error {
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
		if err := ws.SetReadDeadline(time.Now()); err != nil { // interrupt the read
			return fmt.Errorf("context cancelled: %w; and error setting read deadline: %w", ctx.Err(), err)
		}
		return ctx.Err()
	case err := <-done:
		return err
	}
}
