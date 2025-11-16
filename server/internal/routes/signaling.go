package routes

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"

	"github.com/gregriff/vogo/server/internal/dal"
	"github.com/gregriff/vogo/server/internal/middleware"
	"github.com/gregriff/vogo/server/internal/schemas"
	"github.com/pion/webrtc/v4"
	"golang.org/x/net/websocket"
)

// TODO:
// GET /status: returns to the client any calls or channels currently open and associated with the client
// POST /call: allow client to create a call to one other client. client B will need to accept the call for it to begin. call properties
// 			   stored in memory and wiped when all parties leave it
// POST /channel: allow a client to open a channel. this is persisted to sqlite, including who is invited to join it.
// 				  channels are given a unique name, where the CLI can change properties (who's invited) using the PUT
// PUT /channel: modify channel properties
// DELETE /channel

// CallWS lets the caller post an offer, and update it as their ICE candidates come in.
// if the offer (sd) is updated and there are other clients listening for this call, the offer will be sent to them
// via their websocket
//
// CallWS first gets the offer from the caller, and once the answerer is ready it sends this offer to the answerer.
// from then on out, this func queues up ice candidates sent by the caller, that are sent to the answerer once theyre ready.
// it returns once the call connects.
// Note: the channel version of this func will need to stay open until the client exits the channel call.
func (h *RouteHandler) CallWS(ws *websocket.Conn) {
	username := middleware.GetUsernameWS(ws)
	caller, sqlErr := dal.GetUserByUsername(h.db, username)
	if sqlErr != nil {
		log.Println(fmt.Errorf("error fetching caller: %w", sqlErr))
		ws.WriteClose(http.StatusInternalServerError)
		return
	}
	calls := schemas.GetPendingCalls()

	var (
		// TODO: with channel rooms, these chans will need to be per-client
		answerChan = make(chan webrtc.SessionDescription, 1)

		// the answerer will listen on this to recieve the callers ICE candidates
		iceSendChan = make(chan webrtc.ICECandidateInit, 15) // 15 max ice candidates should be enough?

		// we will listen on this to recieve answerer's ICE candidates as they trickle in
		iceRecvChan = make(chan webrtc.ICECandidateInit, 15)

		// for websocket reading ////////////////////////
		readErr    error
		readWg     sync.WaitGroup
		wsReadChan = make(chan webrtc.ICECandidateInit)

		// websocket messages
		candidate webrtc.ICECandidateInit
		offer     schemas.CallRequest
		/////////////////////////////////////////////////
	)
	defer func() {
		ws.Close()
		readWg.Wait()
		log.Println("readWg closed")
	}()

	// TODO: create a codec and quit using the buffer to read, since messages could queue and n is overwritten.
	// currently, the cases are triggering even if its not the correct message

	readErr = websocket.JSON.Receive(ws, &offer)
	if readErr != nil {
		log.Printf("error reading offer from ws: %v", readErr)
		ws.WriteClose(http.StatusBadRequest)
		return
	}
	if offer.Sd.SDP == "" {
		log.Println("empty offer")
		ws.WriteClose(http.StatusBadRequest)
		return
	}
	log.Println("callWS: offer recieved")
	recipient, sqlErr := dal.GetUserByUsername(h.db, offer.RecipientName)
	if sqlErr != nil {
		log.Println(fmt.Errorf("error fetching recipient: %w", sqlErr))
		ws.WriteClose(http.StatusBadRequest)
		return
	}
	schemas.CreateCall(caller, recipient, offer.Sd, iceSendChan, iceRecvChan, answerChan)
	log.Println("call created")

	readWg.Go(func() {
		for {
			readErr = websocket.JSON.Receive(ws, &candidate)
			if readErr != nil {
				if readErr == io.EOF {
					ws.Close()
					return
				}
				log.Printf("error reading from ws: %v", readErr)
				ws.WriteClose(http.StatusInternalServerError)
				return
			}
			wsReadChan <- candidate
		}
	})

	// this request will be canceled by the client once the call is successful,
	// or it will timeout
	ctx := ws.Request().Context()
	for {
		select {
		case <-ctx.Done():
			calls.Delete(caller.Id)
			return
		case answerSd := <-answerChan:
			if wErr := wsWrite(ws, answerSd); wErr != nil {
				log.Printf("error writing answer: %v", wErr)
				return
			}
		case answerCandidate := <-iceRecvChan:
			if wErr := wsWrite(ws, answerCandidate); wErr != nil {
				log.Printf("error writing candidate: %v", wErr)
				return
			}
		case callerCandidate := <-wsReadChan:
			if callerCandidate.Candidate == "" {
				log.Println("empty candidate recieved")
				return
			}
			call, err := calls.Get(caller.Id)
			if err != nil {
				log.Print("call not found during trickle ice")
				return
			}
			call.From.CandidateChan <- callerCandidate
			fmt.Println("caller candidate sent to chan")
		}
	}
}

