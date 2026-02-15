package netw

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/gregriff/vogo/cli/internal/audio"
)

func JoinChannel(ctx context.Context, creds *credentials, ownerName, channelName string) error {
	// TODO:
	// - 1 peer connection per room user
	// - programatically create ICECandidate channels, or don't use them at all (run each listener/forwarder in own goroutine)
	// - remote track automatically created once connection established?
	// - note: later, bulkConnectionRequest could be parallized, and the GUI could use recent status polling to
	//         issue offers ahead of time, cancelling them if joinRoom returns that the user is no longer in the room
	// - TODO: WILL NEED TO DECOUPLE AUDIO MIXER FROM REMOTE TRACKS! needs to communicate between PeerConnections
	// - TODO: need to decouple PC connection from TrackLocal creation, adding Track to each created PC
	//
	// pseudocode:
	// - call joinRoom endpoint
	// - on bulkConnectionMessage, per user, create PeerConnection, offer
	// - send bulkConnectionRequest
	// - start message loop, handle answers, ICE candidates
	// - make sure to send connection successful sentinels

	// pc, track, candidates, connected, err := wrtc.NewAudioPeerConnection(creds.stunServer, creds.username, true)
	// if err != nil {
	// 	return fmt.Errorf("error initializing webrtc: %v", err)
	// }
	// defer wrtc.ClosePC(pc, true)

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
	// defer audio.UninitPlayback(pc, playbackCtx, speaker, &playbackWg)

	var join sync.WaitGroup
	joinCtx, cancelJoin := context.WithCancel(ctx)
	defer func() {
		cancelJoin()
		join.Wait()
	}()

	join.Go(func() {
		defer cancelJoin()

		err := joinChannelAndConnect(joinCtx, creds, ownerName, channelName, abort)
		if err != nil {
			abort <- err
			return
		}
	})

	// todo: block here until at least one PC (and track) have been created. capture goroutine can then run,
	// and will wait until at least 1 PC is in connected state

	var capture sync.WaitGroup
	captureCtx, cancelCapture := context.WithCancel(ctx)
	defer func() {
		cancelCapture()
		capture.Wait()
	}()

	// NOTE: this cannot run until at least 1 PeerConnection (and therefore the Track) has been created.
	// setup microphone once call is connected and capture until cancelled
	capture.Go(func() {
		select {
		case <-captureCtx.Done():
			return
		case <-connected:
			cancelJoin()
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
