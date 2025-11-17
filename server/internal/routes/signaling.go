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

	var offer schemas.CallRequest
	readErr := websocket.JSON.Receive(ws, &offer)
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

	// create the call in memory, with chans to facilitate signaling
	var (
		// TODO: with channel rooms, these chans will need to be per-client
		answerChan = make(chan webrtc.SessionDescription, 1)

		// the answerer will listen on this to recieve the callers ICE candidates
		iceSendChan = make(chan webrtc.ICECandidateInit, 15) // 15 max ice candidates should be enough?

		// we will listen on this to recieve answerer's ICE candidates as they trickle in
		iceRecvChan = make(chan webrtc.ICECandidateInit, 15)
	)
	call := schemas.CreateCall(caller, recipient, offer.Sd, iceSendChan, iceRecvChan, answerChan)
	calls := schemas.GetPendingCalls()
	defer calls.Delete(caller.Id)
	log.Println("call created")

	// read incoming candidates
	var (
		readWg   sync.WaitGroup
		readChan = make(chan webrtc.ICECandidateInit)
	)
	defer func() {
		// ws.Close unblocks the ws reads
		cErr := ws.Close()
		if cErr != nil {
			log.Println("error closing ws during defer: ", cErr)
		}
		readWg.Wait()
	}()
	readWg.Go(func() {
		readCandidates(ws, readChan)
	})

	// this request will be canceled by the client once the call is successful,
	// or it will timeout
	// TODO: add the timeout
	ctx := ws.Request().Context()
	for {
		select {
		case <-ctx.Done():
			log.Println("Call req context done")
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
			// we've sent the caller the answerer's last candidate. nothing left to do
			if !ok {
				return
			}
		// note: this must continue even if the above case completes. in the channel architecture, ensure this is the case?
		// or maybe even then, caller candidates will be present for the answerer so will always finish first
		case callerCandidate, ok := <-readChan:
			if !ok { // caller gather completed
				close(call.From.CandidateChan)
				readChan = nil
				continue
			}
			call.From.CandidateChan <- callerCandidate
			fmt.Println("caller candidate sent")
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

	// ensure the pending call exists
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
		log.Println("call not found")
		ws.WriteClose(http.StatusBadRequest)
		return
	}
	defer calls.Delete(caller.Id)

	// send caller's SD. client will then create an answer and post it to this ws
	if wErr := websocket.JSON.Send(ws, call.From.Sd); wErr != nil {
		log.Printf("error writing offer: %v", wErr)
		return
	}

	// wait for answer from client
	var answer schemas.AnswerRequest
	recvErr := websocket.JSON.Receive(ws, &answer)
	if recvErr != nil {
		log.Printf("error reading answer from ws: %v", recvErr)
		ws.WriteClose(http.StatusBadRequest)
		return
	}
	if answer.Sd.SDP == "" {
		log.Println("empty answer")
		ws.WriteClose(http.StatusBadRequest)
		return
	}
	log.Println("answerWS: answer recieved")
	call.AnswerChan <- answer.Sd

	// read incoming candidates
	var (
		readWg   sync.WaitGroup
		readChan = make(chan webrtc.ICECandidateInit)
	)
	defer func() {
		// ws.Close unblocks the ws reads
		cErr := ws.Close()
		if cErr != nil {
			log.Println("error closing ws during defer: ", cErr)
		}
		readWg.Wait()
	}()
	readWg.Go(func() {
		readCandidates(ws, readChan)
	})

	ctx := ws.Request().Context()
	for {
		select {
		case <-ctx.Done():
			log.Println("Answer req context done")
			return
		// note: this needs to continue to run even if readchan is closed. this may always complete first tho...
		case candidate, ok := <-call.From.CandidateChan:
			if !ok {
				call.From.CandidateChan = nil
			}
			if wErr := websocket.JSON.Send(ws, candidate); wErr != nil {
				log.Printf("error writing answer: %v", wErr)
				return
			}
		case answerCandidate, ok := <-readChan:
			if !ok { // answerer gather completed
				close(call.To.CandidateChan)
				// readChan = nil
				return
			}
			call, err := calls.Get(caller.Id)
			if err != nil {
				log.Print("answer: call not found during trickle ice")
				return
			}
			call.To.CandidateChan <- answerCandidate
			fmt.Println("answer candidate sent")
		}
	}
}

// readCandidates reads from ws in a loop, sending candidates read to the channel ch.
// When an empty candidate is read, the channel is closed, signalling that ICE gather on this
// websocket is finished. If the ws is closed or there is an error while reading, the ws is closed and the loop stops.
func readCandidates(ws *websocket.Conn, ch chan webrtc.ICECandidateInit) {
	var candidate webrtc.ICECandidateInit
	for {
		err := websocket.JSON.Receive(ws, &candidate)
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

		if candidate.Candidate == "" {
			close(ch)
			log.Println("ice gather completed")
			return
		}
		ch <- candidate
	}
}
