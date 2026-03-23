package netw

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/gregriff/vogo/shared/wsock"
	"github.com/gregriff/vogo/shared/wsock/messages"
	"github.com/pion/webrtc/v4"
	"golang.org/x/net/websocket"
)

// recvCandidates reads recipient candidates from ws and adds them to the pc.
func recvCandidates(ctx context.Context, ws *websocket.Conn, pc *webrtc.PeerConnection) error {
	var candidate webrtc.ICECandidateInit
	var count int
	for {
		err := wsock.ReceiveJSON(ctx, ws, &candidate)
		if err != nil {
			return fmt.Errorf("error reading from ws: %w", err)
		}
		if candidate.Candidate == "" {
			log.Printf("ice recv completed, %d candidates total", count)
			return nil
		}
		count++
		if err := pc.AddICECandidate(candidate); err != nil {
			log.Println("WARN: error adding ICE candidate: %w", err)
		}
	}
}

// sendCandidates gathers local ICE candidates created for the connection's recipient
// and sends them to the server via the websocket in a new goroutine. It sends them with a
// [candidateType] tag to let the server know if they're ICE offers or answers. It returns when
// gathering is complete, an error occurs or the context is cancelled.
func sendCandidates(
	ctx context.Context,
	ws *websocket.Conn,
	conn *Connection,
	callerName string,
	tag wsock.MessageType,
) error {
	if !tag.IsICEMessage() {
		log.Panicf("sendCandidates passed a non-ICE MessageType")
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case candidate, ok := <-conn.Candidates:
			bytes, err := json.Marshal(messages.Candidate{
				UserId:    conn.Id,
				Username:  callerName,
				Candidate: candidate,
			})
			if err != nil {
				log.Panicf("error encoding candidate: %v", err)
			}

			msg := wsock.Message{Type: tag, Data: bytes}
			if err := websocket.JSON.Send(ws, msg); err != nil {
				log.Printf("error sending candidate: %v", err)
				conn.Close()
				return err
			}
			if !ok {
				log.Println("ice sending complete")
				return nil
			}
		}
	}
}

// notifyConnected sends a message to the vogo server to signal that this client's
// PeerConnection with peerName is in a Connected state.
func notifyConnected(ws *websocket.Conn, username, peerName string) error {
	data := messages.Connected{Username: username, PeerName: peerName}
	return websocket.JSON.Send(ws, &data)
}
