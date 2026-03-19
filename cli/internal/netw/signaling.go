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

// sendCandidates sends the caller's ICE candidates from ch to the websocket as they're gathered.
// It returns when there are no more candidates or the context is cancelled.
func sendCandidates(ctx context.Context, ws *websocket.Conn, ch <-chan webrtc.ICECandidateInit) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case candidate, ok := <-ch:
			if err := websocket.JSON.Send(ws, candidate); err != nil {
				return fmt.Errorf("error sending ice candidate: %w", err)
			}
			if !ok {
				log.Println("ice send complete")
				return nil
			}
		}
	}
}

type candidateType int

const (
	candidateICEOffer candidateType = iota
	canidateICEAnswer
)

// sendTaggedCandidates gathers local ICE candidates created for the connection's recipient
// and sends them to the server via the websocket in a new goroutine. It sends them with a
// [candidateType] tag to let the server know if they're ICE offers or answers.
func sendTaggedCandidates(
	ctx context.Context,
	ws *websocket.Conn,
	conn *Connection,
	callerName string,
	tag candidateType,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case candidate, ok := <-conn.Candidates:
			bytes, err := json.Marshal(messages.Candidate{
				UserId:    conn.Id,
				Username:  callerName,
				Candidate: candidate,
			})
			if err != nil {
				log.Panicf("error encoding candidate: %v", err)
			}

			var tagStr string
			if tag == candidateICEOffer {
				tagStr = "ice-offer"
			} else {
				tagStr = "ice-answer"
			}
			msg := wsock.Message{Type: tagStr, Data: bytes}
			if err := websocket.JSON.Send(ws, msg); err != nil {
				log.Printf("error sending candidate: %v", err)
				conn.Close()
				return
			}
			if !ok {
				return
			}
		}
	}
}
