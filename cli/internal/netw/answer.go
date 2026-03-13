// package netw implements high-level networking functionality to enable p2p voice chat.
// It handles the client-side connection process, using the wrtc package for signaling.
// In addition, CRUD operations with the vogo server are contained here.
// Many of the public functions in netw map directly to cli commands.
package netw

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/gregriff/vogo/cli/internal/audio"
	"github.com/gregriff/vogo/cli/internal/netw/wrtc"
	"github.com/gregriff/vogo/shared/requests"
	"github.com/gregriff/vogo/shared/wsock"
	"github.com/pion/webrtc/v4"
	"golang.org/x/net/websocket"
	"golang.org/x/sync/errgroup"
)

// Answer establishes a bidirectional voice call with a caller if a call is pending.
// Signaling, speaker init, connecting and microphone init are all run concurrently,
// organized with errgroups and synchronized with channels. The entire process can
// be cancelled with the provided context, and the first error encountered will be returned.
func AnswerCall(ctx context.Context, creds *credentials, caller string) error {
	track := wrtc.CreateAudioTrack(creds.username)
	conn := wrtc.NewConnection(uuid.New(), creds.stunServer, track, true)
	audioState := audio.NewCall(track, wrtc.RecvMTU)
	audioState.AddPeer(conn.Pc)
	defer func() {
		if err := wrtc.ClosePC(conn.Pc, true); err != nil {
			log.Println(err)
		}
	}()

	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		start := time.Now()
		defer func() {
			log.Printf("playback device created in %v", time.Since(start))
		}()
		return audioState.Speaker.Init(audioState.DataProc())
	})
	defer func() {
		if err := conn.Pc.GracefulClose(); err != nil {
			log.Printf("error gracefully closing peer connection: %v\n", err)
		}
		_ = audioState.Speaker.Uninit()
	}()

	g.Go(func() error {
		return answerAndConnect(gCtx, conn, creds, caller)
	})

	g.Go(func() error {
		return conn.HandleEvents(gCtx, caller)
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
			<-audioState.Speaker.Initialized()
			if err := audioState.Speaker.Start(); err != nil {
				return err
			}
			break
		}

		if err := audioState.Mic.Start(gCtx); err != nil {
			return fmt.Errorf("error starting mic: %w", err)
		}
		return nil
	})

	return g.Wait()
}

// answerAndConnect answers and establishes a voice call with a friend client. It
// uses a websocket connection to a vogo server to handle signaling and connecting.
// It uses trickle-ICE for fast connection. It assumes a PeerConnection set up
// correctly for opus audio.
func answerAndConnect(
	ctx context.Context,
	conn *wrtc.Connection,
	credentials *credentials,
	caller string,
) error {
	endpoint := fmt.Sprintf("/answer/%s", caller)
	ws, err := newWebsocket(ctx, credentials, endpoint)
	if err != nil {
		return fmt.Errorf("error creating websocket: %w", err)
	}
	defer func() {
		_ = closeAndWait(ws, nil)
	}()

	offer, err := recvOffer(ctx, ws)
	if err != nil {
		return fmt.Errorf("error receiving offer: %w", err)
	}

	err = conn.CreateAnswer(offer)
	if err != nil {
		return fmt.Errorf("error creating answer %w", err)
	}
	req := requests.Connection{To: caller, Sd: *conn.Pc.LocalDescription()}
	if err = websocket.JSON.Send(ws, req); err != nil {
		return fmt.Errorf("error sending answer: %w", err)
	}
	log.Println("answer sent")

	g, gCtx := errgroup.WithContext(ctx)
	// defer closeAndWait(ws, g)

	// gather local ice candidates and write to websocket
	g.Go(func() error {
		return sendCandidates(gCtx, ws, conn.Candidates)
	})

	// recv caller candidates
	g.Go(func() error {
		return recvCandidates(gCtx, ws, conn.Pc)
	})

	return g.Wait()
}

// recvOffer reads the caller's offer from the websocket and returns it.
// It blocks while waiting to read from the ws.
func recvOffer(ctx context.Context, ws *websocket.Conn) (*webrtc.SessionDescription, error) {
	var offer webrtc.SessionDescription
	if err := wsock.ReceiveJSON(ctx, ws, &offer); err != nil {
		if err == io.EOF {
			// TODO: this doesn't necessarily mean call not found. request ws could have closed on an error...
			return nil, fmt.Errorf("call not found") // should make this a sentinal
		}
		return nil, fmt.Errorf("error reading from ws: %v", err)
	}
	return &offer, nil
}
