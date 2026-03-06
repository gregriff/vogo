package netw

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gregriff/vogo/cli/internal/audio"
	"github.com/gregriff/vogo/cli/internal/netw/wrtc"
	"github.com/gregriff/vogo/shared/requests"
	"github.com/gregriff/vogo/shared/wsock"
	"github.com/gregriff/vogo/shared/wsock/messages"
	"golang.org/x/net/websocket"
)

func JoinChannel(ctx context.Context, creds *credentials, ownerName, channelName string) error {
	// TODO:
	// - remote track automatically created once connection established? yes, per PC
	// - note: later, requests.BulkConnection could be parallized, and the GUI could use recent status polling to
	//         issue offers ahead of time, cancelling them if joinRoom returns that the user is no longer in the room
	//
	// pseudocode:
	// - make sure to send connection successful sentinels

	// sending an error on this channel will abort the call process
	abort := make(chan error, 10)

	track := wrtc.CreateAudioTrack(creds.username)
	audioState := audio.NewChannel(track)

	// initialize speaker asynchronously
	go func() {
		start := time.Now()
		if err := audioState.InitPlayback(); err != nil {
			abort <- fmt.Errorf("error initializing playback system: %w", err)
			return
		}
		log.Printf("playback device created in %v", time.Since(start))
	}()
	defer audioState.UninitPlayback()

	var join sync.WaitGroup
	joinCtx, cancelJoin := context.WithCancel(ctx)
	defer func() {
		cancelJoin()
		join.Wait()
	}()

	join.Go(func() {
		defer cancelJoin()

		// note: could create and return conns in main thread, then merge connected channels to prevent mic
		// init until ness.
		err := joinChannelAndConnect(joinCtx, creds, ownerName, channelName, audioState, abort)
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
		// todo: could do this in another goroutine and use its init chan
		if err := audioState.Mic.Init(); err != nil {
			abort <- err
			return
		}
		defer audioState.Mic.Uninit()

		select {
		case <-captureCtx.Done():
			return
		case <-audioState.Speaker.Initialized():
			if err := audioState.Speaker.Start(); err != nil {
				abort <- err
			}
			break
		}
		if err := audioState.Mic.Start(captureCtx); err != nil {
			abort <- fmt.Errorf("error starting mic: %w", err)
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

func joinChannelAndConnect(
	ctx context.Context,
	creds *credentials,
	ownerName, channelName string,
	audioState *audio.Channel,
	abort chan<- error,
) error {
	ws, err := newWebsocket(ctx, creds, "/channel/join")
	if err != nil {
		return fmt.Errorf("error creating websocket: %w", err)
	}
	defer closeAndWait(ws, nil)

	req := requests.JoinChannel{RoomName: channelName, OwnerName: ownerName}
	if err = websocket.JSON.Send(ws, req); err != nil {
		return fmt.Errorf("error sending join channel request: %w", err)
	}

	// contains users to send offers to
	var res requests.BulkConnection
	if err := wsock.ReceiveJSON(ctx, ws, &res); err != nil {
		return fmt.Errorf("error reading channel users from ws: %v", err)
	}

	// TODO: short circuit here, only sending blank bulkmsg and running msg loop
	if len(res.Users) == 0 {
		log.Println("no users in channel")
	}

	var iceWg sync.WaitGroup
	iceCtx, cancelIce := context.WithCancel(ctx)
	defer func() {
		cancelIce()
		iceWg.Wait()
	}()

	// this holds the state of the room from this client's perspective
	conns := wrtc.NewConnectionMap(ws, creds.stunServer, creds.username, iceCtx, &iceWg)
	defer conns.CloseAll()
	for id, name := range res.Users {
		c := wrtc.NewConnection(id, creds.stunServer, audioState.Mic.Track())
		conns.Update(name, c)
	}
	// todo: track, cleanup failed/expired connections

	// create all offers and send in bulk to server
	start := time.Now()
	bulkReq := messages.BulkConnection{Data: make(map[uuid.UUID]requests.Connection, 6)}
	for recipient, c := range conns.Snapshot() {
		req := c.NewOfferRequest(creds.username, recipient)
		bulkReq.Data[c.Id] = req
	}
	log.Printf("offers took %v to generate\n", time.Since(start))

	// TODO: can combine these two snapshot loops
	start = time.Now()
	for _, c := range conns.Snapshot() {
		c.Pc.OnTrack(audioState.OnRemoteTrack()) // set up audio mixing
	}
	log.Printf("remote handlers took %v to call\n", time.Since(start))

	if err = websocket.JSON.Send(ws, bulkReq); err != nil {
		return fmt.Errorf("error sending bulk offers: %w", err)
	}

	// send ice candidates to each user in the room
	for _, c := range conns.Snapshot() {
		c.SendCandidates(iceCtx, &iceWg, ws, creds.username, "ice-offer")
	}

	// TODO:
	// all the below needs to happen per room user returned in the messages.BulkConnection,
	// and in their own goroutines

	var (
		listen                  sync.WaitGroup
		listenCtx, cancelListen = context.WithCancel(ctx)
		msgs                    = make(chan wsock.Message)
	)
	defer func() {
		cancelListen()
		listen.Wait()
	}()
	listen.Go(func() {
		var err error
		if err = wsock.Listen(listenCtx, ws, msgs); err != nil {
			if err == io.EOF { // todo: may need to handle this in startMessageLoop
				err = errors.New("closed by server")
			}
			_ = ws.WriteClose(1)
		}
		abort <- fmt.Errorf("message loop closed: %w", err)
	})

	var handlerWg sync.WaitGroup
	defer func() {
		log.Println("waiting for handlers to finish")
		handlerWg.Wait()
	}()

	// this should run until the client leaves the room. it needs to handle
	// completing pending connections and also future connections when new users join the room.
	for {
		select {
		case <-ctx.Done():
			return nil
		case msg := <-msgs:
			handlerWg.Go(func() {
				handleMessage(msg, conns, audioState)
			})
		}
	}
}

// handleMessage dispatches handlers for each incoming message. It should be run in its
// own goroutine to prevent blocking the unbuffered message loop.
func handleMessage(
	msg wsock.Message,
	conns *wrtc.ConnectionMap,
	audioState *audio.Channel,
) {
	switch msg.Type {
	case "ice-offer", "ice-answer":
		handleIceMessage(msg, conns)
	case "answer":
		handleAnswerMessage(msg, conns)
	case "offer":
		handleOfferMessage(msg, conns, audioState)
	default:
		log.Printf("WARN: unknown message: %v", msg)
	}
}

// handleOfferMessage happens when the client is already in the room and a new user joins,
// sending the client their offer
func handleOfferMessage(
	msg wsock.Message,
	conns *wrtc.ConnectionMap,
	audioState *audio.Channel,
) {
	var (
		offer requests.ConnectionWithId
		conn  *wrtc.Connection
		ok    bool
	)
	if err := json.Unmarshal(msg.Data, &offer); err != nil {
		log.Panicf("error unmarshaling offer: %v", err)
	}
	if conn, ok = conns.Get(offer.From); ok {
		if err := conn.Pc.Close(); err != nil {
			log.Printf("error closing existing pc for %s: %v", offer.From, err)
		}
		log.Printf("recreating offer to %s", offer.From)
	}
	conn = wrtc.NewConnection(offer.FromId, conns.Server.StunServer, audioState.Mic.Track())
	conns.Update(offer.From, conn)
	log.Printf("received offer from %s, created conn", offer.From)

	// create and send answer
	_, err := wrtc.CreateAnswer(conn.Pc, &offer.Sd)
	if err != nil { // should prob retry
		log.Printf("error creating answer: %v", err)
		if cErr := conn.Pc.GracefulClose(); cErr != nil {
			log.Printf("error closing pc after answer creation error: %v", cErr)
		}
		return
	}

	answer := requests.Connection{From: offer.To, To: offer.From, Sd: *conn.Pc.LocalDescription()}
	bytes, err := json.Marshal(requests.ConnectionWithId{
		Connection: answer,
		FromId:     offer.ToId,
		ToId:       offer.FromId,
	})
	if err != nil {
		log.Panicf("error encoding answer: %v", err)
	}

	aMsg := wsock.Message{Type: "answer", Data: bytes}
	if err := websocket.JSON.Send(conns.Server.Ws, aMsg); err != nil {
		log.Printf("error sending answer: %v", err)
		if cErr := conn.Pc.GracefulClose(); cErr != nil {
			log.Printf("error closing pc after send answer error: %v", cErr)
		}
		return
	}
	log.Printf("sent answer (from %s) to %s to server", answer.From, answer.To)

	conn.SendCandidates(conns.IceCtx, conns.IceWg, conns.Server.Ws, conns.Server.Username, "ice-answer")
	conn.Pc.OnTrack(audioState.OnRemoteTrack())
}

// handleAnswerMessage handles when the client receives an answer from a recipient
func handleAnswerMessage(msg wsock.Message, conns *wrtc.ConnectionMap) {
	var (
		answer requests.Connection
		conn   *wrtc.Connection
		ok     bool
	)
	if err := json.Unmarshal(msg.Data, &answer); err != nil {
		log.Panicf("error unmarshaling answer: %v", err)
	}
	log.Printf("received answer from ws: from:%s to: %s\n", answer.From, answer.To)
	if conn, ok = conns.Get(answer.To); !ok {
		log.Panicf("error: connection for user %s not found", answer.From)
	}
	if err := conn.Pc.SetRemoteDescription(answer.Sd); err != nil {
		log.Printf("error while setting remote description: %v", err)
		if cErr := conn.Pc.GracefulClose(); cErr != nil {
			log.Printf("error closing pc after remote description error: %v", cErr)
		}
		return
	}
	log.Printf("received answer from %s", answer.From)
	return
}

func handleIceMessage(msg wsock.Message, conns *wrtc.ConnectionMap) {
	var data messages.Candidate
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		log.Panicf("error unmarshaling %s candidate: %v", msg.Type, err)
	}

	var (
		conn *wrtc.Connection
		ok   bool
	)
	if conn, ok = conns.Get(data.Username); !ok {
		log.Panicf("error: connection for user %s not found", data.Username)
	}

	if err := conn.Pc.AddICECandidate(data.Candidate); err != nil {
		log.Printf("error adding ICE candidate: %v", err)
	}
	if data.Candidate.Candidate == "" {
		log.Printf("%s NIL recv", msg.Type)
	} else {
		log.Printf("%s candidate received: %s\n", msg.Type, *data.Candidate.SDPMid)
	}
}

// func mergeConnected(ctx context.Context, channels ...<-chan struct{}) <-chan struct{} {
// 	merged := make(chan struct{}, 1)
// 	for _, ch := range channels {
// 		go func(c <-chan struct{}) {
// 			select {
// 			case <-ctx.Done():
// 			case _, ok := <-c:
// 				if ok {
// 					select {
// 					case merged <- struct{}{}:
// 					default: // already notified, drop
// 					}
// 				}
// 			}
// 		}(ch)
// 	}
// 	return merged
// }
