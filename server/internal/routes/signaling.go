package routes

import (
	"encoding/json"
	"fmt"
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
		iceSendChan = make(chan webrtc.ICECandidateInit)

		// we will listen on this to recieve answerer's ICE candidates as they trickle in
		iceRecvChan = make(chan webrtc.ICECandidateInit)

		// for websocket reading ////////////////////////
		buf        = make([]byte, 1500)
		mu         sync.Mutex // TODO: this won't be needed once we send the codes msgs over the readChan
		n          int        // bytes read per message
		readErr    error
		readWg     sync.WaitGroup
		wsReadChan = make(chan struct{})

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

	readWg.Go(func() {
		for {
			mu.Lock()
			// TODO: handle when this unblocks due to ws closing
			// TODO: could define a codec, read like this, send to chan, use a type switch on it for cleaner message parsing
			// var data any
			// websocket.JSON.Receive(ws, data)
			n, readErr = ws.Read(buf)
			mu.Unlock()
			if readErr != nil {
				log.Printf("error reading from ws: %v", readErr)
			}
			wsReadChan <- struct{}{}
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
		case candidate := <-iceRecvChan:
			if wErr := wsWrite(ws, candidate); wErr != nil {
				log.Printf("error writing candidate: %v", wErr)
				return
			}
		case <-wsReadChan:
			mu.Lock()
			switch {
			// Attempt to unmarshal as a SessionDescription. If the SDP field is empty
			// assume it is not one.
			case json.Unmarshal(buf[:n], &offer) == nil && offer.Sd.SDP != "":
				log.Println("callWS: offer recieved")
				recipient, sqlErr := dal.GetUserByUsername(h.db, offer.RecipientName)
				if sqlErr != nil {
					log.Println(fmt.Errorf("error fetching recipient: %w", sqlErr))
					ws.WriteClose(http.StatusBadRequest)
					mu.Unlock()
					return
				}
				schemas.CreateCall(caller, recipient, offer.Sd, iceSendChan, iceRecvChan, answerChan)
				log.Println("call created")
			// Attempt to unmarshal as a ICECandidateInit. If the candidate field is empty
			// assume it is not one.
			case json.Unmarshal(buf[:n], &candidate) == nil && candidate.Candidate != "":
				call, err := calls.Get(caller.Id)
				if err != nil {
					log.Print("call not found during trickle ice")
					mu.Unlock()
					return
				}
				fmt.Println("caller candidate sending to chan")
				call.From.CandidateChan <- candidate
			default:
				log.Print("unknown message")
				mu.Unlock()
				return
			}
			mu.Unlock()
		}
	}
}

// Call is used by a client to create a call. The request contains the caller's offer, after
// their ICE gathering has fully completed.
// func (h *RouteHandler) Call(w http.ResponseWriter, req *http.Request) {
// 	rData := schemas.CallRequest{}
// 	if err := json.NewDecoder(req.Body).Decode(&rData); err != nil {
// 		panic(err)
// 	}

// 	var (
// 		caller,
// 		recipient *schemas.User
// 		sqlErr error
// 	)
// 	username := middleware.GetUsername(req)
// 	if caller, sqlErr = dal.GetUserByUsername(h.db, username); sqlErr != nil {
// 		log.Println(fmt.Errorf("error fetching caller: %w", sqlErr))
// 		http.Error(w, "error finding caller", http.StatusInternalServerError)
// 		return
// 	}
// 	if recipient, sqlErr = dal.GetUserByUsername(h.db, rData.RecipientName); sqlErr != nil {
// 		log.Println(fmt.Errorf("error fetching recipient: %w", sqlErr))
// 		http.Error(w, "error finding recipient", http.StatusBadRequest)
// 		return
// 	}

// 	log.Println("waiting for call to be answered")
// 	select {
// 	case <-req.Context().Done():
// 		log.Println("call request cancelled by client")
// 		break // allow cleanup below to run if request is cancelled
// 	case answererSd := <-schemas.CreateCallAndNotify(*caller, *recipient, rData.Sd):
// 		log.Println("call has been answered")
// 		WriteJSON(w, answererSd)

// 	}
// 	calls := schemas.GetPendingCalls()
// 	calls.Delete(caller.Id)
// }

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

	// for websocket reading
	// var (
	// 	buf     = make([]byte, 1500)
	// 	n       int // bytes read per message
	// 	readErr error

	// 	// websocket message
	// 	// callerName string
	// )
	// n, rErr := ws.Read(buf)
	// if rErr != nil {
	// 	log.Println(rErr)
	// }
	// jsonErr := json.Unmarshal(buf[:n], &callerName)
	// if jsonErr != nil || callerName == "" {
	// log.Println("caller name invalid: ", callerName)
	// ws.WriteClose(http.StatusBadRequest)
	// return
	// }

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
		buf        = make([]byte, 1500)
		n          int // bytes read per message
		readErr    error
		mu         sync.Mutex
		readWg     sync.WaitGroup
		wsReadChan = make(chan struct{})

		// websocket messages
		candidate webrtc.ICECandidateInit
		answer    schemas.AnswerRequest
	)
	defer func() {
		ws.Close()
		readWg.Wait()
		log.Println("readWg closed")
	}()

	readWg.Go(func() {
		for {
			mu.Lock()
			// TODO: handle when this unblocks due to ws closing
			n, readErr = ws.Read(buf)
			mu.Unlock()
			if readErr != nil {
				log.Printf("error reading from ws: %v", readErr)
				return
			}
			wsReadChan <- struct{}{}
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
		case <-wsReadChan:
			mu.Lock()
			// todo: if answer, send to answerchan
			switch {
			case json.Unmarshal(buf[:n], &answer) == nil && answer.Sd.SDP != "":
				log.Println("answerWS: answer recieved")
				caller, sqlErr := dal.GetUserByUsername(h.db, answer.CallerName)
				if sqlErr != nil {
					log.Println(fmt.Errorf("error fetching caller: %w", sqlErr))
					ws.WriteClose(http.StatusInternalServerError)
					mu.Unlock()
					return
				}
				call, err := calls.Get(caller.Id)
				if err != nil {
					log.Printf("call not found during answer: %v", err)
					mu.Unlock()
					return
				}
				fmt.Println("answer sending to chan")
				call.AnswerChan <- answer.Sd
			case json.Unmarshal(buf[:n], &candidate) == nil && candidate.Candidate != "":
				call, err := calls.Get(caller.Id)
				if err != nil {
					log.Print("call not found during trickle ice")
					mu.Unlock()
					return
				}
				fmt.Println("caller candidate sending to chan")
				call.To.CandidateChan <- candidate
			default:
				log.Print("unknown message")
				mu.Unlock()
				return
			}
			mu.Unlock()
		}
	}
}