// AnswerWS obtains the caller's name from the first ws message and sends the caller's offer Sd to the client.
// It then waits for the clients answer, where it then facilitates trickle-ICE gathering between the two clients.
func (h *RouteHandler) AnswerWS(ws *websocket.Conn) {
	username := middleware.GetUsernameWS(ws)
	_, sqlErr := dal.GetUserByUsername(h.db, username)
	if sqlErr != nil {
		log.Println(fmt.Errorf("error fetching recipient: %w", sqlErr))
		ws.WriteClose(http.StatusInternalServerError)
		return
	}

	callerName := ws.Request().PathValue("name")
	caller, sqlErr := dal.GetUserByUsername(h.db, callerName)
	if sqlErr != nil {
		log.Println(fmt.Errorf("error fetching caller: %w", sqlErr))
		ws.WriteClose(http.StatusBadRequest)
		return
	}
	calls := schemas.GetPendingCalls()

	call, err := calls.Get(caller.Id)
	if err != nil {
		log.Printf("%v", err)
		ws.WriteClose(http.StatusBadRequest)
		return
	}

	// send caller's SD. client will then create an answer and post it to this ws
	if wErr := wsWrite(ws, call.From.Sd); wErr != nil {
		log.Printf("error writing offer: %v", wErr)
		return
	}

	// for websocket reading
	var (
		readErr    error
		readWg     sync.WaitGroup
		wsReadChan = make(chan webrtc.ICECandidateInit)

		// websocket messages
		candidate webrtc.ICECandidateInit
		answer    schemas.AnswerRequest
	)
	defer func() {
		ws.Close()
		readWg.Wait()
		log.Println("readWg closed")
	}()

	readErr = websocket.JSON.Receive(ws, &answer)
	if readErr != nil {
		log.Printf("error reading answer from ws: %v", readErr)
		ws.WriteClose(http.StatusBadRequest)
		return
	}
	if answer.Sd.SDP == "" {
		log.Println("empty answer")
		ws.WriteClose(http.StatusBadRequest)
		return
	}
	log.Println("answerWS: answer recieved")

	// TODO: dont need this now that the request is stateful
	caller, sqlErr = dal.GetUserByUsername(h.db, answer.CallerName)
	if sqlErr != nil {
		log.Println(fmt.Errorf("error fetching caller: %w", sqlErr))
		ws.WriteClose(http.StatusInternalServerError)
		return
	}
	call, err = calls.Get(caller.Id)
	if err != nil {
		log.Printf("call not found during answer: %v", err)
		return
	}
	fmt.Println("answer sending to chan")
	call.AnswerChan <- answer.Sd

	readWg.Go(func() {
		for {
			readErr = websocket.JSON.Receive(ws, &candidate)
			if readErr != nil {
				if readErr == io.EOF {
					ws.Close()
					return
				}
				log.Printf("error reading from ws: %v", readErr)
				ws.WriteClose(http.StatusInternalServerError)
				return
			}
			wsReadChan <- candidate
		}
	})

	ctx := ws.Request().Context()
	for {
		select {
		case <-ctx.Done():
			calls.Delete(caller.Id)
			return
		case candidate := <-call.From.CandidateChan:
			if wErr := wsWrite(ws, candidate); wErr != nil {
				log.Printf("error writing answer: %v", wErr)
				return
			}
		case answerCandidate := <-wsReadChan:
			if answerCandidate.Candidate == "" {
				log.Println("empty candidate recieved")
				return
			}
			call, err := calls.Get(caller.Id)
			if err != nil {
				log.Print("answer: call not found during trickle ice")
				return
			}
			call.To.CandidateChan <- answerCandidate
			fmt.Println("answer candidate sent to chan")
		}
	}
}

func wsWrite(ws *websocket.Conn, data any) error {
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	_, err = ws.Write(bytes)
	if err != nil {
		return err
	}
	return nil
}
