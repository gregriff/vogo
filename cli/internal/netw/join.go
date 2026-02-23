package netw

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gregriff/vogo/cli/internal/audio"
	"github.com/gregriff/vogo/cli/internal/netw/wrtc"
	"github.com/pion/webrtc/v4"
	"golang.org/x/net/websocket"
)

func JoinChannel(ctx context.Context, creds *credentials, ownerName, channelName string) error {
	// TODO:
	// - remote track automatically created once connection established? yes, per PC
	// - note: later, bulkConnectionRequest could be parallized, and the GUI could use recent status polling to
	//         issue offers ahead of time, cancelling them if joinRoom returns that the user is no longer in the room
	// - TODO: WILL NEED TO DECOUPLE AUDIO MIXER FROM REMOTE TRACKS! needs to communicate between PeerConnections
	//
	// pseudocode:
	// - make sure to send connection successful sentinels

	// pc, track, candidates, connected, err := wrtc.NewAudioPeerConnection(creds.stunServer, creds.username, true)
	// if err != nil {
	// 	return fmt.Errorf("error initializing webrtc: %v", err)
	// }
	// defer wrtc.ClosePC(pc, true)

	track, err := wrtc.CreateAudioTrack(creds.username)
	if err != nil {
		return err
	}

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

		// note: could create and return conns in main thread, then merge connected channels to prevent mic
		// init until ness.
		err := joinChannelAndConnect(joinCtx, creds, track, ownerName, channelName, abort)
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
		// select {
		// case <-captureCtx.Done():
		// return
		// case <-connected:
		// 	// TODO: this should not cancel the entire join goroutine. we do need to merge all connected channels into one,
		// 	// but a noti on it should not cancel the join process for all pending connections, which is what this would do.
		// 	cancelJoin()
		// 	break
		// }
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
	Data map[uuid.UUID]wrtc.ConnectionRequest
}

// Connection encapsulates a bidirectional audio webrtc connection.
type Connection struct {
	// the uuid of the recipient user.
	id uuid.UUID

	callerName string

	// webrtc PeerConnection
	pc *webrtc.PeerConnection

	// track which audio is written to
	track *webrtc.TrackLocalStaticSample

	// client's offer.
	offer *webrtc.SessionDescription

	// channel for sending ICE candidates
	candidates chan webrtc.ICECandidateInit

	// notification channel for connection status
	connected chan struct{}
}

func newConnection(id uuid.UUID, creds *credentials, track *webrtc.TrackLocalStaticSample) (*Connection, error) {
	pc, track, candidates, connected, err := wrtc.NewAudioPeerConnection(creds.stunServer, track, false)
	if err != nil {
		return &Connection{}, fmt.Errorf("error initializing webrtc: %w", err)
	}
	c := Connection{
		id:         id,
		callerName: creds.username,
		pc:         pc,
		track:      track,
		candidates: candidates,
		connected:  connected,
	}
	return &c, nil
}

// ConnectionMap maps recipient usernames to Connections.
type ConnectionMap struct {
	mu   sync.Mutex
	data map[string]*Connection
}

func (cm *ConnectionMap) Get(username string) (*Connection, bool) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	c, ok := cm.data[username]
	return c, ok
}

// TODO: make updater
func (cm *ConnectionMap) Update(key string, c *Connection) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.data[key] = c
}

