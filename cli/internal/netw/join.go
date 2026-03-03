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

	"github.com/gen2brain/malgo"
	"github.com/google/uuid"
	"github.com/gregriff/vogo/cli/internal/audio"
	"github.com/gregriff/vogo/cli/internal/netw/wrtc"
	"github.com/gregriff/vogo/shared/requests"
	"github.com/gregriff/vogo/shared/wsock"
	"github.com/gregriff/vogo/shared/wsock/messages"
	"github.com/pion/webrtc/v4"
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

	// initialize speaker asynchronously
	var (
		playbackWg       sync.WaitGroup
		playbackCtx      *malgo.AllocatedContext
		speaker          *malgo.Device
		pcmBufs          = audio.NewChannelStreams()
		playbackInitDone = make(chan struct{})
	)
	go func() {
		// TODO: mic capture needs to start after this is completed. add a noti chan
		playbackCtx, speaker, err = audio.SetupPlaybackChannel(pcmBufs)
		if err != nil {
			abort <- fmt.Errorf("error initializing playback system: %w", err)
			return
		}
		log.Println("playback device created")
		playbackInitDone <- struct{}{}
	}()
	defer func() {
		// waits for all PeerConnections to close, so that nothing is writing to the
		// speaker devices when they are uninitialized
		playbackWg.Wait()
		audio.UninitPlayback(playbackCtx, speaker)
	}()

	var join sync.WaitGroup
	joinCtx, cancelJoin := context.WithCancel(ctx)
	defer func() {
		cancelJoin()
		join.Wait()
	}()

	join.Go(func() {
		defer cancelJoin()
		defer close(abort)

		// note: could create and return conns in main thread, then merge connected channels to prevent mic
		// init until ness.
		err := joinChannelAndConnect(joinCtx, creds, track, ownerName, channelName, &playbackWg, pcmBufs, abort)
		if err != nil {
			abort <- err
			return
		}
	})

	// todo: block here until at least one PC (and track) have been created
	// (with bulk message, no block needed). capture goroutine can then run,
	// and will wait until at least 1 PC is in connected state

	var capture sync.WaitGroup
	captureAbort := make(chan error, 10)
	captureCtx, cancelCapture := context.WithCancel(ctx)
	defer func() {
		cancelCapture()
		capture.Wait()
	}()

	// NOTE: this cannot run until at least 1 PeerConnection (and therefore the Track) has been created.
	// setup microphone once call is connected and capture until cancelled
	capture.Go(func() {
		defer close(captureAbort)
		select {
		case <-captureCtx.Done():
			return
		case <-playbackInitDone:
			break
		}
		if err := audio.StartCapture(captureCtx, track); err != nil {
			log.Println("error in startCapture")
			captureAbort <- fmt.Errorf("error with capture device: %w", err)
		}
	})

	// block until sigint or error in goroutines above
	select {
	case err := <-abort:
		return fmt.Errorf("call aborted: %w", err)
	case err := <-captureAbort:
		return fmt.Errorf("call aborted: %w", err)
	case <-ctx.Done():
		return nil
	}
}

// NOTE: the below are types shared with the server repo

// Connection encapsulates a bidirectional audio webrtc connection.
type Connection struct {
	// the uuid of the recipient user.
	id uuid.UUID

	// webrtc PeerConnection
	pc *webrtc.PeerConnection

	// track which audio is written to
	track *webrtc.TrackLocalStaticSample

	// client's offer.
	// offer *webrtc.SessionDescription

	// channel for sending ICE candidates
	candidates chan webrtc.ICECandidateInit

	// notification channel for connection status
	connected chan struct{}
}

