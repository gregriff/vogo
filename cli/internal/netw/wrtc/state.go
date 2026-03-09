package wrtc

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"maps"
	"sync"

	"github.com/google/uuid"
	"github.com/gregriff/vogo/shared"
	"github.com/gregriff/vogo/shared/requests"
	"github.com/gregriff/vogo/shared/wsock"
	"github.com/gregriff/vogo/shared/wsock/messages"
	"github.com/pion/webrtc/v4"
	"golang.org/x/net/websocket"
)

// Connection encapsulates a bidirectional audio webrtc connection.
// TODO: this could be directly created by NewAudioPeerConnection.
type Connection struct {
	// the uuid of the recipient user.
	Id uuid.UUID

	// webrtc PeerConnection
	Pc *webrtc.PeerConnection

	// channel for sending ICE Candidates
	Candidates <-chan webrtc.ICECandidateInit

	StatusUpdates <-chan webrtc.PeerConnectionState

	// notification channels
	Connected <-chan struct{}
	RemoteSet chan struct{}

	Once sync.Once
}

// NewConnection creates a new peer connection with a vogo user given their uuid, and returns a *Connection
// so the caller can keep track of the connection and signaling states.
func NewConnection(
	id uuid.UUID,
	stunServer string,
	track *webrtc.TrackLocalStaticSample,
	exitOnClose bool,
) *Connection {
	pc, candidates, updates, connected := NewAudioPeerConnection(stunServer, track, exitOnClose)
	return &Connection{
		Id:            id,
		Pc:            pc,
		Candidates:    candidates,
		StatusUpdates: updates,
		Connected:     connected,
		RemoteSet:     make(chan struct{}),
	}
}

type candidateType string

var (
	_ candidateType = "ice-offer"
	_ candidateType = "ice-answer"
)

// NewOffer creates a webrtc offer, sets the local description and starts ice gathering.
func (c *Connection) NewOffer(caller, recipient string) requests.Connection {
	if recipient == "" {
		log.Panic("empty recipient in NewOfferRequest")
	}

	offer, err := c.Pc.CreateOffer(nil)
	if err != nil {
		log.Panicf("error creating offer: %v", err)
	}

	// starts ICE gathering and UDP listeners
	if err = c.Pc.SetLocalDescription(offer); err != nil {
		log.Panicf("error setting local description: %v", err)
	}

	req := requests.Connection{From: caller, To: recipient, Sd: offer}
	log.Printf("%s will send offer to %s...\n", caller, recipient)
	return req
}

// CreateAnswer sets the remote description of the caller given their offer,
// creates an answer, sets the local description and starts ice gathering.
// It returns the answer, but the up-to-date local description should probably
// be accessed directly from the pc.
func (c *Connection) CreateAnswer(offer *webrtc.SessionDescription) error {
	if err := c.Pc.SetRemoteDescription(*offer); err != nil {
		return fmt.Errorf("error setting remote description: %v", err)
	}
	c.Once.Do(func() { close(c.RemoteSet) })

	answer, err := c.Pc.CreateAnswer(nil)
	if err != nil {
		return fmt.Errorf("error starting pc or generating local description: %v", err)
	}

	// starts ICE gathering and UDP listeners
	if err := c.Pc.SetLocalDescription(answer); err != nil {
		return fmt.Errorf("error setting local description: %v", err)
	}
	return nil
}

// SendCandidates gathers local ICE candidates created for the connection's recipient
// and sends them to the server via the websocket in a new goroutine. It sends them with a
// [candidateType] tag to let the server know if they're ICE offers or answers.
func (c *Connection) SendCandidates(ctx context.Context, ws *websocket.Conn, callerName string, tag candidateType) {
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

// HandleEvents will handle PeerConnection-related events such as status changes, manual retries
// and failure to write audio packets to the network.
func (c *Connection) HandleEvents(ctx context.Context, peerName string) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case status := <-c.StatusUpdates:
			log.Printf("Peer Connection State with %s has changed: %s\n", peerName, status.String())
			// case failedPeer := <-c.audioState.Mic.FailedPeers():
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
	IceWg *sync.WaitGroup
}

func NewConnectionMap(
	ws *websocket.Conn,
	stunServer, username string,
	iceWg *sync.WaitGroup,
) *ConnectionMap {
	return &ConnectionMap{
		data: make(map[string]*Connection, shared.ChannelCapacity),
		Server: serverConn{
			Ws:         ws,
			StunServer: stunServer,
			Username:   username,
		},
		IceWg: iceWg,
	}
}

// Get returns a *Connection if key is in the ConnectionMap. Nil if not.
func (cm *ConnectionMap) Get(key string) *Connection {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return cm.data[key]
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
				log.Printf("error while trying to close %s's pc: %v", key, err)
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
