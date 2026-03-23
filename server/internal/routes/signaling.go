package routes

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gregriff/vogo/server/internal/dal"
	"github.com/gregriff/vogo/server/internal/middleware"
	"github.com/gregriff/vogo/server/internal/state"
	"github.com/gregriff/vogo/shared"
	"github.com/gregriff/vogo/shared/requests"
	"github.com/gregriff/vogo/shared/wsock"
	"github.com/gregriff/vogo/shared/wsock/messages"
	"golang.org/x/net/websocket"
)

// TODO:
// PUT /channel: modify channel properties
// DELETE /channel

// Call initiates signaling for a voice call that may only be accepted by the intended recipient. The caller's
// ICE candidates are stored in memory until the recipient answers, where they are then forwarded. Call then
// receives the recipient's ICE candidates and forwards them to the caller. When candidates have been fully
// exchanged Call deletes the signaling data from memory and returns.
func (h *RouteHandler) Call(ws *websocket.Conn) {
	ctx, cancel := context.WithTimeout(ws.Request().Context(), time.Second*30)
	defer cancel()

	logger := h.loggers.forRequest(ws.Request())

	username := middleware.GetUsernameWS(ws)
	caller, err := dal.GetUser(h.db, username)
	if err != nil {
		logger.ROUTE.Error("fetching caller", "err", err)
		_ = ws.WriteClose(http.StatusInternalServerError)
		return
	}

	var offer requests.Connection
	if err := wsock.ReceiveJSON(ctx, ws, &offer); err != nil {
		if err == io.EOF {
			return
		}
		logger.ROUTE.Error("receiving offer", "err", err)
		_ = ws.WriteClose(http.StatusBadRequest)
		return
	}
	if offer.Sd.SDP == "" {
		logger.ROUTE.Error("empty offer")
		_ = ws.WriteClose(http.StatusBadRequest)
		return
	}
	logger.WRTC.Debug("offer received")
	recipient, err := dal.GetUser(h.db, offer.To)
	if err != nil {
		logger.ROUTE.Error("fetching recipient", "err", err)
		_ = ws.WriteClose(http.StatusBadRequest)
		return
	}

	friends, err := dal.AreFriends(h.db, caller.Id, recipient.Id)
	if err != nil {
		logger.ROUTE.Error("checking friendship status", "err", err)
		_ = ws.WriteClose(http.StatusInternalServerError)
		return
	}
	if !friends {
		logger.ROUTE.Error("caller not friends with recipient", "err", err)
		_ = ws.WriteClose(http.StatusBadRequest)
		return
	}

	// create the call in memory, delete once answered
	call := state.CreateConnection(*caller, *recipient, offer.Sd)
	calls := state.GetPendingCalls()
	// add this call to pending map, using caller's ID since a client can only make one call at a time
	calls.Add(caller.Id, call)
	defer calls.Delete(caller.Id)
	logger.STATE.Info("call created")

	var (
		wsRecvWg                sync.WaitGroup
		wsRecvCtx, cancelWsRecv = context.WithCancel(ctx)
		msgChan                 = make(chan wsock.Message)
	)
	defer func() {
		cancelWsRecv()
		wsRecvWg.Wait()
	}()
	wsRecvWg.Go(func() {
		defer cancel() // if websocket closes, end all goroutines
		if err := wsock.Listen(wsRecvCtx, ws, msgChan); err != nil {
			if err == io.EOF { // todo: may need to handle this in startMessageLoop
				logger.ROUTE.Info("connection closed")
			} else {
				logger.ROUTE.Error("error during message loop", "err", err)
			}
			_ = ws.WriteClose(http.StatusInternalServerError)
		}
	})

	for {
		select {
		case <-ctx.Done():
			logger.ROUTE.Info("context cancelled", "reason", ctx.Err())
			_ = ws.Close()
			return
		case answerSd := <-call.Answer:
			if err := websocket.JSON.Send(ws, answerSd); err != nil {
				logger.ROUTE.Error("writing answer", "err", err)
				_ = ws.WriteClose(http.StatusBadRequest)
				return
			}
		case answerCandidate, ok := <-call.To.Candidates:
			if err := websocket.JSON.Send(ws, answerCandidate); err != nil {
				logger.ROUTE.Error("writing candidate", "err", err)
				_ = ws.WriteClose(http.StatusInternalServerError)
				return
			}
			if !ok {
				call.To.Candidates = nil
			}
		case msg := <-msgChan:
			switch msg.Type {
			case "connected":
				cancel()
				return
			case "ice-offer":
				var data messages.Candidate
				if err := json.Unmarshal(msg.Data, &data); err != nil {
					logger.ROUTE.Error("unmarshalling ice-offer candidate", "err", err)
					_ = ws.WriteClose(http.StatusBadRequest)
					return
				}

				call, err := calls.Get(caller.Id)
				if err != nil {
					logger.STATE.Error("call not found during trickle ICE", "err", err)
					_ = ws.WriteClose(http.StatusInternalServerError)
					return
				}

				if data.Candidate.Candidate == "" {
					close(call.From.Candidates)
					continue
				}
				call.From.Candidates <- data.Candidate
				logger.WRTC.Debug("caller candidate sent")
			}
		}
	}
}