// Caller is a GET endpoint that the answerer calls when they want the SD of the caller.
// This returns to the answerer the caller's offer, with all ICE candidates present
// func (h *RouteHandler) Caller(w http.ResponseWriter, req *http.Request) {
// 	callerName := req.PathValue("name")

// 	log.Println("caller name: ", callerName)

// 	var (
// 		caller *schemas.User
// 		sqlErr error
// 	)
// 	username := middleware.GetUsername(req)
// 	if _, sqlErr = dal.GetUserByUsername(h.db, username); sqlErr != nil {
// 		log.Println(fmt.Errorf("error fetching recipient: %w", sqlErr))
// 		http.Error(w, "error finding recipient", http.StatusInternalServerError)
// 		return
// 	}
// 	if caller, sqlErr = dal.GetUserByUsername(h.db, callerName); sqlErr != nil {
// 		log.Println(fmt.Errorf("error fetching caller: %w", sqlErr))
// 		http.Error(w, "error finding caller", http.StatusBadRequest)
// 		return
// 	}

// 	calls := schemas.GetPendingCalls()
// 	call, err := calls.Get(caller.Id)
// 	if err != nil {
// 		http.Error(w, err.Error(), http.StatusBadRequest)
// 		return
// 	}

// 	// reply with caller's Sd. client will then use that to resume ICE
// 	WriteJSON(w, &call.From.Sd)
// }

// Answer is used by a client to accept a pending call requested by another client. It sends the caller the
// answerer's answer (with all ICE candidates)
// func (h *RouteHandler) Answer(w http.ResponseWriter, req *http.Request) {
// 	aData := schemas.AnswerRequest{}
// 	if err := json.NewDecoder(req.Body).Decode(&aData); err != nil {
// 		panic(err)
// 	}

// 	var (
// 		caller *schemas.User
// 		sqlErr error
// 	)
// 	username := middleware.GetUsername(req)
// 	if _, sqlErr = dal.GetUserByUsername(h.db, username); sqlErr != nil {
// 		log.Println(fmt.Errorf("error fetching recipient: %w", sqlErr))
// 		http.Error(w, "error finding recipient", http.StatusBadRequest)
// 		return
// 	}
// 	if caller, sqlErr = dal.GetUserByUsername(h.db, aData.CallerName); sqlErr != nil {
// 		log.Println(fmt.Errorf("error fetching caller: %w", sqlErr))
// 		http.Error(w, "error finding caller", http.StatusInternalServerError)
// 		return
// 	}

// 	if answerErr := schemas.AnswerCall(*caller, aData.Sd); answerErr != nil {
// 		http.Error(w, answerErr.Error(), http.StatusBadRequest)
// 		return
// 	}
// 	w.WriteHeader(200)
// }

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
