package netw

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/gregriff/vogo/cli/internal/netw/wrtc"
	"github.com/pion/webrtc/v4"
	"golang.org/x/net/websocket"
)

// SendCallAndConnect creates and establishes a voice call with a friend client, if
// they answer the call. It uses a websocket connection to a vogo server to handle
// signaling and connecting, and uses trickle-ICE for fast connection. It assumes
// a PeerConnection set up correctly for opus audio.
func SendCallAndConnect(
	ctx context.Context,
	pc *webrtc.PeerConnection,
	credentials *credentials,
	recipient string,
	candidates <-chan webrtc.ICECandidateInit,
	abort chan<- error,
) error {
	ws, err := newWebsocket(ctx, credentials, "/call")
	if err != nil {
		return fmt.Errorf("error creating websocket: %w", err)
	}

	if err = wrtc.CreateAndSendOffer(ws, pc, recipient); err != nil {
		return err
	}

	var iceWg sync.WaitGroup
	iceCtx, cancelSendIce := context.WithCancel(ctx)
	defer func() {
		cancelSendIce()
		iceWg.Wait()
	}()

	// gather local ice candidates and write to websocket
	iceWg.Go(func() {
		defer cancelSendIce()
		if err = sendCandidates(iceCtx, ws, candidates); err != nil {
			abort <- err // this will cause surrounding function to cancel
		}
	})

	// wait to recv answer
	var answer webrtc.SessionDescription
	if err = websocket.JSON.Receive(ws, &answer); err != nil {
		return fmt.Errorf("error reading answer from ws: %v", err)
	}
	if err = pc.SetRemoteDescription(answer); err != nil {
		return fmt.Errorf("error while setting remote description: %w", err)
	}
	log.Println("recieved answer")

	var (
		readWg              sync.WaitGroup
		recipientCandidates = make(chan webrtc.ICECandidateInit)
	)
	defer closeAndWait(ws, &readWg)

	// read recipient candidates from ws as they come in
	readWg.Go(func() {
		readCandidates(ws, recipientCandidates)
	})
	if err = addCandidates(ctx, pc, recipientCandidates); err != nil {
		return err
	}
	return nil
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
			log.Println("sent candidate")
			if !ok {
				log.Println("ice gathering completed")
				return nil
			}
		}
	}
}

// addCandidates adds the recipient's ICE candidates from ch to the peer connection until
// there are no more or the context is cancelled.
func addCandidates(ctx context.Context, pc *webrtc.PeerConnection, ch <-chan webrtc.ICECandidateInit) error {
	for {
		select {
		case <-ctx.Done():
			log.Println("ws caller ctx cancelled")
			return nil
		case candidate, ok := <-ch: // from the websocket
			if !ok {
				log.Println("no more recipient candidates")
				ch = nil
				continue
			}
			log.Println("recv recipient candidate")
			if err := pc.AddICECandidate(candidate); err != nil {
				return fmt.Errorf("error recieving ICE candidate: %w", err)
			}
		}
	}
}
