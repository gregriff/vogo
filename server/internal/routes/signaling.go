package routes

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gregriff/vogo/server/internal/dal"
	"github.com/gregriff/vogo/server/internal/middleware"
	"github.com/gregriff/vogo/server/internal/schemas"
	"github.com/pion/webrtc/v4"
	"golang.org/x/net/websocket"
)

// TODO:
// GET /status: returns to the client any calls or channels currently open and associated with the client
// POST /channel: allow a client to open a channel. this is persisted to sqlite, including who is invited to join it.
// 				  channels are given a unique name, where the CLI can change properties (who's invited) using the PUT
// PUT /channel: modify channel properties
// DELETE /channel

// Call initiates signaling for a voice call that may only be accepted by the intended recipient. The caller's
// ICE candidates are stored in memory until the recipient answers, where they are then forwarded. Call then
// recieves the recipient's ICE candidates and forwards them to the caller. When candidates have been fully
// exchanged Call deletes the signaling data from memory and returns.
// Note: the channel version of this func will need to stay open until the client exits the channel call.
func (h *RouteHandler) Call(ws *websocket.Conn) {
	ctx, cancel := context.WithTimeout(ws.Request().Context(), time.Second*30)
	defer cancel()

	username := middleware.GetUsernameWS(ws)
	caller, err := dal.GetUser(h.db, username)
	if err != nil {
		log.Println(fmt.Errorf("error fetching caller: %w", err))
		ws.WriteClose(http.StatusInternalServerError)
		return
	}

	var offer schemas.CallRequest
	err = receiveWithContext(ctx, ws, &offer)
	if err != nil {
		log.Printf("error reading offer from ws: %v", err)
		ws.WriteClose(http.StatusBadRequest)
		return
	}
	if offer.Sd.SDP == "" {
		log.Println("empty offer")
		ws.WriteClose(http.StatusBadRequest)
		return
	}
	log.Println("callWS: offer recieved")
	recipient, err := dal.GetUser(h.db, offer.RecipientName)
	if err != nil {
		log.Println(fmt.Errorf("error fetching recipient: %w", err))
		ws.WriteClose(http.StatusBadRequest)
		return
	}

	// create the call in memory, delete once answered
	call := schemas.CreateCall(caller, recipient, offer.Sd)
	calls := schemas.GetPendingCalls()
	defer calls.Delete(caller.Id)
	log.Println("call created")

	// read incoming candidates
	var (
		readIce                   sync.WaitGroup
		readChan                  = make(chan webrtc.ICECandidateInit)
		readIceCtx, cancelReadIce = context.WithCancel(ctx)
	)
	defer func() {
		cancelReadIce()
		if err := ws.Close(); err != nil {
			log.Println("error closing ws during defer: ", err)
		}
		readIce.Wait()
	}()
	readIce.Go(func() {
		defer cancelReadIce()
		err := readCandidates(readIceCtx, ws, readChan)
		if err != nil {
			log.Println("error during ice reading: ", err)
		}
	})

	// TODO: could run another recvWithContext to listen for the close frame from the client,
	// since we know the only thing the clinet could possibly send at this point is a close frame

	for {
		select {
		case <-ctx.Done():
			log.Println("Call req context done")
			cancel()
			return
		case answerSd := <-call.Answer:
			if err := websocket.JSON.Send(ws, answerSd); err != nil {
				log.Printf("error writing answer: %v", err)
				return
			}
		case answerCandidate, ok := <-call.To.Candidates:
			if err := websocket.JSON.Send(ws, answerCandidate); err != nil {
				log.Printf("error writing candidate: %v", err)
				return
			}
			// we've sent the caller the recipient's last candidate. nothing left to do
			if !ok {
				return
			}
		// note: this must continue even if the above case completes. in the channel architecture, ensure this is the case?
		// or maybe even then, caller candidates will be present for the recipient so will always finish first
		case callerCandidate, ok := <-readChan:
			if !ok { // caller gather completed
				close(call.From.Candidates)
				readChan = nil
				continue
			}
			call.From.Candidates <- callerCandidate
			fmt.Println("caller candidate sent")
		}
	}
}

