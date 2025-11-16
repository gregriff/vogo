package routes

import (
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
		readErr  error
		readWg   sync.WaitGroup
		readChan = make(chan webrtc.ICECandidateInit)

		// websocket messages
		candidate webrtc.ICECandidateInit
		offer     schemas.CallRequest
		/////////////////////////////////////////////////
	)
	defer func() {
		cErr := ws.Close()
		if cErr != nil {
			log.Println("error closing ws during defer: ", cErr)
		}
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
		ReadForever(ws, candidate, readChan)
	})

	// this request will be canceled by the client once the call is successful,
	// or it will timeout
	ctx := ws.Request().Context()
	for {
		select {
		case <-ctx.Done():
			log.Println("Call req context done")
			calls.Delete(caller.Id)
			return
		case answerSd := <-answerChan:
			if wErr := websocket.JSON.Send(ws, answerSd); wErr != nil {
				log.Printf("error writing answer: %v", wErr)
				return
			}
		case answerCandidate, ok := <-iceRecvChan:
			if wErr := websocket.JSON.Send(ws, answerCandidate); wErr != nil {
				log.Printf("error writing candidate: %v", wErr)
				return
			}
			if !ok {
				log.Println("nil answer candidate sent, call will succeed")
				return
			}
		case callerCandidate := <-readChan:
			// TODO: could have readforever do this check and close the chan, and check closure here
			if callerCandidate.Candidate == "" {
				log.Println("empty candidate recieved: ", callerCandidate)
				break
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
	if wErr := websocket.JSON.Send(ws, call.From.Sd); wErr != nil {
		log.Printf("error writing offer: %v", wErr)
		return
	}

	// for websocket reading
	var (
		readErr  error
		readWg   sync.WaitGroup
		readChan = make(chan webrtc.ICECandidateInit)

		// websocket messages
		candidate webrtc.ICECandidateInit
		answer    schemas.AnswerRequest
	)
	defer func() {
		cErr := ws.Close()
		if cErr != nil {
			log.Println("error closing ws during defer: ", cErr)
		}
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
	call.AnswerChan <- answer.Sd

	readWg.Go(func() {
		ReadForever(ws, candidate, readChan)
	})

	ctx := ws.Request().Context()
	for {
		select {
		case <-ctx.Done():
			log.Println("Answer req context done")
			calls.Delete(caller.Id)
			return
		case candidate := <-call.From.CandidateChan:
			if wErr := websocket.JSON.Send(ws, candidate); wErr != nil {
				log.Printf("error writing answer: %v", wErr)
				return
			}
		case answerCandidate := <-readChan:
			call, err := calls.Get(caller.Id)
			if err != nil {
				log.Print("answer: call not found during trickle ice")
				return
			}
			// TODO: maybe dont send the empty candidate, so server is always the one to close the conn?
			call.To.CandidateChan <- answerCandidate
			fmt.Println("answer candidate sent to chan")
			if answerCandidate.Candidate == "" {
				log.Println("empty candidate recieved", answerCandidate)
				close(call.To.CandidateChan)
				return
			}
		}
	}
}

// ReadForever reads from ws in a loop, sending the data read to the channel ch.
// If the ws is closed or there is an error while reading, the ws is closed and the loop stops.
func ReadForever[T any](ws *websocket.Conn, data T, ch chan T) {
	var err error
	for {
		err = websocket.JSON.Receive(ws, &data)
		if err != nil {
			if err == io.EOF {
				return
			}
			log.Printf("error reading from ws: %v", err)
			if closeErr := ws.Close(); closeErr != nil {
				log.Printf("error closing ws: %v", closeErr)
			}
			return
		}
		ch <- data
	}
}