func joinChannelAndConnect(
	ctx context.Context,
	creds *credentials,
	track *webrtc.TrackLocalStaticSample,
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

	conns := &ConnectionMap{
		data: make(map[string]*Connection, 6),
	}
	for id, name := range res.Users {
		c, err := newConnection(id, creds, track)
		defer wrtc.ClosePC(c.pc, true)
		if err != nil {
			return fmt.Errorf("error creating connection for %s: %w", name, err)
		}
		conns.data[name] = c
	}
	// todo: track, cleanup failed/expired connections

	// create all offers and send in bulk to server
	start := time.Now()
	bulkReq := BulkConnectionMessage{Data: make(map[uuid.UUID]wrtc.ConnectionRequest, 6)}
	for recipient, c := range conns.data {
		offer, err := wrtc.CreateOffer(c.pc)
		if err != nil {
			return fmt.Errorf("error creating offer for %s: %w", recipient, err)
		}
		bulkReq.Data[c.id] = wrtc.ConnectionRequest{From: creds.username, To: recipient, Sd: offer}
	}
	fmt.Printf("offers took %v to generate", time.Since(start))

	if err = websocket.JSON.Send(ws, bulkReq); err != nil {
		return fmt.Errorf("error sending bulk offers: %w", err)
	}

	var sendIce sync.WaitGroup
	sendIceCtx, cancelSendIce := context.WithCancel(ctx)
	defer func() {
		cancelSendIce()
		sendIce.Wait()
	}()

	for _, c := range conns.data {
		// gather local ice candidates for each peer and write to websocket
		sendIce.Go(func() {
			defer cancelSendIce()
			if err = sendTaggedCandidates(sendIceCtx, ws, c); err != nil {
				abort <- err // this will cause surrounding function to cancel
				// TODO: this should not stop the entire join process. should fail gracefully and retry
			}
		})
	}

	// TODO:
	// all the below needs to happen per room user returned in the bulkconnectionMessage,
	// and in their own goroutines

	var (
		wsRecvWg                sync.WaitGroup
		wsRecvCtx, cancelWsRecv = context.WithCancel(ctx)
		msgChan                 = make(chan Message)
	)
	defer func() {
		cancelWsRecv()
		wsRecvWg.Wait()
	}()
	wsRecvWg.Go(func() {
		if err := startMessageLoop(wsRecvCtx, ws, msgChan); err != nil {
			if err == io.EOF { // todo: may need to handle this in startMessageLoop
				log.Println("messageLoop EOF")
			} else {
				log.Printf("messageLoop other err: %v", err)
			}
			_ = ws.WriteClose(1)
		}
		abort <- fmt.Errorf("message loop error: %w", err)
	})

	// this should run until the client leaves the room. it needs to handle
	// completing pending connections and also future connections when new users join the room.
	for {
		select {
		case <-ctx.Done():
			return nil
		// NOTE: all funcs in this need to run async, since chan is unbuffered and blocks on sends
		case msg := <-msgChan:
			// TODO: put this switch into its own func. run it in its own goroutine. it should use a waitgroup defined before this
			// event loop "msgHandlerWg". 'tick' will report the wg's counter (manually increment a top level int).
			switch msg.Type {
			// TODO: try to combine offer and answer handlers with additional property in CandidateMessage
			case "ice-answer":
				var (
					data CandidateMessage
					conn *Connection
					ok   bool
				)
				json.Unmarshal(msg.Data, &data)
				if conn, ok = conns.Get(data.Username); !ok {
					return fmt.Errorf("error: connection for user %s not found", data.Username)
				}

				if err := conn.pc.AddICECandidate(data.Candidate); err != nil {
					return fmt.Errorf("error recieving ICE candidate: %w", err)
				}
			// when the client receives an answer from a recipient
			case "answer":
				var (
					answer wrtc.ConnectionRequest
					conn   *Connection
					ok     bool
				)
				json.Unmarshal(msg.Data, &answer)
				if conn, ok = conns.Get(answer.From); !ok {
					return fmt.Errorf("error: connection for user %s not found", answer.From)
				}
				if err = conn.pc.SetRemoteDescription(answer.Sd); err != nil {
					return fmt.Errorf("error while setting remote description: %w", err)
				}
				log.Println("answer handler: answer recieved")
			// this happens when the client is already in the room and a new user joins,
			// sending the client their offer
			case "offer":
				var (
					offer wrtc.ConnectionRequestWithId
					conn  *Connection
					ok    bool
				)
				json.Unmarshal(msg.Data, &offer)
				if conn, ok = conns.Get(offer.From); ok {
					log.Printf("recreating offer to %s", offer.From)
				}
				conn, err := newConnection(offer.FromId, creds, track)
				if err != nil {
					return fmt.Errorf("error creating connection for %s: %w", offer.From, err)
				}
				conns.Update(offer.From, conn)

				err = wrtc.CreateAndSendAnswer(ws, conn.pc, &offer.Sd, offer.From)
				if err != nil {
					return fmt.Errorf("error creating or posting answer %w", err)
				}

				var sendIce sync.WaitGroup
				sendIceCtx, cancelSendIce := context.WithCancel(ctx)
				defer func() {
					cancelSendIce()
					sendIce.Wait()
				}()

				// gather local ice candidates for each peer and write to websocket
				sendIce.Go(func() {
					defer cancelSendIce()
					if err = sendTaggedCandidates(sendIceCtx, ws, conn); err != nil {
						abort <- err // this will cause surrounding function to cancel
						// TODO: this should not stop the entire join process. should fail gracefully and retry
					}
				})
			}
		}
	}
}

type Message struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// startMessageLoop reads from ws until it is closed or errors, and sends the data it reads to ch.
// It assumes ws frames are structured according to the Message struct. It enables the caller to react
// to ws frames in an event-based manner.
func startMessageLoop(ctx context.Context, ws *websocket.Conn, ch chan<- Message) error {
	for {
		var msg Message
		if err := receiveWithContext(ctx, ws, &msg); err != nil {
			log.Printf("error reading answer from ws: %v", err)
			return err
		}
		ch <- msg
	}
}

type CandidateMessage struct {
	UserId    uuid.UUID
	Username  string
	Candidate webrtc.ICECandidateInit
}

// sendTaggedCandidates sends the caller's ICE candidates from ch to the websocket as they're gathered.
// It sends the caller's name along with the candidate. It returns when there are no more
// candidates or the context is cancelled.
func sendTaggedCandidates(ctx context.Context, ws *websocket.Conn, conn *Connection) error {
	defer log.Println("ice gathering completed")
	for {
		select {
		case <-ctx.Done():
			return nil
		case candidate, ok := <-conn.candidates:
			bytes, err := json.Marshal(CandidateMessage{
				UserId:    conn.id,
				Username:  conn.callerName,
				Candidate: candidate,
			})
			if err != nil {
				return fmt.Errorf("error encoding candidate: %w", err)
			}

			msg := Message{Type: "ice-candidate", Data: bytes}
			if err := websocket.JSON.Send(ws, msg); err != nil {
				return fmt.Errorf("error sending candidate: %w", err)
			}
			if !ok {
				return nil
			}
		}
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
