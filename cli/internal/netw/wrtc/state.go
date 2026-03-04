package wrtc

import (
	"context"
	"encoding/json"
	"log"
	"maps"
	"sync"

	"github.com/google/uuid"
	"github.com/gregriff/vogo/shared/requests"
	"github.com/gregriff/vogo/shared/wsock"
	"github.com/gregriff/vogo/shared/wsock/messages"
	"github.com/pion/webrtc/v4"
	"golang.org/x/net/websocket"
)

// Connection encapsulates a bidirectional audio webrtc connection.
// TODO: this could be directly created by NewAudioPeerConnection
type Connection struct {
	// the uuid of the recipient user.
	Id uuid.UUID

	// webrtc PeerConnection
	Pc *webrtc.PeerConnection

	// track which audio is written to
	// track *webrtc.TrackLocalStaticSample

	// channel for sending ICE Candidates
	Candidates chan webrtc.ICECandidateInit

	// notification channel for connection status
	Connected chan struct{}
}

// NewConnection creates a new peer connection with a vogo user given their uuid, and returns a *Connection
// so the caller can keep track of the connection and signaling states.
func NewConnection(
	id uuid.UUID,
	stunServer string,
	track *webrtc.TrackLocalStaticSample,
) *Connection {
	pc, candidates, connected := NewAudioPeerConnection(stunServer, track, false)
	c := Connection{
		Id: id,
		Pc: pc,
		// track:      track,
		Candidates: candidates,
		Connected:  connected,
	}
	return &c
}

type candidateType string

var (
	iceOffer  candidateType = "ice-offer"
	iceAnswer candidateType = "ice-answer"
)

func (c *Connection) NewOfferRequest(caller, recipient string) requests.Connection {
	if recipient == "" {
		log.Panic("empty recipient in NewOfferRequest")
	}
	offer := CreateOffer(c.Pc)
	req := requests.Connection{From: caller, To: recipient, Sd: offer}
	log.Printf("%s will send offer to %s...\n", caller, recipient)
	return req
}

// SendCandidates gathers local ICE candidates created for the connection's recipient
// and sends them to the server via the websocket in a new goroutine.
func (c *Connection) SendCandidates(
	ctx context.Context,
	wg *sync.WaitGroup,
	ws *websocket.Conn,
	username string,
	tag candidateType,
) {
	// gather local ice candidates for each peer and write to websocket
	wg.Go(func() {
		defer func() {
			log.Printf("%s sending done", tag)
		}()
		log.Printf("sending %s's now", tag)
		c.sendTaggedCandidates(ctx, ws, username, tag)
	})
}

// sendTaggedCandidates sends the client's ICE candidates from ch to the websocket as they're gathered.
// It sends the client's name along with the candidate. It returns when there are no more
// candidates or the context is cancelled.
func (c *Connection) sendTaggedCandidates(ctx context.Context, ws *websocket.Conn, callerName string, tag candidateType) {
	for {
		select {
		case <-ctx.Done():
			return
		case candidate, ok := <-c.Candidates:
			bytes, err := json.Marshal(messages.Candidate{
				UserId:    c.Id,
				Username:  callerName,
				Candidate: candidate,
			})
			if err != nil {
				log.Panicf("error encoding candidate: %v", err)
			}

			msg := wsock.Message{Type: string(tag), Data: bytes}
			if err := websocket.JSON.Send(ws, msg); err != nil {
				log.Printf("error sending candidate: %v", err)
				if cErr := c.Pc.GracefulClose(); cErr != nil {
					log.Printf("error closing pc after send candidate error: %v", cErr)
				}
				return
			}
			if !ok {
				return
			}
		}
	}
}

type serverConn struct {
	Ws *websocket.Conn
	StunServer,
	Username string
}

// ConnectionMap maps recipient usernames to Connections. It is used to store the state of all
// outgoing connections to members of a channel.
type ConnectionMap struct {
	mu   sync.Mutex
	data map[string]*Connection

	Server serverConn

	// all conns use this for sending ice candidates
	IceCtx context.Context

	// all conns use this for sending ice candidates
	IceWg *sync.WaitGroup
}

func NewConnectionMap(
	ws *websocket.Conn,
	stunServer, username string,
	iceCtx context.Context,
	iceWg *sync.WaitGroup,
) *ConnectionMap {
	return &ConnectionMap{
		data: make(map[string]*Connection, 6),
		Server: serverConn{
			Ws:         ws,
			StunServer: stunServer,
			Username:   username,
		},
		IceCtx: iceCtx,
		IceWg:  iceWg,
	}
}

func (cm *ConnectionMap) Get(username string) (*Connection, bool) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	c, ok := cm.data[username]
	return c, ok
}

func (cm *ConnectionMap) Update(key string, c *Connection) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.data[key] = c
}

// TODO: do we want to call this every time a PC is closed? or retry closed conns?
// func (cm *ConnectionMap) Delete(key string) {
// 	cm.mu.Lock()
// 	delete(cm.data, key)
// 	cm.mu.Unlock()
// }

// Snapshot returns a shallow copy of the ConnectionMap. It should be used when iteration
// over the Connections is required, and it prevents concurrent writes from causing errors.
func (cm *ConnectionMap) Snapshot() map[string]*Connection {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return maps.Clone(cm.data)
}

func (cm *ConnectionMap) CloseAll() {
	log.Println("closing all conns in connmap")
	var wg sync.WaitGroup
	cm.mu.Lock()

	for key, c := range cm.data {
		wg.Go(func() {
			if err := c.Pc.GracefulClose(); err != nil {
				log.Printf("error while trying to close %s's pc: %v", err)
			}
			log.Printf("%s's pc closed", key)
		})
	}

	cm.mu.Unlock()
	wg.Wait()
}

// EnsureClosed closes the PeerConnection stored by ConnectionMap with the given key,
// if it exists. NOTE: users may leave, and their pc's destroyed before this runs
// func (cm *ConnectionMap) EnsureClosed(key string) {
// 	// defer cm.Delete(key)
// 	if c, ok := cm.Get(key); ok {
// 		if err := c.Pc.GracefulClose(); err != nil {
// 			log.Printf("error while trying to close %s's pc: %v", err)
// 		}
// 		log.Printf("%s's pc closed", key)
// 		return
// 	}
// 	log.Printf("tried to close connection for %s, did not exist", key)
// }
