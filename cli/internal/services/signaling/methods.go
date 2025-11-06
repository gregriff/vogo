package signaling

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/pion/webrtc/v4"
)

type CallRequest struct {
	RecipientName string
	Sd            webrtc.SessionDescription
}

func CallFriend(client http.Client, friendName string, offer webrtc.SessionDescription) (*webrtc.SessionDescription, error) {
	callReq := CallRequest{RecipientName: friendName, Sd: offer}
	payload, err := json.Marshal(callReq)
	if err != nil {
		return nil, fmt.Errorf("error creating call request: %w", err)
	}
	// Send our offer to vogo-server. This request will hang until recipient answers
	log.Printf("actually posting offer to vogo server (/call)")
	res, rErr := client.Post(
		"/call",
		"application/json; charset=utf-8",
		bytes.NewReader(payload),
	)

	if rErr != nil {
		return nil, fmt.Errorf("error during call request: %w", rErr)
	}

	defer func() {
		_ = res.Body.Close()
	}()

	log.Printf("Recieved answerer response: %s", res.Status)
	if res.StatusCode != 200 {
		// TODO: make this a sentinel error
		return nil, fmt.Errorf("call unsucessful: %w", rErr)
	}

	sd := webrtc.SessionDescription{}
	if sdpErr := json.NewDecoder(res.Body).Decode(&sd); sdpErr != nil {
		return nil, fmt.Errorf("error parsing answer: %w", sdpErr)
	}
	return &sd, nil
}

func GetCallerSd(client http.Client, callerName string) *webrtc.SessionDescription {
	res, err := client.Get(fmt.Sprintf("/caller/%s", callerName))
	if err != nil {
		log.Fatalf("request to get caller information failed: %v", err)
	}

	log.Printf("Recieved /caller response: %s", res.Status)
	if res.StatusCode != 200 {
		log.Fatalf("failed to get caller from server: %s", res.Status)
	}

	// this needs to be the callers Sd
	callerSd := webrtc.SessionDescription{}
	if err = json.NewDecoder(res.Body).Decode(&callerSd); err != nil {
		panic(err)
	}
	_ = res.Body.Close()
	return &callerSd
}

type AnswerRequest struct {
	CallerName string
	Sd         webrtc.SessionDescription
}

func PostAnswer(client http.Client, callerName string, localDescription webrtc.SessionDescription) {
	answerReq := AnswerRequest{CallerName: callerName, Sd: localDescription}
	payload, err := json.Marshal(answerReq)
	if err != nil {
		panic(err)
	}

	log.Println("answerer actually posting answer to vogo server")
	res, err := client.Post(
		"/answer",
		"application/json; charset=utf-8",
		bytes.NewReader(payload),
	)
	if err != nil {
		log.Fatalf("error posting answer: %v", err)
	}
	if res.StatusCode != 200 {
		log.Fatalf("failed to post answer: %s", res.Status)
	}

	log.Println("Answer was successful")
	_ = res.Body.Close()
}
