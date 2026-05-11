package netw

// conn.go contains the Connection struct, which is used to create and maintain a 1:1
// webrtc PeerConnection for voice chat. It is also used by the rooms package to compose state
// for multi-user voice channels.

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"

	"github.com/google/uuid"
	"github.com/gregriff/vogo/cli/internal/netw/wrtc"
	"github.com/gregriff/vogo/shared/requests"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v4"
)

// Connection encapsulates a bidirectional audio webrtc connection.
type Connection struct {
	// the uuid of the recipient user.
	Id uuid.UUID

	// webrtc PeerConnection
	Pc *webrtc.PeerConnection

	// used to obtain Receiver Reports
	sender *webrtc.RTPSender

	// used to obtain Sender Reports
	receiver *webrtc.RTPReceiver

	// channel for sending ICE Candidates
	Candidates chan webrtc.ICECandidateInit

	connStateChan chan webrtc.PeerConnectionState
	iceStateChan  chan webrtc.ICEConnectionState

	// notification channels
	Connected,
	RemoteSet chan struct{}

	// CloseRemoteSetOnce is used to signal that the remote description has been set on the PeerConnection
	CloseRemoteSetOnce sync.Once

	// closeOnce is used to ensure the PeerConnection is closed only one time.
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
	pc, sender, receiver := wrtc.NewAudioPeerConnection(stunServer, track)
	conn := &Connection{
		Id:       id,
		Pc:       pc,
		sender:   sender,
		receiver: receiver,

		// where ice candidates will be sent as they're gathered
		Candidates: make(chan webrtc.ICECandidateInit, 10),

		// NOTE: these being unbuffered may be causing a lock if pc.Close() is changed to pc.GracefulClose()
		iceStateChan: make(chan webrtc.ICEConnectionState),

		// channel to pass along connection status pcUpdates
		connStateChan: make(chan webrtc.PeerConnectionState),

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
	c.CloseRemoteSetOnce.Do(func() { close(c.RemoteSet) })

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

// CollectReceiverReports listens and collects ReceiverReports from the RTPSender. These
// can be used to estimate connection quality. Later this could be used to adjust bitrate.
// This function should be run in its own goroutine. It will return nil when the Sender is closed,
// which must be explicitally called somewhere.
func (c *Connection) CollectReceiverReports() error {
	buf := make([]byte, wrtc.RecvMTU)
	for {
		packets, _, err := wrtc.ReadSenderRTCP(c.sender, buf)
		if err != nil {
			if err == io.EOF || err == io.ErrClosedPipe {
				return nil // sender closed
			}
			log.Printf("readRTCP err: %v", err)
			continue // hopefully a temporary error
		}

		for _, pkt := range packets {
			switch report := pkt.(type) {
			case *rtcp.ReceiverReport:
				for _, block := range report.Reports {
					rtt := wrtc.CalculateRTT(&block)
					lossPercent := float64(block.FractionLost) / 256.0 * 100
					log.Printf("RR -- SSRC: %d | Loss: %.1f%% | Jitter: %.2f ms | RTT: %v",
						block.SSRC,
						lossPercent,
						wrtc.JitterToMs(block.Jitter),
						rtt,
					)
				}
			default:
				log.Printf("Got RTCP packet: %T", pkt)
			}
		}
	}
}

// CollectSenderReports listens and collects SenderReports from the RTPReceiver.
// For some reason, sender reports for a peer connectiondon't seem to be generated
// unless this function runs. It will return nil when the Receiver is closed,
// which must be explicitally called somewhere.
func (c *Connection) CollectSenderReports() error {
	buf := make([]byte, wrtc.RecvMTU)
	for {
		_, _, err := wrtc.ReadReceiverRTCP(c.receiver, buf)
		if err != nil {
			if err == io.EOF || err == io.ErrClosedPipe {
				return nil // sender closed
			}
			log.Printf("readRTCP err: %v", err)
			continue // hopefully a temporary error
		}
	}
}

// HandleEvents will handle PeerConnection-related events such as status changes, manual retries
// and failure to write audio packets to the network. It will also handle updating the UI with the status changes.
// On PeerConnection failure, it returns an error, so returning that in an errgroup can be used to see when the PC
// has ended and is closed.
func (c *Connection) HandleEvents(ctx context.Context, peerName string) error {
	defer func() {
		if err := c.sender.Stop(); err != nil {
			log.Printf("error stopping RTPSender: %v", err)
		}
		if err := c.receiver.Stop(); err != nil {
			log.Printf("error stopping RTPReceiver: %v", err)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case status, ok := <-c.connStateChan:
			log.Printf("PeerConnectionState with %s has changed: %s\n", peerName, status.String())
			if !ok {
				c.connStateChan = nil
				continue
			}

			switch status {
			case webrtc.PeerConnectionStateClosed, webrtc.PeerConnectionStateFailed:
				return io.EOF
			default:
				continue
			}
		// case failedPeer := <-c.audioState.Mic.FailedPeers():
		case status, ok := <-c.iceStateChan:
			log.Printf("IceConnectionState with %s has changed: %s\n", peerName, status.String())
			if !ok {
				c.iceStateChan = nil
				continue
			}

			switch status {
			case webrtc.ICEConnectionStateClosed, webrtc.ICEConnectionStateFailed:
				return io.EOF
			default:
				continue
			}
		}
	}
}

// onConnectionStateChange performs mandatory event handling for connection state changes. Logging
// and other non-essential event handling should be done in handleEvents.
func (c *Connection) onConnectionStateChange(state webrtc.PeerConnectionState) {
	c.connStateChan <- state

	switch state {
	case webrtc.PeerConnectionStateConnected:
		close(c.Connected)
	// https://github.com/pion/webrtc/wiki/Release-WebRTC@v4.0.0
	// if PeerConnection was explicitly closed, this usually happens from a DTLS CloseNotify
	case webrtc.PeerConnectionStateClosed, webrtc.PeerConnectionStateFailed:
		close(c.connStateChan)
		c.closeOnce.Do(func() { _ = c.Pc.Close() })
	default:
		return
	}
}

func (c *Connection) onICEConnectionStateChange(state webrtc.ICEConnectionState) {
	c.iceStateChan <- state

	switch state {
	case webrtc.ICEConnectionStateClosed, webrtc.ICEConnectionStateFailed:
		close(c.iceStateChan)
		c.closeOnce.Do(func() { _ = c.Pc.Close() })
	default:
		return
	}
}

// Close closes the PeerConnection held by the Connection.
func (c *Connection) Close() {
	// Note: replacing this with graceful close will hang and cause audio bug.
	// Some webrtc goroutine is not finishing...
	c.closeOnce.Do(func() { _ = c.Pc.Close() })
}
