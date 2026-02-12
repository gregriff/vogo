package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gregriff/vogo/server/internal/dal"
	"github.com/gregriff/vogo/server/internal/middleware"
	"github.com/gregriff/vogo/server/internal/schemas"
	"github.com/gregriff/vogo/server/internal/state"
	"github.com/pion/webrtc/v4"
	"golang.org/x/net/websocket"
)

// TODO:
// PUT /channel: modify channel properties
// DELETE /channel

// Call initiates signaling for a voice call that may only be accepted by the intended recipient. The caller's
// ICE candidates are stored in memory until the recipient answers, where they are then forwarded. Call then
// recieves the recipient's ICE candidates and forwards them to the caller. When candidates have been fully
// exchanged Call deletes the signaling data from memory and returns.
func (h *RouteHandler) Call(ws *websocket.Conn) {
	ctx, cancel := context.WithTimeout(ws.Request().Context(), time.Second*30)
	defer cancel()

	username := middleware.GetUsernameWS(ws)
	caller, err := dal.GetUser(h.db, username)
	if err != nil {
		log.Println(fmt.Errorf("error fetching caller: %w", err))
		_ = ws.WriteClose(http.StatusInternalServerError)
		return
	}

	var offer schemas.CallRequest
	err = receiveWithContext(ctx, ws, &offer)
	if err != nil {
		if err == io.EOF {
			return
		}
		log.Printf("error reading offer from ws: %v", err)
		_ = ws.WriteClose(http.StatusBadRequest)
		return
	}
	if offer.Sd.SDP == "" {
		log.Println("empty offer")
		_ = ws.WriteClose(http.StatusBadRequest)
		return
	}
	log.Println("callWS: offer recieved")
	recipient, err := dal.GetUser(h.db, offer.RecipientName)
	if err != nil {
		log.Println(fmt.Errorf("error fetching recipient: %w", err))
		_ = ws.WriteClose(http.StatusBadRequest)
		return
	}

	friends, err := dal.AreFriends(h.db, caller.Id, recipient.Id)
	if err != nil {
		log.Println(fmt.Errorf("error checking friendship status: %w", err))
		_ = ws.WriteClose(http.StatusInternalServerError)
		return
	}
	if !friends {
		log.Println(fmt.Errorf("caller not friends with recipient: %w", err))
		_ = ws.WriteClose(http.StatusBadRequest)
		return
	}

	// create the call in memory, delete once answered
	call := state.CreateConnection(caller, recipient, offer.Sd)
	calls := state.GetPendingCalls()
	// add this call to pending map, using caller's ID since a client can only make one call at a time
	calls.Add(caller.Id, call)
	defer calls.Delete(caller.Id)
	log.Println("call created")

	// read incoming candidates
	var (
		readIce                   sync.WaitGroup
		readIceCtx, cancelReadIce = context.WithCancel(ctx)
		readChan                  = make(chan webrtc.ICECandidateInit)
		canListenForClose         = make(chan struct{}, 1)
	)
	defer func() {
		cancelReadIce()
		_ = ws.Close()
		readIce.Wait()
	}()
	readIce.Go(func() {
		defer close(canListenForClose)
		defer cancelReadIce()
		err := readCandidates(readIceCtx, ws, readChan)
		if err != nil {
			if err == io.EOF {
				cancel()
				return
			}
			log.Println("error during ice reading: ", err)
		}
		canListenForClose <- struct{}{}
	})

	// listen for the close frame from the client, since we know the only
	// thing the client could possibly send after ICE gather is a close frame
	var (
		listen sync.WaitGroup
		closed = make(chan struct{}, 1)
	)
	defer listen.Wait()
	listen.Go(func() {
		// wait for ice gather to complete
		if _, ok := <-canListenForClose; !ok {
			return
		}
		err := receiveWithContext(ctx, ws, &struct{}{})
		if err == io.EOF {
			log.Println("listenForClose EOF")
			closed <- struct{}{}
		} else if err != nil {
			log.Println("listenForClose NON EOF ERR: ", err)
		}
	})

	for {
		select {
		case <-ctx.Done():
		case <-closed:
			log.Println("Call req context done or conn closed")
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
	recipient, err := dal.GetUser(h.db, username)
	if err != nil {
		log.Println(fmt.Errorf("error fetching recipient: %w", err))
		_ = ws.WriteClose(http.StatusInternalServerError)
		return
	}

	// ensure the pending call exists
	callerName := ws.Request().PathValue("name")
	caller, err := dal.GetUser(h.db, callerName)
	if err != nil {
		log.Println(fmt.Errorf("error fetching caller: %w", err))
		_ = ws.WriteClose(http.StatusBadRequest)
		return
	}
	friends, err := dal.AreFriends(h.db, caller.Id, recipient.Id)
	if err != nil {
		log.Println(fmt.Errorf("error checking friendship status: %w", err))
		_ = ws.WriteClose(http.StatusInternalServerError)
		return
	}
	if !friends {
		log.Println(fmt.Errorf("recipient not friends with caller: %w", err))
		_ = ws.WriteClose(http.StatusBadRequest)
		return
	}

	calls := state.GetPendingCalls()
	call, err := calls.Get(caller.Id)
	if err != nil {
		log.Println("call not found")
		_ = ws.WriteClose(http.StatusBadRequest)
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
		if err == io.EOF {
			return
		}
		log.Printf("error reading answer from ws: %v", err)
		_ = ws.WriteClose(http.StatusBadRequest)
		return
	}
	if answer.Sd.SDP == "" {
		log.Println("empty answer")
		_ = ws.WriteClose(http.StatusBadRequest)
		return
	}
	log.Println("answerWS: answer recieved")
	call.Answer <- answer.Sd

	// read incoming candidates
	var (
		readIce                   sync.WaitGroup
		readIceCtx, cancelReadIce = context.WithCancel(ctx)
		readChan                  = make(chan webrtc.ICECandidateInit)
		canListenForClose         = make(chan struct{}, 1)
	)
	defer func() {
		cancelReadIce()
		_ = ws.Close()
		readIce.Wait()
	}()
	readIce.Go(func() {
		defer close(canListenForClose)
		defer cancelReadIce()
		err := readCandidates(readIceCtx, ws, readChan)
		if err != nil {
			if err == io.EOF {
				cancel()
				return
			}
			log.Println("error during ice reading: ", err)
		}
		canListenForClose <- struct{}{} // unness?
	})

	// listen for the close frame from the client, since we know the only
	// thing the client could possibly send after ICE gather is a close frame
	var (
		listen sync.WaitGroup
		closed = make(chan struct{}, 1)
	)
	defer listen.Wait()
	listen.Go(func() {
		// wait for ice gather to complete
		if _, ok := <-canListenForClose; !ok {
			return
		}
		err := receiveWithContext(ctx, ws, &struct{}{})
		if err == io.EOF {
			log.Println("listenForClose EOF")
			closed <- struct{}{}
		} else if err != nil {
			log.Println("listenForClose NON EOF ERR: ", err)
		}
	})

	for {
		select {
		case <-ctx.Done():
		case <-closed:
			log.Println("Answer req context done or conn closed")
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
				return err // ws closed, propogate up
			}
			if cErr := ws.Close(); cErr != nil {
				return fmt.Errorf("error closing ws: %v, after err: %v", cErr, err)
			}
			return err
		}

		if candidate.Candidate == "" {
			close(ch)
			log.Println("ice gather completed")
			return nil
		}
		ch <- candidate
	}
}

// JoinRoom lets a user join a room they are a member of, given its name and owner's name,
// and creates the in-memory representation of that room if no members are currently connected
// to it. This is a websocket endpoint that will stay open until the user leaves the room call
// or is disconnected. When another member joins the room, this endpoint will send their Sd to
// the user, to facilitate the webrtc signalling for all members connected to the room.
func (h *RouteHandler) JoinRoom(ws *websocket.Conn) {
	ctx, cancel := context.WithCancel(ws.Request().Context())
	defer cancel()

	username := middleware.GetUsernameWS(ws)
	user, err := dal.GetUser(h.db, username)
	if err != nil {
		log.Println(fmt.Errorf("error fetching user: %w", err))
		_ = ws.WriteClose(http.StatusInternalServerError)
		return
	}

	var req schemas.JoinRoomRequest
	err = receiveWithContext(ctx, ws, &req)
	if err != nil {
		if err == io.EOF {
			return
		}
		log.Printf("error reading offer from ws: %v", err)
		_ = ws.WriteClose(http.StatusBadRequest)
		return
	}
	log.Println("joinRoom: req recieved")
	owner, err := dal.GetUser(h.db, req.OwnerName)
	if err != nil {
		log.Println(fmt.Errorf("error fetching room owner: %w", err))
		_ = ws.WriteClose(http.StatusBadRequest)
		return
	}

	// add areFriends check? TODO: removeFriend and blockFriend endpoints should remove user
	// from relevant rooms in the same DB transaction
	c, err := dal.GetChannelOfMember(h.db, req.RoomName, user.Id, owner.Id)
	if err != nil {
		log.Println(fmt.Errorf("error getting channel of member: %w", err))
		_ = ws.WriteClose(http.StatusInternalServerError)
		return
	}
	c.Owner = owner.Name

	// create or join room
	roomUser := state.NewRoomUser(user)
	room, err := state.CreateOrJoinRoom(c, roomUser)
	if err != nil {
		log.Println(fmt.Errorf("error creating or joining room: %w", err))
		_ = ws.WriteClose(http.StatusInternalServerError)
		return
	}
	log.Println("room created")
	defer room.Leave(roomUser)

	users, checkedAt := room.Users(roomUser.Id)
	newConnections := state.BulkConnectionMessage{
		Usernames: make([]string, 0, state.MaxRoomUsers-1),
	}
	for _, user := range users {
		newConnections.Usernames = append(newConnections.Usernames, user.Name)
	}
	if err := websocket.JSON.Send(ws, newConnections); err != nil {
		log.Printf("error writing NewConnections msg: %v", err)
		return
	}

	// note: parallelize this
	var offers schemas.BulkConnectionMessage
	err = receiveWithContext(ctx, ws, &offers)
	if err != nil {
		if err == io.EOF {
			return
		}
		log.Printf("error reading offers from ws: %v", err)
		_ = ws.WriteClose(http.StatusBadRequest)
		return
	}
	if len(offers.Data) == 0 {
		log.Println("no offers recieved from ws")
		_ = ws.WriteClose(http.StatusBadRequest)
		return
	}

	// note: be careful that you dont send a newConnectionMsg for a user that you then get a NewUser event for and then resend...

	var (
		connectWg                 sync.WaitGroup
		connectCtx, cancelConnect = context.WithCancel(ctx)
	)
	defer func() {
		cancelConnect()
		connectWg.Wait()
	}()

	// note: parallelize this
	for recipientName, offer := range offers.Data {
		connectWg.Go(func() {
			ctx, cancel := context.WithTimeout(connectCtx, 15*time.Second)
			defer cancel()

			recipient, err := dal.GetUser(h.db, recipientName)
			if err != nil {
				log.Println(fmt.Errorf("error fetching recipient: %w", err))
				return
			}

			// TODO: ACTUALLY SEND OFFER EVENT SO ANSWERER KNOWS
			log.Println("NEED TO ACTUALLY SEND OFFER EVENT")
			conn := state.CreateConnection(user, recipient, offer)
			roomUser.PendingConnections.Add(recipient.Id, conn)
			defer roomUser.PendingConnections.Delete(recipient.Id)
			log.Printf("conn created, signalling beginning with %s", recipient.Name)

			for {
				select {
				case <-ctx.Done():
					return
				case answerSd := <-conn.Answer:
					msg := schemas.AnswerNotificationMessage{
						RecipientName: recipient.Name,
						Sd:            answerSd,
					}
					if err := websocket.JSON.Send(ws, msg); err != nil {
						log.Printf("error writing answer: %v", err)
						return
					}
				case answerCandidate, ok := <-conn.To.Candidates:
					msg := schemas.CandidateMessage{
						UserId:    recipient.Id,
						Username:  recipient.Name,
						Candidate: answerCandidate,
					}
					if err := websocket.JSON.Send(ws, msg); err != nil {
						log.Printf("error writing candidate: %v", err)
						return
					}
					// we've sent the caller the recipient's last candidate. nothing left to do
					if !ok {
						conn.To.Candidates = nil // unness
						return
					}
				}
			}
		})
	}

	var (
		wsRecvWg                sync.WaitGroup
		wsRecvCtx, cancelWsRecv = context.WithCancel(ctx)
		msgChan                 = make(chan Message)
	)
	defer func() {
		cancelWsRecv()
		wsRecvWg.Wait()
	}()
	wsRecvWg.Go(func() {
		defer cancel() // if websocket closes, end all goroutines
		if err := startMessageLoop(wsRecvCtx, ws, msgChan); err != nil {
			if err == io.EOF { // todo: may need to handle this in startMessageLoop
				log.Println("messageLoop EOF")
			} else {
				log.Printf("messageLoop other err: %v", err)
			}
			_ = ws.WriteClose(http.StatusInternalServerError)
		}
	})

	// now that the offers are being sent to existing users, we can start the loop that will run for the rest of the time
	// that the user is in the room.
	// 1. Listen for new users. if one, listen for their offer, then prepare an answer
	// NOTE: could simplify the event system with just one channel per room user to notify them of incoming offers.

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	defer func() {
		_ = ws.Close()
	}()

	// this is logic that needs to run for the duration of the session/ws.
	// listen for room events. when another user joins, this logic must begin signalling with that user
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			log.Println("tick")
		// NOTE: all funcs in this need to run async, since chan is unbuffered and blocks on sends
		case msg := <-msgChan:
			switch msg.Type {
			case "ice-offer":
				var data schemas.CandidateMessage
				json.Unmarshal(msg.Data, &data)

				conn, err := roomUser.PendingConnections.Get(data.UserId)
				if err != nil {
					log.Printf("unable to get conn for %s in ice-offer handler: ", data.Username)
					_ = ws.WriteClose(http.StatusInternalServerError)
					return
				}
				if data.Candidate.Candidate == "" {
					close(conn.From.Candidates)
					log.Println("caller ice gather completed")
					break
				}
				conn.From.Candidates <- data.Candidate
			case "ice-answer":
				var data schemas.CandidateMessage
				json.Unmarshal(msg.Data, &data)

				caller := room.GetUser(data.UserId)
				if caller == nil {
					log.Printf("caller %s not found in room while handling ice-answer message", data.Username)
					// _ = ws.WriteClose(http.StatusBadRequest)
					break
				}
				conn, err := caller.PendingConnections.Get(roomUser.Id)
				if err != nil {
					log.Printf("unable to get conn for %s in ice-answer handler: ", data.Username)
					// _ = ws.WriteClose(http.StatusBadRequest)
					break
				}
				if data.Candidate.Candidate == "" {
					close(conn.To.Candidates)
					log.Println("answerer ice gather completed")
					break
				}
				conn.To.Candidates <- data.Candidate
			case "answer":
				var data schemas.AnswerRoomUserRequest
				json.Unmarshal(msg.Data, &data)

				if data.Sd.SDP == "" {
					log.Println("empty answer")
					// _ = ws.WriteClose(http.StatusBadRequest)
					break
				}
				log.Println("answer handler: answer recieved")

				caller := room.GetUser(data.CallerId)
				if caller == nil {
					log.Printf("caller %s not found in room while handling answer message", data.CallerName)
					// _ = ws.WriteClose(http.StatusBadRequest)
					break
				}
				conn, err := caller.PendingConnections.Get(roomUser.Id)
				if err != nil {
					log.Printf("unable to get conn for %s in answer handler: ", data.CallerName)
					// _ = ws.WriteClose(http.StatusBadRequest)
					break
				}
				conn.Answer <- data.Sd

				var (
					sendIce                   sync.WaitGroup
					sendIceCtx, cancelSendIce = context.WithTimeout(ctx, 30*time.Second)
				)
				defer func() {
					cancelSendIce()
					sendIce.Wait()
				}()
				sendIce.Go(func() {
					defer cancelSendIce()
					for {
						select {
						case <-sendIceCtx.Done():
							log.Println("answer handler sendICE ctx cancelled, stopping listening for caller candidates")
							return
						// forwards caller's candidates to answerer
						case candidate, ok := <-conn.From.Candidates:
							msg := schemas.CandidateMessage{
								UserId:    caller.Id,
								Username:  caller.Name,
								Candidate: candidate,
							}
							if err := websocket.JSON.Send(ws, msg); err != nil {
								log.Printf("error writing caller candidate to answerer ws: %v", err)
								return
							}
							if !ok { // empty end candidate sent, return
								conn.From.Candidates = nil
								return
							}
						}
					}
				})
			}
		case event := <-roomUser.Events:
			fmt.Printf("user %s recieved event [type=%s, user=%s]", roomUser.Name, event.Type, event.User.Name)
			if event.CreatedAt.Before(checkedAt) {
				fmt.Printf("EVENT %s happened before we got list of current users, disregarding...", event.Type)
				continue
			}
			switch event.Type {
			case state.JoinEvent:
				_ = 0
			case state.OfferEvent:
				conn, err := event.User.PendingConnections.Get(user.Id)
				if err != nil {
					log.Printf("error getting offer from newly joined user: %v", err)
					break
				}

				msg := schemas.AnswerConnectionMessage{
					CallerName: event.User.Name,
					Sd:         conn.From.Sd,
				}
				if err := websocket.JSON.Send(ws, msg); err != nil {
					log.Printf("error writing AnswerConnectionMessage to %s: %v", event.User.Name, err)
					break
				}
			case state.ExitEvent:
				_ = 0
			default:
				log.Printf("ERROR: unhandled event: %s", event.Type)
			}
		}
	}
}
