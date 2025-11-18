// package core contains code wrapping websockets and webrtc signaling and connecting.
package core

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/pion/webrtc/v4"
	"golang.org/x/net/websocket"
)

// AnswerAndConnect answers and establishes a voice call with a friend client. It
// uses a websocket connection to a vogo server to handle signaling and connecting.
// It uses trickle-ICE for fast connection. It assumes a PeerConnection set up
// correctly for opus audio.
func AnswerAndConnect(
	ctx context.Context,
	pc *webrtc.PeerConnection,
	credentials *credentials,
	caller string,
	candidates <-chan webrtc.ICECandidateInit,
) error {
	endpoint := fmt.Sprintf("/answer/%s", caller)
	ws, err := newWebsocket(ctx, credentials, endpoint)
	if err != nil {
		return fmt.Errorf("error creating websocket: %w", err)
	}

	offer, err := recieveOffer(ws)
	if err != nil {
		return fmt.Errorf("error recieving offer: %w", err)
	}
	err = createAndSendAnswer(ws, pc, offer, caller)
	if err != nil {
		return fmt.Errorf("error creating or posting answer %w", err)
	}
	log.Println("answer sent")

	// read caller candidates from ws as they come in
	var (
		readWg           sync.WaitGroup
		callerCandidates = make(chan webrtc.ICECandidateInit)
	)
	defer closeAndWait(ws, &readWg)

	readWg.Go(func() {
		readCandidates(ws, callerCandidates)
	})
	err = exchangeCandidates(ctx, ws, pc, candidates, callerCandidates)
	if err != nil {
		return err
	}
	return nil
}

// exchangeCandidates handles sending the client's ICE candidates to the websocket to be
// forwarded to the caller, and adding the caller's candidates recieved from the websocket.
// It does this until both sources are exhausted, or until the context is cancelled.
func exchangeCandidates(
	ctx context.Context,
	ws *websocket.Conn,
	pc *webrtc.PeerConnection,
	candidates, callerCandidates <-chan webrtc.ICECandidateInit,
) error {
	for {
		select {
		case <-ctx.Done():
			log.Println("ws answer ctx cancelled")
			return nil
		case candidate, ok := <-candidates:
			if err := websocket.JSON.Send(ws, candidate); err != nil {
				return fmt.Errorf("error sending ice candidate: %w", err)
			}
			log.Println("sent candidate")
			if !ok {
				log.Println("gathering completed")
				candidates = nil
				continue
			}
		// recv caller candidates from the websocket
		case callerCandidate, ok := <-callerCandidates:
			if !ok {
				log.Println("no more caller candidates")
				callerCandidates = nil
				continue
			}
			log.Println("recv caller candidate")
			if err := pc.AddICECandidate(callerCandidate); err != nil {
				return fmt.Errorf("error recieving ICE candidate: %w", err)
			}
		}
	}
}
