package wrtc

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
	"golang.org/x/net/websocket"
)

// ConnectionRequest encapsulates offers and answers. It can optionally contain
// the username of the sender and/or the recipient, depending on if additional context is needed.
type ConnectionRequest struct {
	From,
	To string
	Sd webrtc.SessionDescription
}

// ConnectionRequestWithId extends ConnectionRequest with user Ids.
type ConnectionRequestWithId struct {
	ConnectionRequest
	FromId,
	ToId uuid.UUID
}

// CreateAndSendOffer creates the offer, starts ICE gathering, and sends the offer over ws,
// for the specified recipient (username)
func CreateAndSendOffer(ws *websocket.Conn, pc *webrtc.PeerConnection, recipient string) error {
	offer, err := pc.CreateOffer(nil)
	if err != nil {
		return fmt.Errorf("error creating offer: %v", err)
	}

	// starts ICE gathering and UDP listeners
	if err = pc.SetLocalDescription(offer); err != nil {
		return fmt.Errorf("error setting local description: %v", err)
	}

	req := ConnectionRequest{To: recipient, Sd: offer}
	if err = websocket.JSON.Send(ws, req); err != nil {
		return fmt.Errorf("error sending offer: %w", err)
	}
	return nil
}

// CreateAndSendAnswer sets the remote description of the caller given their offer, creates the answer,
// starts ICE gathering, then sends the answer to ws
func CreateAndSendAnswer(ws *websocket.Conn, pc *webrtc.PeerConnection, offer *webrtc.SessionDescription, callerName string) error {
	if err := pc.SetRemoteDescription(*offer); err != nil {
		return fmt.Errorf("error setting remote description: %v", err)
	}

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		return fmt.Errorf("error creating answer: %v", err)
	}

	// starts ICE gathering and UDP listeners
	err = pc.SetLocalDescription(answer)
	if err != nil {
		return fmt.Errorf("error setting local description: %v", err)
	}

	req := ConnectionRequest{To: callerName, Sd: *pc.LocalDescription()}
	if err = websocket.JSON.Send(ws, req); err != nil {
		return fmt.Errorf("error sending answer: %w", err)
	}
	return nil
}
