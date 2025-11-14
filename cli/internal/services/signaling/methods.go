package signaling

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/pion/webrtc/v4"
	"golang.org/x/net/websocket"
)

type CallRequest struct {
	RecipientName string
	Sd            webrtc.SessionDescription
}

// func CallFriend(ctx context.Context, client http.Client, friendName string, offer webrtc.SessionDescription) (*webrtc.SessionDescription, error) {
// 	callReq := CallRequest{RecipientName: friendName, Sd: offer}
// 	payload, err := json.Marshal(callReq)
// 	if err != nil {
// 		return nil, fmt.Errorf("json marshal error: %w", err)
// 	}
// 	req, err := http.NewRequestWithContext(ctx, "POST", "/call", bytes.NewReader(payload))
// 	if err != nil {
// 		return nil, fmt.Errorf("error creating call request: %w", err)
// 	}

// 	// Send our offer to vogo-server. This request will hang until recipient answers
// 	res, err := client.Do(req)
// 	if err != nil {
// 		return nil, fmt.Errorf("error during call request: %w", err)
// 	}

// 	defer func() {
// 		_ = res.Body.Close()
// 	}()

// 	if res.StatusCode != 200 {
// 		// TODO: make this a sentinel error
// 		return nil, fmt.Errorf("call unsucessful: %s", res.Status)
// 	}

// 	sd := webrtc.SessionDescription{}
// 	if err := json.NewDecoder(res.Body).Decode(&sd); err != nil {
// 		return nil, fmt.Errorf("error parsing answer: %w", err)
// 	}
// 	return &sd, nil
// }

func GetCallerSd(client http.Client, callerName string) (*webrtc.SessionDescription, error) {
	res, err := client.Get(fmt.Sprintf("/caller/%s", callerName))
	if err != nil {
		return nil, fmt.Errorf("request to get caller information failed: %w", err)
	}
	defer func() {
		_ = res.Body.Close()
	}()

	log.Printf("Recieved /caller response: %s", res.Status)
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get caller from server: %s", res.Status)
	}

	// this needs to be the callers Sd
	callerSd := webrtc.SessionDescription{}
	if err = json.NewDecoder(res.Body).Decode(&callerSd); err != nil {
		return nil, fmt.Errorf("json decode error: %w", err)
	}
	return &callerSd, nil
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

func PostAnswer(client http.Client, callerName string, localDescription webrtc.SessionDescription) error {
	answerReq := AnswerRequest{CallerName: callerName, Sd: localDescription}
	payload, err := json.Marshal(answerReq)
	if err != nil {
		return fmt.Errorf("json marshal error: %w", err)
	}

	log.Println("answerer actually posting answer to vogo server")
	res, err := client.Post(
		"/answer",
		"application/json; charset=utf-8",
		bytes.NewReader(payload),
	)
	if err != nil {
		return fmt.Errorf("error posting answer: %w", err)
	}
	defer func() {
		_ = res.Body.Close()
	}()

	if res.StatusCode != 200 {
		return fmt.Errorf("failed to post answer: %s", res.Status)
	}

	log.Println("Answer was successful")
	return nil
}
