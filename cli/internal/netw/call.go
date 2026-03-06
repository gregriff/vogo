package netw

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/gregriff/vogo/cli/internal/audio"
	"github.com/gregriff/vogo/cli/internal/netw/wrtc"
	"github.com/gregriff/vogo/shared/requests"
	"github.com/gregriff/vogo/shared/wsock"
	"github.com/pion/webrtc/v4"
	"golang.org/x/net/websocket"
)

// CallFriend creates a bidirectional voice call to the intended recipient.
// Signaling, speaker init, connecting and microphone init are all run concurrently,
// organized with waitgroups and synchronized with channels. The entire process can
// be cancelled with the provided context, and the first error encountered will be returned.
func CallFriend(ctx context.Context, creds *credentials, recipient string) error {
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
	// go func() {
	// 	// TODO: mic capture needs to start after this is completed. add a noti chan.
	// 	// also, find slowest part of speaker init with logging.
	// 	// also, manually start mic once speaker is started. but let mic init async
	// 	// also, manually start devices onPeerStateConnecting
	// 	start := time.Now()
	// 	if err := audioState.InitPlayback(pc); err != nil {
	// 		abort <- fmt.Errorf("error initializing playback system: %w", err)
	// 		return
	// 	}
	// 	log.Printf("playback device created in %v", time.Since(start))
	// }()
	// defer audioState.UninitPlayback(pc)

	var call sync.WaitGroup
	callCtx, cancelCall := context.WithCancel(ctx)
	defer call.Wait()
	defer cancelCall()

	call.Go(func() {
		defer cancelCall()
		err := sendCallAndConnect(callCtx, pc, creds, recipient, candidates, abort)
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
			cancelCall()
			// if err := audioState.StartSpeaker(); err != nil {
			// 	abort <- err
			// }
			break
		}

		if err := audioState.Mic.Start(captureCtx); err != nil {
			abort <- fmt.Errorf("error starting mic: %w", err)
			return
		}
	})

	// block until sigint or error in goroutines above
	select {
	case err := <-abort:
		return fmt.Errorf("call aborted: %w", err)
	case <-ctx.Done():
		return nil
	}
}

// sendCallAndConnect creates and establishes a voice call with a friend client, if
// they answer the call. It uses a websocket connection to a vogo server to handle
// signaling and connecting, and uses trickle-ICE for fast connection. It assumes
// a PeerConnection set up correctly for opus audio.
func sendCallAndConnect(
	ctx context.Context,
	pc *webrtc.PeerConnection,
	creds *credentials,
	recipient string,
	candidates <-chan webrtc.ICECandidateInit,
	abort chan<- error,
) error {
	ws, err := newWebsocket(ctx, creds, "/call")
	if err != nil {
		return fmt.Errorf("error creating websocket: %w", err)
	}
	defer closeAndWait(ws, nil)

	offer := wrtc.CreateOffer(pc)

	// send offer
	req := requests.Connection{To: recipient, Sd: offer}
	if err = websocket.JSON.Send(ws, req); err != nil {
		return fmt.Errorf("error sending offer: %w", err)
	}

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

	// wait to recv answer
	var answer webrtc.SessionDescription
	if err := wsock.ReceiveJSON(ctx, ws, &answer); err != nil {
		return fmt.Errorf("error reading answer from ws: %v", err)
	}
	if err := pc.SetRemoteDescription(answer); err != nil {
		return fmt.Errorf("error while setting remote description: %w", err)
	}
	log.Println("received answer")

	// recv recipient candidates
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

// recvCandidates reads recipient candidates from ws and adds them to the pc.
func recvCandidates(ctx context.Context, ws *websocket.Conn, pc *webrtc.PeerConnection) error {
	var candidate webrtc.ICECandidateInit
	for {
		err := wsock.ReceiveJSON(ctx, ws, &candidate)
		if err != nil {
			return fmt.Errorf("error reading from ws: %w", err)
		}
		if candidate.Candidate == "" {
			log.Println("ice recv completed")
			return nil
		}
		log.Println("recv candidate")
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
