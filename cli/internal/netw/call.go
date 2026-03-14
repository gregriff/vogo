package netw

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/gregriff/vogo/cli/internal/audio"
	"github.com/gregriff/vogo/cli/internal/netw/wrtc"
	"github.com/gregriff/vogo/shared/wsock"
	"github.com/pion/webrtc/v4"
	"golang.org/x/net/websocket"
	"golang.org/x/sync/errgroup"
)

// CallFriend creates a bidirectional voice call to the intended recipient.
// Signaling, speaker init, connecting and microphone init are all run concurrently,
// organized with waitgroups and synchronized with channels. The entire process can
// be cancelled with the provided context, and the first error encountered will be returned.
func CallFriend(ctx context.Context, creds *credentials, recipient string) error {
	track := wrtc.CreateAudioTrack(creds.username)
	conn := wrtc.NewConnection(uuid.New(), recipient, creds.stunServer, track, true)
	audioState := audio.NewCall(track, wrtc.RecvMTU)
	audioState.AddPeer(conn.Pc)
	defer conn.Close()

	// g.Go(func() error {
	// 	start := time.Now()
	// 	defer log.Printf("playback device created in %v", time.Since(start))
	// 	return audioState.Speaker.Init(audioState.DataProc())
	// })
	// defer func() {
	// 	if err := conn.Pc.GracefulClose(); err != nil {
	// 		log.Printf("error gracefully closing peer connection: %v\n", err)
	// 	}
	// 	_ = audioState.Speaker.Uninit()
	// }()

	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return sendCallAndConnect(gCtx, conn, creds, recipient)
	})

	g.Go(func() error {
		return conn.HandleEvents(gCtx, recipient)
	})

	// init microphone, and start it and the speaker once call is connected
	g.Go(func() error {
		// todo: could do this in another goroutine and use its init chan
		if err := audioState.Mic.Init(); err != nil {
			return err
		}
		defer func() {
			_ = audioState.Mic.Uninit()
		}()

		select {
		case <-gCtx.Done():
			return nil
		case <-conn.Connected:
			// <-audioState.Speaker.Initialized()
			// if err := audioState.StartSpeaker(); err != nil {
			// 	return err
			// }
			break
		}

		if err := audioState.Mic.Start(gCtx); err != nil {
			return fmt.Errorf("error starting mic: %w", err)
		}
		return nil
	})

	return g.Wait()
}

// sendCallAndConnect creates and establishes a voice call with a friend client, if
// they answer the call. It uses a websocket connection to a vogo server to handle
// signaling and connecting, and uses trickle-ICE for fast connection. It assumes
// a PeerConnection set up correctly for opus audio.
func sendCallAndConnect(
	ctx context.Context,
	conn *wrtc.Connection,
	creds *credentials,
	recipient string,
) error {
	ws, err := newWebsocket(ctx, creds, "/call")
	if err != nil {
		return fmt.Errorf("error creating websocket: %w", err)
	}
	defer func() {
		_ = closeAndWait(ws, nil)
	}()

	// send offer
	offer := conn.NewOffer(creds.username, recipient)
	if err = websocket.JSON.Send(ws, offer); err != nil {
		return fmt.Errorf("error sending offer: %w", err)
	}

	g, gCtx := errgroup.WithContext(ctx)
	defer func() {
		_ = closeAndWait(ws, g)
	}()

	// gather local ice candidates and write to websocket
	g.Go(func() error {
		return sendCandidates(gCtx, ws, conn.Candidates)
	})

	// wait to recv answer
	var answer webrtc.SessionDescription
	if err := wsock.ReceiveJSON(gCtx, ws, &answer); err != nil {
		return fmt.Errorf("error reading answer from ws: %v", err)
	}
	if err := conn.Pc.SetRemoteDescription(answer); err != nil {
		return fmt.Errorf("error while setting remote description: %w", err)
	}
	log.Println("received answer")

	// recv recipient candidates
	g.Go(func() error {
		return recvCandidates(gCtx, ws, conn.Pc)
	})

	// TODO: if sendIce needs to continue to run after it recvs last candidate, add:
	// <-conn.Connected
	return g.Wait()
}

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
