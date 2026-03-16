package wrtc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	Candidates chan webrtc.ICECandidateInit

	ConnectionStateChanges chan webrtc.PeerConnectionState
	ICEStateChanges        chan webrtc.ICEConnectionState

	// notification channels
	Connected chan struct{}
	RemoteSet chan struct{}

	Once      sync.Once
	closeOnce sync.Once
}

// NewConnection creates a new peer connection with a vogo user given their uuid, and returns a *Connection
// so the caller can keep track of the connection and signaling states.
func NewConnection(
	id uuid.UUID,
	recipient,
	stunServer string,
	track *webrtc.TrackLocalStaticSample,
) *Connection {
	pc := NewAudioPeerConnection(stunServer, track)
	conn := &Connection{
		Id: id,
		Pc: pc,

		// where ice candidates will be sent as they're gathered
		Candidates: make(chan webrtc.ICECandidateInit, 10),

		ICEStateChanges: make(chan webrtc.ICEConnectionState),

		// channel to pass along connection status pcUpdates
		ConnectionStateChanges: make(chan webrtc.PeerConnectionState),

		// notification channel for when the peer connection becomes connected
		Connected: make(chan struct{}),

		RemoteSet: make(chan struct{}),
	}

	// set up webrtc event handlers
	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			close(conn.Candidates)
			return
		}
		conn.Candidates <- c.ToJSON()
	})
	pc.OnSignalingStateChange(func(s webrtc.SignalingState) {
		log.Printf("SignalingState with %s has changed: %s", recipient, s.String())
	})
	pc.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		conn.onConnectionStateChange(s)
	})

	pc.OnICEConnectionStateChange(func(s webrtc.ICEConnectionState) {
		conn.onICEConnectionStateChange(s)
	})
	return conn
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
				c.Close()
				return
			}
			if !ok {
				return
			}
		}
	}
}

// HandleEvents will handle PeerConnection-related events such as status changes, manual retries
// and failure to write audio packets to the network. It will also handle updating the UI with the status changes.
// On PeerConnection failure, it returns an error, so returning that in an errgroup can be used to see when the PC
// has ended and is closed.
func (c *Connection) HandleEvents(ctx context.Context, peerName string) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case status, ok := <-c.ConnectionStateChanges:
			log.Printf("PeerConnectionState with %s has changed: %s\n", peerName, status.String())
			if !ok {
				c.ConnectionStateChanges = nil
				continue
			}

			switch status {
			case webrtc.PeerConnectionStateClosed, webrtc.PeerConnectionStateFailed:
				return io.EOF
			}
		// case failedPeer := <-c.audioState.Mic.FailedPeers():
		case status, ok := <-c.ICEStateChanges:
			log.Printf("IceConnectionState with %s has changed: %s\n", peerName, status.String())
			if !ok {
				c.ICEStateChanges = nil
				continue
			}

			switch status {
			case webrtc.ICEConnectionStateClosed, webrtc.ICEConnectionStateFailed:
				return io.EOF
			}
		}
	}
}

// onConnectionStateChange performs mandatory event handling for connection state changes. Logging
// and other non-essential event handling should be done in handleEvents.
func (c *Connection) onConnectionStateChange(state webrtc.PeerConnectionState) {
	c.ConnectionStateChanges <- state

	switch state {
	case webrtc.PeerConnectionStateConnected:
		close(c.Connected)
	// https://github.com/pion/webrtc/wiki/Release-WebRTC@v4.0.0
	// if PeerConnection was explicitly closed, this usually happens from a DTLS CloseNotify
	case webrtc.PeerConnectionStateClosed, webrtc.PeerConnectionStateFailed:
		close(c.ConnectionStateChanges)
		c.closeOnce.Do(func() { _ = c.Pc.Close() })
	}
}

func (c *Connection) onICEConnectionStateChange(state webrtc.ICEConnectionState) {
	c.ICEStateChanges <- state

	switch state {
	case webrtc.ICEConnectionStateClosed, webrtc.ICEConnectionStateFailed:
		close(c.ICEStateChanges)
		c.closeOnce.Do(func() { _ = c.Pc.Close() })
	}
}

// Close closes the PeerConnection held by the Connection.
func (c *Connection) Close() {
	// Note: replacing this with graceful close will hang and cause audio bug.
	// Some webrtc goroutine is not finishing...
	c.closeOnce.Do(func() { _ = c.Pc.Close() })
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

	// this is responsible for removing a Connection from the ConnectionMap
	// when it's PeerConnection is closed.
	Wg *sync.WaitGroup

	// all conns use this for sending ice candidates
	IceWg *sync.WaitGroup
}

func NewConnectionMap(
	ws *websocket.Conn,
	stunServer, username string,
) *ConnectionMap {
	return &ConnectionMap{
		data: make(map[string]*Connection, shared.ChannelCapacity),
		Server: serverConn{
			Ws:         ws,
			StunServer: stunServer,
			Username:   username,
		},
		Wg:    &sync.WaitGroup{},
		IceWg: &sync.WaitGroup{},
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

func (cm *ConnectionMap) Len() int {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return len(cm.data)
}

// TODO: do we want to call this every time a PC is closed? or retry closed conns?
func (cm *ConnectionMap) Delete(key string) {
	cm.mu.Lock()
	delete(cm.data, key)
	cm.mu.Unlock()
	log.Printf("deleted %s from connMap", key)
}

// Snapshot returns a shallow copy of the ConnectionMap. It should be used when iteration
// over the Connections is required, and it prevents concurrent writes from causing errors.
func (cm *ConnectionMap) Snapshot() map[string]*Connection {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return maps.Clone(cm.data)
}

// CloseAll closes all the Connection's PeerConnections.
func (cm *ConnectionMap) CloseAll() {
	var wg sync.WaitGroup
	cm.mu.Lock()

	for _, c := range cm.data {
		wg.Go(func() {
			c.Close()
		})
	}

	cm.mu.Unlock()
	wg.Wait()
}