func newConnection(id uuid.UUID, stunServer string, track *webrtc.TrackLocalStaticSample) (*Connection, error) {
	pc, track, candidates, connected, err := wrtc.NewAudioPeerConnection(stunServer, track, false)
	if err != nil {
		return &Connection{}, fmt.Errorf("error initializing webrtc: %w", err)
	}
	c := Connection{
		id:         id,
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

// TODO: make updater.
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
	playbackWg *sync.WaitGroup,
	pcmBufs *audio.ChannelStreams,
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

	conns := &ConnectionMap{
		data: make(map[string]*Connection, 6),
	}
	for id, name := range res.Users {
		c, err := newConnection(id, creds.stunServer, track)
		defer func() {
			// users may leave, and their pc's destroyed before this runs
			var ok bool
			if c, ok = conns.Get(name); ok {
				wrtc.ClosePC(c.pc, true)
			}
		}()
		if err != nil {
			return fmt.Errorf("error creating connection for %s: %w", name, err)
		}
		conns.data[name] = c
	}
	// todo: track, cleanup failed/expired connections

	// create all offers and send in bulk to server
	start := time.Now()
	bulkReq := messages.BulkConnection{Data: make(map[uuid.UUID]requests.Connection, 6)}
	for recipient, c := range conns.data {
		if recipient == "" {
			return errors.New("empty recipient")
		}
		// TODO: start playback stuff here?
		offer, err := wrtc.CreateOffer(c.pc)
		if err != nil {
			return fmt.Errorf("error creating offer for %s: %w", recipient, err)
		}
		bulkReq.Data[c.id] = requests.Connection{From: creds.username, To: recipient, Sd: offer}
		log.Printf("%s will send offer to %s...\n", creds.username, recipient)
	}
	fmt.Printf("offers took %v to generate\n", time.Since(start))

	start = time.Now()
	for recipient, c := range conns.data {
		c.pc.OnTrack(audio.OnRemoteTrack(playbackWg, pcmBufs))
		defer func() {
			// users may leave, and their pc's destroyed before this runs
			var ok bool
			if c, ok = conns.Get(recipient); !ok {
				return
			}
			if err := c.pc.GracefulClose(); err != nil {
				log.Printf("graceful close error: %v", err)
			}
			log.Printf("%s's pc gracefully closed", recipient)
		}()
	}
	fmt.Printf("remote handlers took %v to call\n", time.Since(start))

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
			defer func() {
				cancelSendIce()
				log.Println("ice-offer sending done")
			}()
			log.Println("sending ice-offers now")
			if err = sendTaggedCandidates(sendIceCtx, ws, c, creds.username, "ice-offer"); err != nil {
				log.Printf("SEND TAGGED CANDIDATES ERR: %v\n", err)
				abort <- err // this will cause surrounding function to cancel
				// TODO: this should not stop the entire join process. should fail gracefully and retry
			}
		})
	}

	// TODO:
	// all the below needs to happen per room user returned in the messages.BulkConnection,
	// and in their own goroutines

	var (
		wsRecvWg                sync.WaitGroup
		wsRecvCtx, cancelWsRecv = context.WithCancel(ctx)
		msgChan                 = make(chan wsock.Message)
	)
	defer func() {
		cancelWsRecv()
		wsRecvWg.Wait()
	}()
	wsRecvWg.Go(func() {
		var err error
		if err = wsock.Listen(wsRecvCtx, ws, msgChan); err != nil {
			if err == io.EOF { // todo: may need to handle this in startMessageLoop
				err = errors.New("closed by server")
			}
			_ = ws.WriteClose(1)
		}
		abort <- fmt.Errorf("message loop closed: %w", err)
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
			// TODO: try to combine offer and answer handlers with additional property in messages.Candidate
			case "ice-offer":
				var (
					data messages.Candidate
					conn *Connection
					ok   bool
				)
				if err := json.Unmarshal(msg.Data, &data); err != nil {
					return fmt.Errorf("error unmarshaling ice-offer candidate: %w", err)
				}
				if conn, ok = conns.Get(data.Username); !ok {
					return fmt.Errorf("error: connection for user %s not found", data.Username)
				}

				if err := conn.pc.AddICECandidate(data.Candidate); err != nil {
					return fmt.Errorf("error receiving ICE candidate: %w", err)
				}
				if data.Candidate.Candidate == "" {
					log.Printf("ICE offer NIL recv")
				} else {
					log.Printf("ICE offer candidate received: %s\n", *data.Candidate.SDPMid)
				}
			case "ice-answer":
				var (
					data messages.Candidate
					conn *Connection
					ok   bool
				)
				if err := json.Unmarshal(msg.Data, &data); err != nil {
					return fmt.Errorf("error unmarshaling ice-answer candidate: %w", err)
				}
				if conn, ok = conns.Get(data.Username); !ok {
					return fmt.Errorf("error: connection for user %s not found", data.Username)
				}

				if err := conn.pc.AddICECandidate(data.Candidate); err != nil {
					return fmt.Errorf("error receiving ICE candidate: %w", err)
				}
				if data.Candidate.Candidate == "" {
					log.Printf("ICE answer NIL recv")
				} else {
					log.Printf("ICE answer candidate received: %s\n", *data.Candidate.SDPMid)
				}
			// when the client receives an answer from a recipient
			case "answer":
				var (
					answer requests.Connection
					conn   *Connection
					ok     bool
				)
				if err := json.Unmarshal(msg.Data, &answer); err != nil {
					return fmt.Errorf("error unmarshaling answer: %w", err)
				}
				log.Printf("received answer from ws: from:%s to: %s\n", answer.From, answer.To)
				if conn, ok = conns.Get(answer.To); !ok {
					return fmt.Errorf("error: connection for user %s not found", answer.From)
				}
				if err = conn.pc.SetRemoteDescription(answer.Sd); err != nil {
					return fmt.Errorf("error while setting remote description: %w", err)
				}
				log.Printf("received answer from %s", answer.From)
			// this happens when the client is already in the room and a new user joins,
			// sending the client their offer
			case "offer":
				var (
					offer requests.ConnectionWithId
					conn  *Connection
					ok    bool
				)
				if err := json.Unmarshal(msg.Data, &offer); err != nil {
					return fmt.Errorf("error unmarshaling offer: %w", err)
				}
				if conn, ok = conns.Get(offer.From); ok {
					_ = conn.pc.Close() // temp
					log.Printf("recreating offer to %s", offer.From)
				}
				conn, err := newConnection(offer.FromId, creds.stunServer, track)
				if err != nil {
					return fmt.Errorf("error creating connection for %s: %w", offer.From, err)
				}
				conns.Update(offer.From, conn)
				log.Printf("received offer from %s, created conn", offer.From)

				// create and send answer
				_, err = wrtc.CreateAnswer(conn.pc, &offer.Sd)
				if err != nil {
					return fmt.Errorf("error creating or posting answer %w", err)
				}

				conn.pc.OnTrack(audio.OnRemoteTrack(playbackWg, pcmBufs))
				defer func() {
					// users may leave, and their pc's destroyed before this runs
					var ok bool
					if conn, ok = conns.Get(offer.From); !ok {
						return
					}
					if err := conn.pc.GracefulClose(); err != nil {
						log.Printf("graceful close error: %v", err)
					}
					log.Printf("%s's pc gracefully closed", offer.From)
				}()

				answer := requests.Connection{From: offer.To, To: offer.From, Sd: *conn.pc.LocalDescription()}
				bytes, err := json.Marshal(requests.ConnectionWithId{
					Connection: answer,
					FromId:     offer.ToId,
					ToId:       offer.FromId,
				})
				if err != nil {
					return fmt.Errorf("error encoding answer: %w", err)
				}

				msg := wsock.Message{Type: "answer", Data: bytes}
				if err := websocket.JSON.Send(ws, msg); err != nil {
					return fmt.Errorf("error sending candidate: %w", err)
				}
				log.Printf("sent answer (from %s) to %s to server", answer.From, answer.To)

				var sendIce sync.WaitGroup
				sendIceCtx, cancelSendIce := context.WithCancel(ctx)
				defer func() {
					cancelSendIce()
					sendIce.Wait()
				}()

				// gather local ice candidates for each peer and write to websocket
				sendIce.Go(func() {
					defer func() {
						cancelSendIce()
						log.Println("ice-answer sending done")
					}()
					log.Println("sending ice-answers now")
					if err = sendTaggedCandidates(sendIceCtx, ws, conn, creds.username, "ice-answer"); err != nil {
						log.Printf("SEND TAGGED CANDIDATES ERR: %v\n", err)
						abort <- err // this will cause surrounding function to cancel
						// TODO: this should not stop the entire join process. should fail gracefully and retry
					}
				})
			default:
				log.Printf("unknown message: %v", msg)
			}
		}
	}
}

// sendTaggedCandidates sends the client's ICE candidates from ch to the websocket as they're gathered.
// It sends the client's name along with the candidate. It returns when there are no more
// candidates or the context is cancelled.
func sendTaggedCandidates(ctx context.Context, ws *websocket.Conn, conn *Connection, callerName, tag string) error {
	defer log.Println("ice gathering completed")
	for {
		select {
		case <-ctx.Done():
			return nil
		case candidate, ok := <-conn.candidates:
			bytes, err := json.Marshal(messages.Candidate{
				UserId:    conn.id,
				Username:  callerName,
				Candidate: candidate,
			})
			if err != nil {
				return fmt.Errorf("error encoding candidate: %w", err)
			}

			msg := wsock.Message{Type: tag, Data: bytes}
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