// Answer obtains the caller's name from the first ws message and sends the caller's offer Sd to the client.
// It then waits for the clients answer, where it then facilitates trickle-ICE gathering between the two clients.
func (h *RouteHandler) Answer(ws *websocket.Conn) {
	ctx, cancel := context.WithTimeout(ws.Request().Context(), time.Second*15)
	defer cancel()

	username := middleware.GetUsernameWS(ws)
	_, err := dal.GetUser(h.db, username)
	if err != nil {
		log.Println(fmt.Errorf("error fetching recipient: %w", err))
		ws.WriteClose(http.StatusInternalServerError)
		return
	}

	// ensure the pending call exists
	callerName := ws.Request().PathValue("name")
	caller, err := dal.GetUser(h.db, callerName)
	if err != nil {
		log.Println(fmt.Errorf("error fetching caller: %w", err))
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
	if err := websocket.JSON.Send(ws, call.From.Sd); err != nil {
		log.Printf("error writing offer: %v", err)
		return
	}

	// wait for answer from client
	var answer schemas.AnswerRequest
	err = receiveWithContext(ctx, ws, &answer)
	if err != nil {
		log.Printf("error reading answer from ws: %v", err)
		ws.WriteClose(http.StatusBadRequest)
		return
	}
	if answer.Sd.SDP == "" {
		log.Println("empty answer")
		ws.WriteClose(http.StatusBadRequest)
		return
	}
	log.Println("answerWS: answer recieved")
	call.Answer <- answer.Sd

	// read incoming candidates
	var (
		readIce                   sync.WaitGroup
		readChan                  = make(chan webrtc.ICECandidateInit)
		readIceCtx, cancelReadIce = context.WithCancel(ctx)
	)
	defer func() {
		cancelReadIce()
		if err := ws.Close(); err != nil {
			log.Println("error closing ws during defer: ", err)
		}
		readIce.Wait()
	}()
	readIce.Go(func() {
		defer cancelReadIce()
		err := readCandidates(readIceCtx, ws, readChan)
		if err != nil {
			log.Println("error during ice reading: ", err)
		}
	})

	for {
		select {
		case <-ctx.Done():
			log.Println("Answer req context done")
			cancel()
			return
		// note: this needs to continue to run even if readchan is closed. this may always complete first tho...
		case candidate, ok := <-call.From.Candidates:
			if !ok {
				call.From.Candidates = nil
			}
			if err := websocket.JSON.Send(ws, candidate); err != nil {
				log.Printf("error writing answer: %v", err)
				return
			}
		case answerCandidate, ok := <-readChan:
			if !ok { // recipient gather completed
				close(call.To.Candidates)
				return
			}
			call, err := calls.Get(caller.Id)
			if err != nil {
				log.Print("answer: call not found during trickle ice")
				return
			}
			call.To.Candidates <- answerCandidate
			fmt.Println("answer candidate sent")
		}
	}
}

// readCandidates reads from ws in a loop, sending candidates read to the channel ch.
// When an empty candidate is read, the channel is closed, signalling that ICE gather on this
// websocket is finished. If the ws is closed or there is an error while reading, the ws is closed and the loop stops.
func readCandidates(ctx context.Context, ws *websocket.Conn, ch chan webrtc.ICECandidateInit) error {
	var candidate webrtc.ICECandidateInit
	for {
		if err := receiveWithContext(ctx, ws, &candidate); err != nil {
			if err == io.EOF {
				return fmt.Errorf("EOF during ws read")
			}
			log.Printf("error reading from ws: %v", err)
			if err := ws.Close(); err != nil {
				return fmt.Errorf("error closing ws: %v", err)
			}
			return nil
		}

		if candidate.Candidate == "" {
			close(ch)
			log.Println("ice gather completed")
			return nil
		}
		ch <- candidate
	}
}

// receiveWithContext reads json into v from ws in a new goroutine and cancels
// the read if ctx is cancelled. Param v should be a pointer.
func receiveWithContext(ctx context.Context, ws *websocket.Conn, v any) error {
	var (
		recv sync.WaitGroup
		done = make(chan error, 1)
	)
	defer recv.Wait()

	recv.Go(func() {
		done <- websocket.JSON.Receive(ws, v)
	})

	select {
	case <-ctx.Done():
		ws.SetReadDeadline(time.Now()) // interrupt the read
		return ctx.Err()
	case err := <-done:
		return err
	}
}
