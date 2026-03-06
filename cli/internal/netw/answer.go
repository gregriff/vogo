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
	"sync"
	"time"

	"github.com/gregriff/vogo/cli/internal/audio"
	"github.com/gregriff/vogo/cli/internal/netw/wrtc"
	"github.com/gregriff/vogo/shared/requests"
	"github.com/gregriff/vogo/shared/wsock"
	"github.com/pion/webrtc/v4"
	"golang.org/x/net/websocket"
)

// Answer establishes a bidirectional voice call with a caller if a call is pending.
// Signaling, speaker init, connecting and microphone init are all run concurrently,
// organized with waitgroups and synchronized with channels. The entire process can
// be cancelled with the provided context, and the first error encountered will be returned.
func AnswerCall(ctx context.Context, creds *credentials, caller string) error {
	track := wrtc.CreateAudioTrack(creds.username)
	pc, candidates, connected := wrtc.NewAudioPeerConnection(creds.stunServer, track, true)
	defer func() {
		if err := wrtc.ClosePC(pc, true); err != nil {
			log.Println(err)
		}
	}()

	// sending an error on this channel will abort the call process
	abort := make(chan error, 10)
	defer func() {
		log.Println("ABORT ERRS:")
		select {
		case err := <-abort:
			log.Println(err)
		default:
			return
		}
	}()

	audioState := audio.NewCall(track)
	audioState.AddPeer(pc)
	go func() {
		// TODO: manually start devices onPeerStateConnecting
		start := time.Now()
		if err := audioState.Speaker.Init(audioState.DataProc()); err != nil {
			abort <- fmt.Errorf("error initializing playback system: %w", err)
			return
		}
		log.Printf("playback device created in %v", time.Since(start))
	}()
	defer func() {
		if err := pc.GracefulClose(); err != nil {
			log.Printf("error gracefully closing peer connection: %v\n", err)
		}
		audioState.Speaker.Uninit()
	}()

	var answer sync.WaitGroup
	answerCtx, cancelAnswer := context.WithCancel(ctx)
	defer answer.Wait()
	defer cancelAnswer()

	answer.Go(func() {
		defer cancelAnswer()
		err := answerAndConnect(answerCtx, pc, creds, caller, candidates, abort)
		if err != nil {
			abort <- err
			return
		}
	})

	var capture sync.WaitGroup
	captureCtx, cancelCapture := context.WithCancel(ctx)
	defer capture.Wait()
	defer cancelCapture()

	// setup microphone once call is connected and capture until cancelled
	capture.Go(func() {
		// todo: could do this in another goroutine and use its init chan
		if err := audioState.Mic.Init(); err != nil {
			abort <- err
			return
		}
		defer audioState.Mic.Uninit()

		select {
		case <-captureCtx.Done():
			return
		case <-connected:
			cancelAnswer()
			if err := audioState.Speaker.Start(); err != nil {
				abort <- err
			}
			break
		}

		if err := audioState.Mic.Start(captureCtx); err != nil {
			abort <- fmt.Errorf("error starting mic: %w", err)
			return
		}
	})

	// block until ctrl C or an error in capture goroutine
	select {
	case err := <-abort:
		return fmt.Errorf("call aborted: %w", err)
	case <-ctx.Done():
		return nil
	}
}

// answerAndConnect answers and establishes a voice call with a friend client. It
// uses a websocket connection to a vogo server to handle signaling and connecting.
// It uses trickle-ICE for fast connection. It assumes a PeerConnection set up
// correctly for opus audio.
func answerAndConnect(
	ctx context.Context,
	pc *webrtc.PeerConnection,
	credentials *credentials,
	caller string,
	candidates <-chan webrtc.ICECandidateInit,
	abort chan<- error,
) error {
	endpoint := fmt.Sprintf("/answer/%s", caller)
	ws, err := newWebsocket(ctx, credentials, endpoint)
	if err != nil {
		return fmt.Errorf("error creating websocket: %w", err)
	}

	offer, err := recvOffer(ctx, ws)
	if err != nil {
		return fmt.Errorf("error receiving offer: %w", err)
	}
	err = wrtc.CreateAnswer(pc, offer)
	if err != nil {
		return fmt.Errorf("error creating answer %w", err)
	}
	req := requests.Connection{To: caller, Sd: *pc.LocalDescription()}
	if err = websocket.JSON.Send(ws, req); err != nil {
		return fmt.Errorf("error sending answer: %w", err)
	}
	log.Println("answer sent")

	var sendIce sync.WaitGroup
	sendIceCtx, cancelSendIce := context.WithCancel(ctx)
	defer closeAndWait(ws, &sendIce)
	defer cancelSendIce()

	// gather local ice candidates and write to websocket
	sendIce.Go(func() {
		defer cancelSendIce()
		if err := sendCandidates(sendIceCtx, ws, candidates); err != nil {
			abort <- err // this will cause surrounding function to cancel
		}
	})

	// recv caller candidates
	readIceCtx, cancelReadIce := context.WithCancel(ctx)
	defer cancelReadIce()
	if err := recvCandidates(readIceCtx, ws, pc); err != nil {
		return err
	}

	// don't return now b/c sendICE could still be running.
	// wait for cancel from calling func on connected <-
	<-ctx.Done()
	return nil
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