// Answer obtains the caller's name from the first ws message and sends the caller's offer Sd to the client.
// It then waits for the clients answer, where it then facilitates trickle-ICE gathering between the two clients.
func (h *RouteHandler) Answer(ws *websocket.Conn) {
	ctx, cancel := context.WithTimeout(ws.Request().Context(), time.Second*15)
	defer cancel()

	logger := h.loggers.forRequest(ws.Request())

	username := middleware.GetUsernameWS(ws)
	recipient, err := dal.GetUser(h.db, username)
	if err != nil {
		logger.ROUTE.Error("fetching recipient", "err", err)
		_ = ws.WriteClose(http.StatusInternalServerError)
		return
	}

	// ensure the pending call exists
	callerName := ws.Request().PathValue("name")
	caller, err := dal.GetUser(h.db, callerName)
	if err != nil {
		logger.ROUTE.Error("fetching caller", "err", err)
		_ = ws.WriteClose(http.StatusBadRequest)
		return
	}
	friends, err := dal.AreFriends(h.db, caller.Id, recipient.Id)
	if err != nil {
		logger.ROUTE.Error("checking friendship status", "err", err)
		_ = ws.WriteClose(http.StatusInternalServerError)
		return
	}
	if !friends {
		logger.ROUTE.Error("recipient not friends with caller", "err", err)
		_ = ws.WriteClose(http.StatusBadRequest)
		return
	}

	calls := state.GetPendingCalls()
	call, err := calls.Get(caller.Id)
	if err != nil {
		logger.STATE.Error("call not found")
		_ = ws.WriteClose(http.StatusBadRequest)
		return
	}
	defer calls.Delete(caller.Id)

	// send caller's SD. client will then create an answer and post it to this ws
	if err := websocket.JSON.Send(ws, call.From.Sd); err != nil {
		logger.ROUTE.Error("writing offer", "err", err)
		return
	}

	// wait for answer from client
	var answer requests.Connection
	if err = wsock.ReceiveJSON(ctx, ws, &answer); err != nil {
		if err == io.EOF {
			return
		}
		logger.ROUTE.Error("receiving answer", "err", err)
		_ = ws.WriteClose(http.StatusBadRequest)
		return
	}
	if answer.Sd.SDP == "" {
		logger.WRTC.Error("empty answer")
		_ = ws.WriteClose(http.StatusBadRequest)
		return
	}
	logger.WRTC.Debug("answer received")
	call.Answer <- answer.Sd

	var (
		wsRecvWg                sync.WaitGroup
		wsRecvCtx, cancelWsRecv = context.WithCancel(ctx)
		msgChan                 = make(chan wsock.Message)
	)
	defer func() {
		cancelWsRecv()
		wsRecvWg.Wait()
	}()
	wsRecvWg.Go(func() {
		defer cancel() // if websocket closes, end all goroutines
		if err := wsock.Listen(wsRecvCtx, ws, msgChan); err != nil {
			if err == io.EOF { // todo: may need to handle this in startMessageLoop
				logger.ROUTE.Info("connection closed")
			} else {
				logger.ROUTE.Error("error during message loop", "err", err)
			}
			_ = ws.WriteClose(http.StatusInternalServerError)
		}
	})

	for {
		select {
		case <-ctx.Done():
			logger.ROUTE.Info("context cancelled", "reason", ctx.Err())
			_ = ws.Close()
			return
		// note: this needs to continue to run even if readchan is closed. this may always complete first tho...
		case candidate, ok := <-call.From.Candidates:
			if !ok {
				call.From.Candidates = nil
			}
			if err := websocket.JSON.Send(ws, candidate); err != nil {
				logger.ROUTE.Error("writing answer", "err", err)
				return
			}
		case msg := <-msgChan:
			switch msg.Type {
			case "connected":
				cancel()
				return
			case "ice-answer":
				var data messages.Candidate
				if err := json.Unmarshal(msg.Data, &data); err != nil {
					logger.ROUTE.Error("unmarshalling ice-answer candidate", "err", err)
					_ = ws.WriteClose(http.StatusBadRequest)
					return
				}

				call, err := calls.Get(caller.Id)
				if err != nil {
					logger.STATE.Error("call not found during trickle ICE", "err", err)
					_ = ws.WriteClose(http.StatusInternalServerError)
					return
				}

				if data.Candidate.Candidate == "" {
					close(call.To.Candidates)
					continue
				}
				call.To.Candidates <- data.Candidate
				logger.WRTC.Debug("answer candidate sent")
			}
		}
	}
}

