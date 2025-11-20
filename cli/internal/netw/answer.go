// package netw implements high-level networking functionality to enable p2p voice chat.
// It handles the client-side connection process, using the wrtc package for signaling.
// In addition, CRUD operations with the vogo server are contained here.
// Many of the public functions in netw map directly to cli commands.
package netw

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/gen2brain/malgo"
	"github.com/gregriff/vogo/cli/internal/audio"
	"github.com/gregriff/vogo/cli/internal/netw/wrtc"
	"github.com/pion/webrtc/v4"
	"golang.org/x/net/websocket"
)

// Answer establishes a bidirectional voice call with a caller if a call is pending.
// Signaling, speaker init, connecting and microphone init are all run concurrently,
// organized with waitgroups and synchronized with channels. The entire process can
// be cancelled with the provided context, and the first error encountered will be returned.
func AnswerCall(ctx context.Context, credentials *credentials, caller string) error {
	pc, track, candidates, connected, err := wrtc.NewAudioPeerConnection(credentials.stunServer, credentials.username, true)
	if err != nil {
		return fmt.Errorf("error initializing webrtc: %w", err)
	}
	defer wrtc.ClosePC(pc, true)

	// sending an error on this channel will abort the call process
	abort := make(chan error, 10)

	// initalize speaker asynchronously
	var (
		playbackWg  sync.WaitGroup
		playbackCtx *malgo.AllocatedContext
		speaker     *malgo.Device
	)
	go func() {
		// TODO: mic capture needs to start after this is completed. add a noti chan.
		// also, find slowest part of speaker init with logging.
		// also, manually start mic once speaker is started. but let mic init async
		// also, manually start devices onPeerStateConnecting
		playbackCtx, speaker, err = audio.SetupPlayback(pc, &playbackWg)
		if err != nil {
			abort <- fmt.Errorf("error initializing playback system: %w", err)
			return
		}
		log.Println("playback device created")
	}()
	defer audio.UninitPlayback(pc, playbackCtx, speaker, &playbackWg)

	var answer sync.WaitGroup
	answerCtx, cancelAnswer := context.WithCancel(ctx)
	defer func() { // wait for capture device teardown
		cancelAnswer()
		answer.Wait()
		log.Println("answer wg completed")
	}()

	answer.Go(func() {
		defer cancelAnswer()

		err = answerAndConnect(answerCtx, pc, credentials, caller, candidates)
		if err != nil {
			abort <- err
			return
		}
	})

	var capture sync.WaitGroup
	captureCtx, cancelCapture := context.WithCancel(ctx)
	defer func() {
		cancelCapture()
		capture.Wait()
	}()

	// setup microphone once call is connected and capture until cancelled
	capture.Go(func() {
		select {
		case <-captureCtx.Done():
			return
		case <-connected:
			cancelAnswer()
			break
		}
		if err = audio.StartCapture(captureCtx, pc, track); err != nil {
			abort <- fmt.Errorf("error with capture device: %w", err)
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
) error {
	endpoint := fmt.Sprintf("/answer/%s", caller)
	ws, err := newWebsocket(ctx, credentials, endpoint)
	if err != nil {
		return fmt.Errorf("error creating websocket: %w", err)
	}

	offer, err := wrtc.RecieveOffer(ws)
	if err != nil {
		return fmt.Errorf("error recieving offer: %w", err)
	}
	err = wrtc.CreateAndSendAnswer(ws, pc, offer, caller)
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
