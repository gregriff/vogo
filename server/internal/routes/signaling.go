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

	"github.com/google/uuid"
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

	var offer schemas.ConnectionRequest
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
	recipient, err := dal.GetUser(h.db, offer.To)
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
	call := state.CreateConnection(*caller, *recipient, offer.Sd)
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
	var answer schemas.ConnectionRequest
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
	defer room.Leave(roomUser)

	users := room.Users(roomUser.Id)
	// own func
	newConns := state.BulkConnectionRequest{
		Users: make(map[uuid.UUID]string, state.MaxRoomUsers-1),
	}
	for id, user := range users {
		newConns.Users[id] = user.Name
	}
	if err := websocket.JSON.Send(ws, newConns); err != nil {
		log.Printf("error writing NewConnections msg: %v", err)
		return
	}
	// ^^

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
	if len(offers.Data) == 0 && len(users) != 0 {
		log.Println("no offers recieved from ws")
		_ = ws.WriteClose(http.StatusBadRequest)
		return
	}

	var (
		connectWg                 sync.WaitGroup
		connectCtx, cancelConnect = context.WithCancel(ctx)
	)
	defer func() {
		cancelConnect()
		connectWg.Wait()
	}()

	// note: pull out into own func, then later parallelize
	for recipientId, req := range offers.Data {
		connectWg.Go(func() {
			ctx, cancel := context.WithTimeout(connectCtx, 15*time.Second)
			defer cancel()

			if _, ok := users[recipientId]; !ok {
				log.Printf("warn! client wants to send an offer to %s, who is not in the room!", req.To)
				return
			}

			recipient := schemas.User{
				Id:   recipientId,
				Name: users[recipientId].Name,
			}
			conn := state.CreateConnection(*user, recipient, req.Sd)
			roomUser.PendingConnections.Add(recipientId, conn)
			defer roomUser.PendingConnections.Delete(recipientId)
			// note: its important to ensure that none of the below goroutines never die and
			// keep holding references to this connection^^ (could set it to nil to debug?)

			offer := schemas.ConnectionRequestWithId{
				ConnectionRequest: schemas.ConnectionRequest{
					From: user.Name, // this is the caller's name
					To:   recipient.Name,
					Sd:   conn.From.Sd,
				},
				FromId: user.Id,
				ToId:   recipientId,
			}
			users[recipientId].Offers <- offer
			log.Printf("%s created conn, signalling beginning with %s", username, req.To)

			// own func
			for {
				select {
				case <-ctx.Done():
					return
				case answerSd, ok := <-conn.Answer:
					if !ok {
						conn.Answer = nil
						continue
					}
					log.Printf("%s received %s's answer from chan\n", username, req.To)
					bytes, err := json.Marshal(schemas.ConnectionRequest{
						To: req.To, // this is the recipient
						Sd: answerSd,
					})
					if err != nil {
						log.Printf("error encoding candidate: %v", err)
						return
					}

					msg := Message{Type: "answer", Data: bytes}
					if err := websocket.JSON.Send(ws, msg); err != nil {
						log.Printf("error writing answer: %v", err)
						return
					}
					log.Printf("%s's answer sent to %s\n", req.To, username)
				case answerCandidate, ok := <-conn.To.Candidates:
					bytes, err := json.Marshal(schemas.CandidateMessage{
						UserId:    recipientId,
						Username:  req.To, // this is the recipient
						Candidate: answerCandidate,
					})
					if err != nil {
						log.Printf("error encoding candidate: %v", err)
						return
					}

					msg := Message{Type: "ice-answer", Data: bytes}
					if err := websocket.JSON.Send(ws, msg); err != nil {
						log.Printf("error writing answer candidate: %v", err)
						return
					}
					log.Printf("%s's candidate read and sent to %s", req.To, username)
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

	var sendIce sync.WaitGroup
	// ticker := time.NewTicker(1 * time.Second)
	defer func() {
		// ticker.Stop()
		_ = ws.Close()
		sendIce.Wait()
	}()

	// this is logic that needs to run for the duration of the session/ws.
	// listen for room events. when another user joins, this logic must begin signalling with that user
	for {
		select {
		case <-ctx.Done():
			return
		// case <-ticker.C:
		// log.Println("tick")
		case offer := <-roomUser.Offers:
			bytes, err := json.Marshal(offer)
			if err != nil {
				log.Printf("error encoding candidate: %v", err)
				return
			}

			msg := Message{Type: "offer", Data: bytes}
			if err := websocket.JSON.Send(ws, msg); err != nil {
				log.Printf("error sending new offer to existing user %s: %v", offer.To, err)
			}
			log.Printf("%s received offer from %s and it was written to their ws\n", username, offer.From)
		// NOTE: all funcs in this need to run async, since chan is unbuffered and blocks on sends
		case msg := <-msgChan:
			// TODO: put this switch into its own func. run it in its own goroutine. it should use a waitgroup defined before this
			// event loop "msgHandlerWg". 'tick' will report the wg's counter (manually increment a top level int).
			switch msg.Type {
			// TODO: try to combine offer and answer handlers with additional property in CandidateMessage
			case "ice-offer":
				var data schemas.CandidateMessage
				json.Unmarshal(msg.Data, &data)

				conn, err := roomUser.PendingConnections.Get(data.UserId)
				if err != nil {
					log.Printf("%s unable to get conn for %s in ice-offer handler\n", username, data.Username)
					_ = ws.WriteClose(http.StatusInternalServerError)
					return
				}
				if data.Candidate.Candidate == "" {
					close(conn.From.Candidates)
					log.Printf("caller (%s) ice gather completed\n", data.Username)
					break
				}
				conn.From.Candidates <- data.Candidate
			case "ice-answer":
				var data schemas.CandidateMessage
				json.Unmarshal(msg.Data, &data)

				caller := room.GetUser(data.UserId)
				if caller == nil {
					log.Printf("%s unable to find caller %s in room while handling ice-answer message\n", username, data.Username)
					// _ = ws.WriteClose(http.StatusBadRequest)
					break
				}
				conn, err := caller.PendingConnections.Get(roomUser.Id)
				if err != nil {
					log.Printf("%s unable to get conn for %s in ice-answer handler\n", username, data.Username)
					// _ = ws.WriteClose(http.StatusBadRequest)
					break
				}
				if data.Candidate.Candidate == "" {
					close(conn.To.Candidates)
					log.Printf("answerer (%s) ice gather completed\n", data.Username)
					break
				}
				conn.To.Candidates <- data.Candidate
			// this is when the client answers a new user's offer
			case "answer":
				var answer schemas.ConnectionRequestWithId
				json.Unmarshal(msg.Data, &answer)

				if answer.Sd.SDP == "" {
					log.Println("empty answer")
					// _ = ws.WriteClose(http.StatusBadRequest)
					break
				}
				log.Printf("%s prepared answer (intended for %s)", username, answer.To)

				caller := room.GetUser(answer.ToId)
				if caller == nil {
					log.Printf("caller %s not found in room while handling answer message", answer.From)
					// _ = ws.WriteClose(http.StatusBadRequest)
					break
				}
				conn, err := caller.PendingConnections.Get(roomUser.Id)
				// conn, err := roomUser.PendingConnections.Get(answer.FromId)
				if err != nil {
					log.Printf("%s unable to get %s's conn for %s in answer handler\n", caller.Name, username)
					// _ = ws.WriteClose(http.StatusBadRequest)
					break
				}
				conn.Answer <- answer.Sd
				close(conn.Answer)
				log.Printf("answer sent to %s's chan", answer.To)

				sendIceCtx, cancelSendIce := context.WithTimeout(ctx, 30*time.Second)
				defer cancelSendIce()
				sendIce.Go(func() {
					defer cancelSendIce()
					for {
						select {
						case <-sendIceCtx.Done():
							log.Println("answer handler sendICE ctx cancelled, stopping listening for caller candidates")
							return
						// forwards caller's candidates to answerer
						case candidate, ok := <-conn.From.Candidates:
							bytes, err := json.Marshal(schemas.CandidateMessage{
								UserId:    caller.Id,
								Username:  caller.Name,
								Candidate: candidate,
							})
							if err != nil {
								log.Printf("error encoding candidate: %v", err)
								return
							}

							msg := Message{Type: "ice-offer", Data: bytes}
							if err := websocket.JSON.Send(ws, msg); err != nil {
								log.Printf("error writing caller candidate to answerer ws: %v", err)
								return
							}
							log.Printf("sent ice-offer from %s", caller.Name)
							if !ok { // empty end candidate sent, return
								conn.From.Candidates = nil // unness?
								return
							}
						}
					}
				})
			default:
				log.Printf("unknown message: %v", msg)
			}
		}
	}
}
