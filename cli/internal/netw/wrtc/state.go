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
	"github.com/gregriff/vogo/cli/internal/audio"
	"github.com/gregriff/vogo/shared"
	"github.com/gregriff/vogo/shared/requests"
	"github.com/gregriff/vogo/shared/wsock"
	"github.com/gregriff/vogo/shared/wsock/messages"
	"github.com/pion/interceptor"
	"github.com/pion/rtcp"
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

	// used to obtain Receiver Reports
	Sender *webrtc.RTPSender

	// used to obtain Sender Reports
	Receiver *webrtc.RTPReceiver

	// channel for sending ICE Candidates
	Candidates chan webrtc.ICECandidateInit

	ConnStateChan chan webrtc.PeerConnectionState
	ICEStateChan  chan webrtc.ICEConnectionState

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
	pc, sender, receiver := NewAudioPeerConnection(stunServer, track)
	conn := &Connection{
		Id:       id,
		Pc:       pc,
		Sender:   sender,
		Receiver: receiver,

		// where ice candidates will be sent as they're gathered
		Candidates: make(chan webrtc.ICECandidateInit, 10),

		// NOTE: these being unbuffered may be causing a lock if pc.Close() is changed to pc.GracefulClose()
		ICEStateChan: make(chan webrtc.ICEConnectionState),

		// channel to pass along connection status pcUpdates
		ConnStateChan: make(chan webrtc.PeerConnectionState),

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

type candidateType int

const (
	CandidateICEOffer = iota
	CanidateICEAnswer
)

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

			var tagStr string
			if tag == CandidateICEOffer {
				tagStr = "ice-offer"
			} else {
				tagStr = "ice-answer"
			}
			msg := wsock.Message{Type: tagStr, Data: bytes}
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

// CollectReceiverReports listens and collects ReceiverReports from the RTPSender. These
// can be used to estimate connection quality. Later this could be used to adjust bitrate.
// This function should be run in its own goroutine. It will return nil when the Sender is closed,
// which must be explicitally called somewhere.
func (c *Connection) CollectReceiverReports() error {
	buf := make([]byte, RecvMTU)
	for {
		packets, _, err := c.ReadRTCP(receiverReportMode, buf)
		if err != nil {
			if err == io.EOF {
				return nil // sender closed
			}
			log.Printf("readRTCP err: %v", err)
			continue // hopefully a temporary error
		}

		for _, pkt := range packets {
			switch report := pkt.(type) {
			case *rtcp.ReceiverReport:
				for _, block := range report.Reports {
					rtt := calculateRTT(&block)
					lossPercent := float64(block.FractionLost) / 256.0 * 100
					log.Printf("RR -- SSRC: %d | Loss: %.1f%% | Jitter: %.2f ms | RTT: %v",
						block.SSRC,
						lossPercent,
						jitterToMs(block.Jitter),
						rtt,
					)
				}
			default:
				log.Printf("Got RTCP packet: %T", pkt)
			}
		}
	}
}

// CollectSenderReports listens and reads RTCP sender reports until the sender is stopped.
// For some reason, sender reports for a peer connectiondon't seem to be generated
// unless this function runs.
func (c *Connection) CollectSenderReports() error {
	buf := make([]byte, RecvMTU)
	for {
		_, _, err := c.ReadRTCP(senderReportMode, buf)
		if err != nil {
			if err == io.EOF {
				return nil // sender closed
			}
			log.Printf("readRTCP err: %v", err)
			continue // hopefully a temporary error
		}
	}
}

type RTCPReadMode int

const (
	senderReportMode = iota
	receiverReportMode
)

// ReadRTCP reads an RTCP packet into a preallocated buffer and returns a slice of the RTCP packets, if any.
// It must be set to operate on either the RTPSender or RTPReceiver.
func (c *Connection) ReadRTCP(mode RTCPReadMode, dst []byte) (pkts []rtcp.Packet, iAttrs interceptor.Attributes, err error) {
	var i int
	if mode == senderReportMode {
		i, iAttrs, err = c.Receiver.Read(dst)
	} else { // ReceiverReport
		i, iAttrs, err = c.Sender.Read(dst)
	}
	if err != nil {
		return nil, nil, err
	}

	pkts, err = rtcp.Unmarshal(dst[:i])
	if err != nil {
		return nil, nil, err
	}

	return pkts, iAttrs, nil
}

// HandleEvents will handle PeerConnection-related events such as status changes, manual retries
// and failure to write audio packets to the network. It will also handle updating the UI with the status changes.
// On PeerConnection failure, it returns an error, so returning that in an errgroup can be used to see when the PC
// has ended and is closed.
func (c *Connection) HandleEvents(ctx context.Context, peerName string) error {
	defer func() {
		if err := c.Sender.Stop(); err != nil {
			log.Printf("error stopping RTPSender: %v", err)
		}
		if err := c.Receiver.Stop(); err != nil {
			log.Printf("error stopping RTPReceiver: %v", err)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case status, ok := <-c.ConnStateChan:
			log.Printf("PeerConnectionState with %s has changed: %s\n", peerName, status.String())
			if !ok {
				c.ConnStateChan = nil
				continue
			}

			switch status {
			case webrtc.PeerConnectionStateClosed, webrtc.PeerConnectionStateFailed:
				return io.EOF
			}
		// case failedPeer := <-c.audioState.Mic.FailedPeers():
		case status, ok := <-c.ICEStateChan:
			log.Printf("IceConnectionState with %s has changed: %s\n", peerName, status.String())
			if !ok {
				c.ICEStateChan = nil
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
	c.ConnStateChan <- state

	switch state {
	case webrtc.PeerConnectionStateConnected:
		close(c.Connected)
	// https://github.com/pion/webrtc/wiki/Release-WebRTC@v4.0.0
	// if PeerConnection was explicitly closed, this usually happens from a DTLS CloseNotify
	case webrtc.PeerConnectionStateClosed, webrtc.PeerConnectionStateFailed:
		close(c.ConnStateChan)
		c.closeOnce.Do(func() { _ = c.Pc.Close() })
	}
}

func (c *Connection) onICEConnectionStateChange(state webrtc.ICEConnectionState) {
	c.ICEStateChan <- state

	switch state {
	case webrtc.ICEConnectionStateClosed, webrtc.ICEConnectionStateFailed:
		close(c.ICEStateChan)
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

	// Server encapsulates a websocket connection to the vogo server.
	Server serverConn

	// Channel allows access to the audio devices and systems of the multi-user voice channel.
	Channel *audio.Channel

	// this is responsible for removing a Connection from the ConnectionMap
	// when it's PeerConnection is closed.
	Wg *sync.WaitGroup

	// all conns use this for sending ice candidates
	IceWg *sync.WaitGroup
}

func NewConnectionMap(
	ws *websocket.Conn,
	channel *audio.Channel,
	stunServer, username string,
) *ConnectionMap {
	return &ConnectionMap{
		data: make(map[string]*Connection, shared.ChannelCapacity),
		Server: serverConn{
			Ws:         ws,
			StunServer: stunServer,
			Username:   username,
		},
		Channel: channel,
		Wg:      &sync.WaitGroup{},
		IceWg:   &sync.WaitGroup{},
	}
}

// AddConnection creates a new *Connection for the recipient, sets up audio playback for their RemoteTrack, and
// starts an eventlistener for this connection.
func (cm *ConnectionMap) AddConnection(ctx context.Context, recipientId uuid.UUID, recipientName string) *Connection {
	c := NewConnection(recipientId, recipientName, cm.Server.StunServer, cm.Channel.Mic.Track())
	cm.Channel.AddPeer(c.Pc)
	cm.Update(recipientName, c)
	cm.Wg.Go(func() {
		err := c.HandleEvents(ctx, recipientName)
		if err == io.EOF { // pc closed
			cm.Delete(recipientName)
		}
	})

	cm.Wg.Go(func() {
		_ = c.CollectSenderReports()
	})

	// TODO: this will need to be appended to a struct that stores them for UI to pull from.
	cm.Wg.Go(func() {
		_ = c.CollectReceiverReports()
	})
	return c
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
