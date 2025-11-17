package signaling

import (
	"fmt"

	"github.com/pion/webrtc/v4"
	"golang.org/x/net/websocket"
)

type callRequest struct {
	RecipientName string
	Sd            webrtc.SessionDescription
}

type answerRequest struct {
	CallerName string
	Sd         webrtc.SessionDescription
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

	callReq := callRequest{RecipientName: recipient, Sd: offer}
	if err = websocket.JSON.Send(ws, callReq); err != nil {
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

	answerReq := answerRequest{CallerName: callerName, Sd: *pc.LocalDescription()}
	if err = websocket.JSON.Send(ws, answerReq); err != nil {
		return fmt.Errorf("error sending answer: %w", err)
	}
	return nil
}
