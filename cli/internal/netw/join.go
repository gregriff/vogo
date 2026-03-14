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
	"github.com/gregriff/vogo/shared"
	"github.com/gregriff/vogo/shared/requests"
	"github.com/gregriff/vogo/shared/wsock"
	"github.com/gregriff/vogo/shared/wsock/messages"
	"golang.org/x/net/websocket"
	"golang.org/x/sync/errgroup"
)

func JoinChannel(ctx context.Context, creds *credentials, ownerName, channelName string) error {
	// TODO:
	// - note: later, requests.BulkConnection could be parallized, and the GUI could use recent status polling to
	//         issue offers ahead of time, cancelling them if joinRoom returns that the user is no longer in the room
	// pseudocode:
	// - make sure to send connection successful sentinels

	track := wrtc.CreateAudioTrack(creds.username)
	audioState := audio.NewChannel(track, wrtc.RecvMTU)

	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		start := time.Now()
		defer func() {
			log.Printf("playback device created in %v", time.Since(start))
		}()
		return audioState.Speaker.Init(audioState.DataProc())
	})
	defer func() {
		_ = audioState.Speaker.Uninit()
	}()

	g.Go(func() error {
		return joinChannelAndConnect(gCtx, creds, ownerName, channelName, audioState)
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
		case <-audioState.Speaker.Initialized():
			// for testing, disable speaker for test user two
			// NOTE: if two speakers are playing on the same machine, audio will sound bad
			// NOTE: if Speaker.Start() is not called, memory leaks occur.
			// if user := os.Getenv("VOGOENV"); user == "two" {
			// 	break
			// }
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

func joinChannelAndConnect(
	ctx context.Context,
	creds *credentials,
	ownerName, channelName string,
	audioState *audio.Channel,
) error {
	ws, err := newWebsocket(ctx, creds, "/channel/join")
	if err != nil {
		return fmt.Errorf("error creating websocket: %w", err)
	}
	defer func() {
		_ = closeAndWait(ws, nil)
	}()

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
	defer iceWg.Wait()

	var statusWg sync.WaitGroup
	defer statusWg.Wait()

	// this holds the state of the room from this client's perspective
	conns := wrtc.NewConnectionMap(ws, creds.stunServer, creds.username, &iceWg)
	defer conns.CloseAll()
	for id, name := range res.Users {
		log.Printf("creating new connection with %s", name)
		c := wrtc.NewConnection(id, name, creds.stunServer, audioState.Mic.Track(), false)
		conns.Update(name, c)
		statusWg.Go(func() {
			_ = c.HandleEvents(ctx, name)
		})
	}
	// todo: track, cleanup failed/expired connections

	// create all offers and send in bulk to server
	start := time.Now()
	bulkReq := messages.BulkConnection{Data: make(map[uuid.UUID]requests.Connection, shared.ChannelCapacity-1)}
	for recipient, c := range conns.Snapshot() {
		req := c.NewOffer(creds.username, recipient)
		bulkReq.Data[c.Id] = req
		audioState.AddPeer(c.Pc) // set up audio mixing
	}
	log.Printf("offers took %v to generate\n", time.Since(start))

	if err = websocket.JSON.Send(ws, bulkReq); err != nil {
		return fmt.Errorf("error sending bulk offers: %w", err)
	}

	// send ice candidates to each user in the room
	for _, c := range conns.Snapshot() {
		iceWg.Go(func() {
			defer log.Println("ice-offer sending done")
			log.Println("sending ice-offers now")
			c.SendCandidates(ctx, ws, creds.username, "ice-offer")
		})
	}

	g, gCtx := errgroup.WithContext(ctx)
	defer func() {
		if err := closeAndWait(ws, g); err != nil {
			log.Printf("error: %v", err)
		}
	}()

	msgs := make(chan wsock.Message)
	g.Go(func() error {
		var err error
		if err = wsock.Listen(gCtx, ws, msgs); err != nil {
			if err == io.EOF { // todo: may need to handle this in startMessageLoop
				err = errors.New("closed by server")
			}
			_ = ws.WriteClose(1)
		}
		return err
	})

	var handlerWg sync.WaitGroup
	defer handlerWg.Wait()

	// this should run until the client leaves the room. it needs to handle
	// completing pending connections and also future connections when new users join the room.
	for {
		select {
		case <-ctx.Done():
			return g.Wait()
		case msg := <-msgs:
			handlerWg.Go(func() {
				handleMessage(ctx, msg, conns, audioState)
			})
		}
	}
}

// handleMessage dispatches handlers for each incoming message. It should be run in its
// own goroutine to prevent blocking the unbuffered message loop.
func handleMessage(
	ctx context.Context,
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
		handleOfferMessage(ctx, msg, conns, audioState)
	default:
		log.Printf("WARN: unknown message: %v", msg)
	}
}

// handleOfferMessage happens when the client is already in the room and a new user joins,
// sending the client their offer.
func handleOfferMessage(
	ctx context.Context,
	msg wsock.Message,
	conns *wrtc.ConnectionMap,
	audioState *audio.Channel,
) {
	var offer requests.ConnectionWithId
	if err := json.Unmarshal(msg.Data, &offer); err != nil {
		log.Panicf("error unmarshaling offer: %v", err)
	}

	var conn *wrtc.Connection
	if conn = conns.Get(offer.From); conn != nil {
		conn.Close()
		// if err := conn.Pc.Close(); err != nil {
		// 	log.Printf("error closing existing pc for %s: %v", offer.From, err)
		// }
		log.Printf("recreating offer to %s", offer.From)
	}
	if conns.Len() >= audio.MaxStreams {
		// TODO: send err on channel for UI to pick up.
		log.Printf("could not accept offer from %s: already have max audio streams. num conns=%d", offer.From, conns.Len())
		return
	}
	conn = wrtc.NewConnection(offer.FromId, offer.From, conns.Server.StunServer, audioState.Mic.Track(), false)
	audioState.AddPeer(conn.Pc)
	conns.Update(offer.From, conn)
	log.Printf("received offer from %s, created conn", offer.From)

	// create and send answer. TODO: are retries automatic?
	err := conn.CreateAnswer(&offer.Sd)
	if err != nil { // should prob retry
		log.Printf("error creating answer: %v", err)
		conn.Close()
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
		conn.Close()
		return
	}
	log.Printf("sent answer (from %s) to %s to server", answer.From, answer.To)

	conns.IceWg.Go(func() {
		defer log.Println("ice-answer sending done")
		log.Println("sending ice-answers now")
		conn.SendCandidates(ctx, conns.Server.Ws, conns.Server.Username, "ice-answer")
	})
}

// handleAnswerMessage handles when the client receives an answer from a recipient.
func handleAnswerMessage(msg wsock.Message, conns *wrtc.ConnectionMap) {
	var answer requests.Connection
	if err := json.Unmarshal(msg.Data, &answer); err != nil {
		log.Panicf("error unmarshaling answer: %v", err)
	}
	log.Printf("received answer from ws: from:%s to: %s\n", answer.From, answer.To)

	var conn *wrtc.Connection
	if conn = conns.Get(answer.To); conn == nil {
		log.Panicf("error: connection for user %s not found", answer.From)
	}
	if err := conn.Pc.SetRemoteDescription(answer.Sd); err != nil {
		log.Printf("error while setting remote description: %v", err)
		conn.Close()
		return
	}
	conn.Once.Do(func() { close(conn.RemoteSet) })
	log.Printf("received answer from %s", answer.From)
}

func handleIceMessage(msg wsock.Message, conns *wrtc.ConnectionMap) {
	var data messages.Candidate
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		log.Panicf("error unmarshaling %s candidate: %v", msg.Type, err)
	}

	var conn *wrtc.Connection
	if conn = conns.Get(data.Username); conn == nil {
		log.Panicf("error: connection for user %s not found", data.Username)
	}

	// wait for remote description to be set
	<-conn.RemoteSet

	// todo: ensure remote description has been set before this runs
	if err := conn.Pc.AddICECandidate(data.Candidate); err != nil {
		log.Printf("error adding ICE candidate: %v", err)
	}
	if data.Candidate.Candidate == "" {
		log.Printf("%s NIL recv", msg.Type)
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
