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
	track, err := wrtc.CreateAudioTrack(creds.username)
	if err != nil {
		return err
	}
	pc, track, candidates, connected, err := wrtc.NewAudioPeerConnection(creds.stunServer, track, true)
	if err != nil {
		return fmt.Errorf("error initializing webrtc: %v", err)
	}
	defer wrtc.ClosePC(pc, true)

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

	// initalize speaker asynchronously
	// var (
	// 	playbackWg  sync.WaitGroup
	// 	playbackCtx *malgo.AllocatedContext
	// 	speaker     *malgo.Device
	// )
	// go func() {
	// 	// TODO: mic capture needs to start after this is completed. add a noti chan
	// 	playbackCtx, speaker, err = audio.SetupPlayback(pc, &playbackWg)
	// 	if err != nil {
	// 		abort <- fmt.Errorf("error initializing playback system: %w", err)
	// 		return
	// 	}
	// 	log.Println("playback device created")
	// }()
	// defer audio.UninitCallPlayback(pc, playbackCtx, speaker, &playbackWg)

	var call sync.WaitGroup
	callCtx, cancelCall := context.WithCancel(ctx)
	defer func() {
		cancelCall()
		call.Wait()
	}()

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
			cancelCall()
			break
		}
		if err := audio.StartCapture(captureCtx, track); err != nil {
			abort <- fmt.Errorf("error with capture device: %w", err)
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

	offer, err := wrtc.CreateOffer(pc)
	if err != nil {
		return err
	}

	// send offer
	req := requests.Connection{To: recipient, Sd: offer}
	if err = websocket.JSON.Send(ws, req); err != nil {
		return fmt.Errorf("error sending offer: %w", err)
	}

	var sendIce sync.WaitGroup
	sendIceCtx, cancelSendIce := context.WithCancel(ctx)
	defer func() {
		cancelSendIce()
		sendIce.Wait()
	}()

	// gather local ice candidates and write to websocket
	sendIce.Go(func() {
		defer cancelSendIce()
		if err = sendCandidates(sendIceCtx, ws, candidates); err != nil {
			abort <- err // this will cause surrounding function to cancel
		}
	})

	// wait to recv answer
	var answer webrtc.SessionDescription
	if err := wsock.ReceiveJSON(ctx, ws, &answer); err != nil {
		return fmt.Errorf("error reading answer from ws: %v", err)
	}
	if err = pc.SetRemoteDescription(answer); err != nil {
		return fmt.Errorf("error while setting remote description: %w", err)
	}
	log.Println("recieved answer")

	var (
		readIce                   sync.WaitGroup
		readIceCtx, cancelReadIce = context.WithCancel(ctx)
		recipientCandidates       = make(chan webrtc.ICECandidateInit)
	)
	defer closeAndWait(ws, &readIce)
	defer cancelReadIce()

	// read recipient candidates from ws as they come in
	readIce.Go(func() {
		defer cancelReadIce()
		err := readCandidates(readIceCtx, ws, recipientCandidates)
		if err != nil {
			abort <- fmt.Errorf("error during readICE: %w", err)
		}
	})
	// todo: this could be done in readIce goroutine if you pass in the pc...
	if err = addCandidates(ctx, pc, recipientCandidates); err != nil {
		return err
	}
	return nil
}

// sendCandidates sends the caller's ICE candidates from ch to the websocket as they're gathered.
// It returns when there are no more candidates or the context is cancelled.
func sendCandidates(ctx context.Context, ws *websocket.Conn, ch <-chan webrtc.ICECandidateInit) error {
	defer log.Println("ice gathering completed")
	for {
		select {
		case <-ctx.Done():
			return nil
		case candidate, ok := <-ch:
			if err := websocket.JSON.Send(ws, candidate); err != nil {
				return fmt.Errorf("error sending ice candidate: %w", err)
			}
			// log.Println("sent candidate")
			if !ok {
				return nil
			}
		}
	}
}

// addCandidates adds the recipient's ICE candidates from ch to the peer connection. This function will continue
// until its context is cancelled even once all candidates are exhausted.
// TODO: i think this could be done in the readIce goroutine
func addCandidates(ctx context.Context, pc *webrtc.PeerConnection, ch <-chan webrtc.ICECandidateInit) error {
	for {
		select {
		case <-ctx.Done():
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
