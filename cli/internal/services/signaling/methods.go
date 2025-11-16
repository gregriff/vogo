package signaling

import (
	"fmt"
	"log"

	"github.com/pion/webrtc/v4"
	"golang.org/x/net/websocket"
)

type CallRequest struct {
	RecipientName string
	Sd            webrtc.SessionDescription
}

type AnswerRequest struct {
	CallerName string
	Sd         webrtc.SessionDescription
}

func CreateAndPostAnswer(ws *websocket.Conn, pc *webrtc.PeerConnection, offer *webrtc.SessionDescription, callerName string) error {
	log.Println("setting caller's remote description")
	if err := pc.SetRemoteDescription(*offer); err != nil {
		fmt.Printf("error setting callers remote description: %v", err)
		return err
	}

	log.Println("answerer creating answer")
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		fmt.Printf("error creating answer: %v", err)
		return err
	}

	// Sets the LocalDescription, and starts our UDP listeners
	log.Println("answerer setting localDescription and listening for UDP")
	err = pc.SetLocalDescription(answer)
	if err != nil {
		fmt.Printf("error setting local description: %v", err)
		return err
	}
	answerReq := AnswerRequest{CallerName: callerName, Sd: *pc.LocalDescription()}
	if err = WriteWS(ws, answerReq); err != nil {
		return err
	}
	return nil
}
