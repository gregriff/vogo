package netw

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/google/uuid"
	"github.com/gregriff/vogo/cli/internal/audio"
	"github.com/pion/webrtc/v4"
	"golang.org/x/net/websocket"
)

func JoinChannel(ctx context.Context, creds *credentials, ownerName, channelName string) error {
	// TODO:
	// - 1 peer connection per room user
	// - remote track automatically created once connection established? yes, per PC
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

	// todo: block here until at least one PC (and track) have been created
	// (with bulk message, no block needed). capture goroutine can then run,
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

// NOTE: the below are types shared with the server repo

type JoinChannelRequest struct {
	RoomName,
	OwnerName string
}

// BulkConnectionRequest is sent to the client when they need to start connecting with multiple
// users in a room. This happens when a client joins a room. Users may be empty if noone is in the room.
type BulkConnectionRequest struct {
	Users map[uuid.UUID]string
}

// BulkConnectionMessage is sent from the client when they are joining a room and
// have prepared offers for all users currently in the channel. Data maps the usernames
// to the offer being made to them.
type BulkConnectionMessage struct {
	Data map[uuid.UUID]ConnectionRequest
}

// ConnectionRequest encapsulates offers and answers. It can optionally contain
// the username of the sender and/or the recipient, depending on if additional context is needed.
type ConnectionRequest struct {
	From,
	To string
	Sd webrtc.SessionDescription
}

func joinChannelAndConnect(
	ctx context.Context,
	creds *credentials,
	ownerName, channelName string,
	abort chan<- error,
) error {
	ws, err := newWebsocket(ctx, creds, "/channel/join")
	if err != nil {
		return fmt.Errorf("error creating websocket: %w", err)
	}
	defer closeAndWait(ws, nil)

	req := JoinChannelRequest{RoomName: channelName, OwnerName: ownerName}
	if err = websocket.JSON.Send(ws, req); err != nil {
		return fmt.Errorf("error sending join channel request: %w", err)
	}

	// contains users to send offers to
	var res BulkConnectionRequest
	if err := receiveWithContext(ctx, ws, &res); err != nil {
		return fmt.Errorf("error reading channel users from ws: %v", err)
	}

	// create pcPool mapping userNames to peerConnections
	// conns := PcPool(res.Users)
	// conns.CreateOffers()
	// - at this point we'll have all the offers needed to send BulkMessage to server. send that
	// reference here:
	// offer, err := pc.CreateOffer(nil)
	// if err != nil {
	// 	return fmt.Errorf("error creating offer: %v", err)
	// }

	// FOR EACH PC:
	// // starts ICE gathering and UDP listeners
	// if err = pc.SetLocalDescription(offer); err != nil {
	// 	return fmt.Errorf("error setting local description: %v", err)
	// }

	// req := ConnectionRequest{To: recipient, Sd: offer}
	// bulkMessage.append(req)

	// TODO:
	// - create offers for every user in the response, create objs to store these (along with their PCs),
	//   then send server all of these offers at once. then...
	//
	// all the below needs to happen per room user returned in the bulkconnectionMessage,
	// and in their own goroutines

	// var sendIce sync.WaitGroup
	// sendIceCtx, cancelSendIce := context.WithCancel(ctx)
	// defer func() {
	// 	cancelSendIce()
	// 	sendIce.Wait()
	// }()

	// // gather local ice candidates and write to websocket
	// sendIce.Go(func() {
	// 	defer cancelSendIce()
	// 	if err = sendCandidates(sendIceCtx, ws, candidates); err != nil {
	// 		abort <- err // this will cause surrounding function to cancel
	// 	}
	// })

	// // wait to recv answer
	// var answer webrtc.SessionDescription
	// if err := receiveWithContext(ctx, ws, &answer); err != nil {
	// 	return fmt.Errorf("error reading answer from ws: %v", err)
	// }
	// if err = pc.SetRemoteDescription(answer); err != nil {
	// 	return fmt.Errorf("error while setting remote description: %w", err)
	// }
	// log.Println("recieved answer")

	// var (
	// 	readIce                   sync.WaitGroup
	// 	readIceCtx, cancelReadIce = context.WithCancel(ctx)
	// 	recipientCandidates       = make(chan webrtc.ICECandidateInit)
	// )
	// defer closeAndWait(ws, &readIce)
	// defer cancelReadIce()

	// TODO: this will need to be modified to mux the ICE candidates given their username. use messageLoop like in server...
	// // read recipient candidates from ws as they come in
	// readIce.Go(func() {
	// 	defer cancelReadIce()
	// 	err := readCandidates(readIceCtx, ws, recipientCandidates)
	// 	if err != nil {
	// 		abort <- fmt.Errorf("error during readICE: %w", err)
	// 	}
	// })
	// if err = addCandidates(ctx, pc, recipientCandidates); err != nil {
	// 	return err
	// }
	return nil
}