// JoinRoom lets a user join a room they are a member of, given its name and owner's name,
// and creates the in-memory representation of that room if no members are currently connected
// to it. This is a websocket endpoint that will stay open until the user leaves the room call
// or is disconnected. When another member joins the room, this endpoint will send their Sd to
// the user, to facilitate the webrtc signaling for all members connected to the room.
func (h *RouteHandler) JoinRoom(ws *websocket.Conn) {
	ctx, cancel := context.WithCancel(ws.Request().Context())
	defer cancel()

	logger := h.loggers.forRequest(ws.Request())

	username := middleware.GetUsernameWS(ws)
	user, err := dal.GetUser(h.db, username)
	if err != nil {
		logger.ROUTE.Error("fetching user", "err", err)
		_ = ws.WriteClose(http.StatusInternalServerError)
		return
	}

	var req requests.JoinRoom
	err = wsock.ReceiveJSON(ctx, ws, &req)
	if err != nil {
		if err == io.EOF {
			return
		}
		logger.ROUTE.Error("reading offer from ws", "err", err)
		_ = ws.WriteClose(http.StatusBadRequest)
		return
	}
	owner, err := dal.GetUser(h.db, req.OwnerName)
	if err != nil {
		logger.ROUTE.Error("fetching room owner", "err", err)
		_ = ws.WriteClose(http.StatusBadRequest)
		return
	}

	// add areFriends check? TODO: removeFriend and blockFriend endpoints should remove user
	// from relevant rooms in the same DB transaction
	c, err := dal.GetChannelOfMember(h.db, req.RoomName, user.Id, owner.Id)
	if err != nil {
		logger.ROUTE.Error("getting channel of member", "err", err)
		_ = ws.WriteClose(http.StatusInternalServerError)
		return
	}
	c.Owner = owner.Name

	// create or join room
	roomUser := state.NewRoomUser(user)
	room, err := state.CreateOrJoinRoom(c, roomUser, logger.Root())
	if err != nil {
		logger.STATE.Error("creating or joining room", "err", err)
		_ = ws.WriteClose(http.StatusInternalServerError)
		return
	}
	defer room.Leave(roomUser)

	users := room.Users(roomUser.Id)
	// own func
	newConns := requests.BulkConnection{
		Users: make(map[uuid.UUID]string, shared.ChannelCapacity-1),
	}
	for id, user := range users {
		newConns.Users[id] = user.Name
	}
	if err := websocket.JSON.Send(ws, newConns); err != nil {
		logger.ROUTE.Error("writing NewConnections msg", "err", err)
		return
	}
	// ^^

	// note: parallelize this
	var offers messages.BulkConnection
	if err = wsock.ReceiveJSON(ctx, ws, &offers); err != nil {
		if err == io.EOF {
			return
		}
		logger.ROUTE.Error("reading offers from ws", "err", err)
		_ = ws.WriteClose(http.StatusBadRequest)
		return
	}
	if len(offers.Data) == 0 && len(users) != 0 {
		logger.ROUTE.Error("no offers received from ws", "users_in_room", len(users))
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
				logger.STATE.Warn("client wants to send an offer to someone who is not in the room", "recipient", req.To)
				return
			}

			recipient := dal.User{
				Id:   recipientId,
				Name: users[recipientId].Name,
			}
			conn := state.CreateConnection(*user, recipient, req.Sd)
			roomUser.PendingConnections.Add(recipientId, conn)
			defer roomUser.PendingConnections.Delete(recipientId)
			// note: its important to ensure that none of the below goroutines never die and
			// keep holding references to this connection^^ (could set it to nil to debug?)

			offer := requests.ConnectionWithId{
				Connection: requests.Connection{
					From: user.Name, // this is the caller's name
					To:   recipient.Name,
					Sd:   conn.From.Sd,
				},
				FromId: user.Id,
				ToId:   recipientId,
			}
			users[recipientId].Offers <- offer
			logger.WRTC.Debug("conn created, signaling beginning", "with", req.To)

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
					logger.WRTC.Debug("received answer from chan", "answer_from", req.To)
					bytes, err := json.Marshal(requests.Connection{
						To: req.To, // this is the recipient
						Sd: answerSd,
					})
					if err != nil {
						logger.ROUTE.Error("encoding candidate", "err", err)
						return
					}

					msg := wsock.Message{Type: "answer", Data: bytes}
					if err := websocket.JSON.Send(ws, msg); err != nil {
						logger.ROUTE.Error("writing answer", "err", err)
						return
					}
					logger.WRTC.Debug("answer sent to self", "answer_from", req.To)
				case answerCandidate, ok := <-conn.To.Candidates:
					bytes, err := json.Marshal(messages.Candidate{
						UserId:    recipientId,
						Username:  req.To, // this is the recipient
						Candidate: answerCandidate,
					})
					if err != nil {
						logger.ROUTE.Error("encoding candidate", "err", err)
						return
					}

					msg := wsock.Message{Type: "ice-answer", Data: bytes}
					if err := websocket.JSON.Send(ws, msg); err != nil {
						logger.ROUTE.Error("writing answer candidate", "err", err)
						return
					}
					logger.WRTC.Debug("recipient candidate read and sent to self", "recipient", req.To)
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
		msgChan                 = make(chan wsock.Message)
	)
	defer func() {
		cancelWsRecv()
		wsRecvWg.Wait()
	}()
	wsRecvWg.Go(func() {
		defer cancel() // if websocket closes, end all goroutines
		if err := wsock.Listen(wsRecvCtx, ws, msgChan); err != nil {
			if err == io.EOF { // todo: may need to handle this in startMessageLoop
				logger.ROUTE.Info("connection closed")
			} else {
				logger.ROUTE.Error("error during message loop", "err", err)
			}
			_ = ws.WriteClose(http.StatusInternalServerError)
		}
	})

	// now that the offers are being sent to existing users, we can start the loop that will run for the rest of the time
	// that the user is in the room.

	var sendIce sync.WaitGroup
	defer func() {
		_ = ws.Close()
		sendIce.Wait()
	}()

	// this is logic that needs to run for the duration of the session/ws.
	// listen for room events. when another user joins, this logic must begin signaling with that user
	for {
		select {
		case <-ctx.Done():
			return
		case offer := <-roomUser.Offers:
			bytes, err := json.Marshal(offer)
			if err != nil {
				logger.ROUTE.Error("encoding candidate", "err", err)
				return
			}

			msg := wsock.Message{Type: "offer", Data: bytes}
			if err := websocket.JSON.Send(ws, msg); err != nil {
				logger.ROUTE.Error("sending new offer to existing user", "to", offer.To, "err", err)
			}
			logger.WRTC.Debug("offer received, written to answerer's ws", "offerer", offer.From)
		// NOTE: all funcs in this need to run async, since chan is unbuffered and blocks on sends
		case msg := <-msgChan:
			// TODO: put this switch into its own func. run it in its own goroutine. it should use a waitgroup defined before this
			// event loop "msgHandlerWg". 'tick' will report the wg's counter (manually increment a top level int).
			switch msg.Type {
			// TODO: try to combine offer and answer handlers with additional property in messages.Candidate
			case "ice-offer":
				var data messages.Candidate
				if err := json.Unmarshal(msg.Data, &data); err != nil {
					logger.ROUTE.Error("unmarshalling ice-offer candidate", "err", err)
					_ = ws.WriteClose(http.StatusBadRequest)
					return
				}

				conn, err := roomUser.PendingConnections.Get(data.UserId)
				if err != nil {
					logger.ROUTE.Error("unable to get conn in ice-offer handler", "conn-owner", data.Username)
					_ = ws.WriteClose(http.StatusInternalServerError)
					return
				}
				if data.Candidate.Candidate == "" {
					close(conn.From.Candidates)
					logger.WRTC.Debug("caller ice gather completed", "callerName", data.Username)
					break
				}
				conn.From.Candidates <- data.Candidate
			case "ice-answer":
				var data messages.Candidate
				if err := json.Unmarshal(msg.Data, &data); err != nil {
					logger.ROUTE.Error("unmarshaling ice-answer candidate", "err", err)
					_ = ws.WriteClose(http.StatusBadRequest)
					return
				}

				caller := room.GetUser(data.UserId)
				if caller == nil {
					logger.STATE.Error("unable to find caller in room while handling ice-answer message", "caller", data.Username)
					// _ = ws.WriteClose(http.StatusBadRequest)
					break
				}
				conn, err := caller.PendingConnections.Get(roomUser.Id)
				if err != nil {
					logger.STATE.Warn("unable to get conn in ice-answer handler", "conn_owner", data.Username)
					// _ = ws.WriteClose(http.StatusBadRequest)
					break
				}
				if data.Candidate.Candidate == "" {
					close(conn.To.Candidates)
					logger.WRTC.Debug("answerer ice gather completed", "answerer", data.Username)
					break
				}
				conn.To.Candidates <- data.Candidate
			// this is when the client answers a new user's offer
			case "answer":
				var answer requests.ConnectionWithId
				if err := json.Unmarshal(msg.Data, &answer); err != nil {
					logger.ROUTE.Error("unmarshalling answer", "err", err)
					_ = ws.WriteClose(http.StatusBadRequest)
					return
				}

				if answer.Sd.SDP == "" {
					logger.WRTC.Debug("empty answer", "from", answer.From)
					// _ = ws.WriteClose(http.StatusBadRequest)
					break
				}
				logger.WRTC.Debug("answer prepared", "for", answer.To)

				caller := room.GetUser(answer.ToId)
				if caller == nil {
					logger.STATE.Error("caller not found in room while handling answer message", "caller", answer.From)
					// _ = ws.WriteClose(http.StatusBadRequest)
					break
				}
				conn, err := caller.PendingConnections.Get(roomUser.Id)
				if err != nil {
					logger.STATE.Warn("unable to get conn in answer handler", "conn_owner", caller.Name, "conn_for", conn.To.User.Name)
					// _ = ws.WriteClose(http.StatusBadRequest)
					break
				}
				conn.Answer <- answer.Sd
				close(conn.Answer)
				logger.WRTC.Debug("answer sent to caller's chan", "callerName", answer.To)

				sendIceCtx, cancelSendIce := context.WithTimeout(ctx, 30*time.Second)
				defer cancelSendIce()
				sendIce.Go(func() {
					defer cancelSendIce()
					for {
						select {
						case <-sendIceCtx.Done():
							logger.ROUTE.Debug("answer handler sendICE ctx cancelled. stopping listening for caller candidates")
							return
						// forwards caller's candidates to answerer
						case candidate, ok := <-conn.From.Candidates:
							bytes, err := json.Marshal(messages.Candidate{
								UserId:    caller.Id,
								Username:  caller.Name,
								Candidate: candidate,
							})
							if err != nil {
								logger.ROUTE.Error("encoding candidates", "err", err)
								return
							}

							msg := wsock.Message{Type: "ice-offer", Data: bytes}
							if err := websocket.JSON.Send(ws, msg); err != nil {
								logger.ROUTE.Error("writing caller candidate to answerer ws", "err", err)
								return
							}
							logger.WRTC.Debug("sent ice-offer", "from", caller.Name)
							if !ok { // empty end candidate sent, return
								conn.From.Candidates = nil // unness?
								return
							}
						}
					}
				})
			default:
				logger.ROUTE.Error("unknown message", "message", msg)
			}
		}
	}
}
