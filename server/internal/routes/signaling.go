package routes

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
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
	"golang.org/x/sync/errgroup"
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
	defer func() {
		cancel()
		_ = ws.Close()
	}()

	logger := h.loggers.forRequest(ws.Request())

	username := middleware.GetUsernameWS(ws)
	caller, err := dal.GetUser(h.db, username)
	if err != nil {
		logger.ROUTE.Error("querying user", "err", err)
		_ = ws.WriteClose(getUserErrCode(err))
		return
	}

	offer, err := recvConnectionRequest(ctx, ws)
	if err != nil {
		logger.ROUTE.Error("receiving offer", "err", err)
		return
	}
	logger.WRTC.Debug("offer received")

	recipient, err := dal.GetUser(h.db, offer.To)
	if err != nil {
		logger.ROUTE.Error("querying recipient", "err", err)
		_ = ws.WriteClose(getUserErrCode(err))
		return
	}

	friends, err := caller.HasFriend(h.db, recipient.Id)
	if err != nil {
		logger.ROUTE.Error("querying friendship status", "with", recipient.Name, "err", err)
		_ = ws.WriteClose(http.StatusInternalServerError)
		return
	}
	if !friends {
		logger.ROUTE.Error("not friends", "with", recipient.Name, "err", err)
		_ = ws.WriteClose(http.StatusBadRequest)
		return
	}

	// create the call in memory, delete once answered
	call := state.CreateConnection(*caller, *recipient, offer.Sd)
	calls := state.GetPendingCalls()
	// defer calls.PrintAll(logger.STATE)
	// add this call to pending map, using caller's ID since a client can only make one call at a time
	calls.Add(caller.Id, call)
	defer calls.Delete(caller.Id)
	logger.STATE.Info("call created")

	// websocket message dispatcher
	g, gCtx := errgroup.WithContext(ctx)
	defer cleanupDispatcher(g, cancel, ws, logger.ROUTE)

	msgChan := make(chan wsock.Message)
	g.Go(func() error {
		return dispatchMessages(gCtx, cancel, ws, msgChan, logger.ROUTE)
	})

	for {
		select {
		case <-ctx.Done():
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
			case wsock.Connected:
				// when a client sends a 'connected' message, they close the WS immediately after.
				logger.ROUTE.Info("call connected", "with", recipient.Name)
				return
			case wsock.ICEOffer:
				data, err := parseCandidate(ws, msg.Data)
				if err != nil {
					logger.ROUTE.Error("parsing ice-offer candidate", "err", err)
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
			case wsock.Offer, wsock.Answer, wsock.ICEAnswer:
				logger.ROUTE.Error("unexpected message", "type", msg.Type, "data", msg.Data)
				_ = ws.WriteClose(http.StatusBadRequest)
				return
			}
		}
	}
}

// recvConnectionRequest blocks until an offer or answer request is received from ws,
// and closes ws if an error is encountered or the request's SDP is empty.
func recvConnectionRequest(ctx context.Context, ws *websocket.Conn) (requests.Connection, error) {
	var req requests.Connection
	if err := wsock.ReceiveJSON(ctx, ws, &req); err != nil {
		if err != io.EOF {
			_ = ws.WriteClose(http.StatusBadRequest)
		}
		return req, err
	}

	if req.Sd.SDP == "" {
		_ = ws.WriteClose(http.StatusBadRequest)
		return req, fmt.Errorf("empty sdp")
	}
	return req, nil
}

// dispatchMessages sends messages received from ws to ch until ctx is cancelled
// or an error is encountered. It should be run in its own goroutine, so that it can signal
// and cancel the message receiving loop running in the main request handler goroutine.
// Upon returning, it cancels its context, which should be the top-level context.
// This is used during calling and answering to terminate the infinite message receive
// loop and close the ws once the webrtc connection has been made.
func dispatchMessages(
	ctx context.Context,
	cancel context.CancelFunc,
	ws *websocket.Conn,
	ch chan<- wsock.Message,
	logger *slog.Logger,
) error {
	defer cancel()

	err := wsock.Listen(ctx, ws, ch)
	if err == io.EOF {
		logger.Info("connection closed by client")
		return nil
	}
	if err != nil {
		logger.Error("during message loop", "err", err)
	}
	return err
}

// cleanupDispatcher cleans up the message dispatcher. It cancels the dispatcher's context,
// which immediately unblocks it from reading the websocket. It then waits for the errgroup,
// logs any errors and closes the websocket once done. It should be deferred by its caller.
func cleanupDispatcher(
	g *errgroup.Group,
	cancel context.CancelFunc,
	ws *websocket.Conn,
	logger *slog.Logger,
) {
	cancel()
	if err := g.Wait(); err != nil {
		logger.Error("while cancelling", "err", err)
	}
	_ = ws.Close()
}

// parseCandidate parses an ICE candidate from data and returns
// the candidate. It closes the ws if an error is encountered.
func parseCandidate(ws *websocket.Conn, data json.RawMessage) (messages.Candidate, error) {
	var c messages.Candidate

	err := json.Unmarshal(data, &c)
	if err != nil {
		_ = ws.WriteClose(http.StatusBadRequest)
	}
	return c, err
}

// Answer obtains the caller's name from the first ws message and sends the caller's offer Sd to the client.
// It then waits for the clients answer, where it then facilitates trickle-ICE gathering between the two clients.
func (h *RouteHandler) Answer(ws *websocket.Conn) {
	ctx, cancel := context.WithTimeout(ws.Request().Context(), time.Second*15)
	defer func() {
		cancel()
		_ = ws.Close()
	}()

	logger := h.loggers.forRequest(ws.Request())

	username := middleware.GetUsernameWS(ws)
	recipient, err := dal.GetUser(h.db, username)
	if err != nil {
		logger.ROUTE.Error("querying user", "err", err)
		_ = ws.WriteClose(getUserErrCode(err))
		return
	}

	// ensure the pending call exists
	callerName := ws.Request().PathValue("name")
	caller, err := dal.GetUser(h.db, callerName)
	if err != nil {
		logger.ROUTE.Error("querying caller", "err", err)
		_ = ws.WriteClose(getUserErrCode(err))
		return
	}

	friends, err := recipient.HasFriend(h.db, caller.Id)
	if err != nil {
		logger.ROUTE.Error("querying friendship status", "with", callerName, "err", err)
		_ = ws.WriteClose(http.StatusInternalServerError)
		return
	}
	if !friends {
		logger.ROUTE.Error("not friends", "with", callerName, "err", err)
		_ = ws.WriteClose(http.StatusBadRequest)
		return
	}

	calls := state.GetPendingCalls()
	// defer calls.PrintAll(logger.STATE)
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
	answer, err := recvConnectionRequest(ctx, ws)
	if err != nil {
		logger.ROUTE.Error("receiving answer", "err", err)
		return
	}
	logger.WRTC.Debug("answer received")

	call.Answer <- answer.Sd

	// websocket message dispatcher
	g, gCtx := errgroup.WithContext(ctx)
	defer cleanupDispatcher(g, cancel, ws, logger.ROUTE)

	msgChan := make(chan wsock.Message)
	g.Go(func() error {
		return dispatchMessages(gCtx, cancel, ws, msgChan, logger.ROUTE)
	})

	for {
		select {
		case <-ctx.Done():
			return
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
			case wsock.Connected:
				// when a client sends a 'connected' message, they close the WS immediately after.
				logger.ROUTE.Info("call connected", "with", caller.Name)
				return
			case wsock.ICEAnswer:
				data, err := parseCandidate(ws, msg.Data)
				if err != nil {
					logger.ROUTE.Error("parsing ice-answer candidate", "err", err)
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
			case wsock.Offer, wsock.Answer, wsock.ICEOffer:
				logger.ROUTE.Error("unexpected message", "type", msg.Type, "data", msg.Data)
				_ = ws.WriteClose(http.StatusBadRequest)
				return
			}
		}
	}
}

// getUserErrCode returns an http error code for a non-nil
// error returned by dal.GetUser().
func getUserErrCode(err error) int {
	if err == sql.ErrNoRows {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
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
		logger.ROUTE.Error("querying user", "err", err)
		_ = ws.WriteClose(getUserErrCode(err))
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
		logger.ROUTE.Error("querying room owner", "err", err)
		_ = ws.WriteClose(getUserErrCode(err))
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

					msg := wsock.Message{Type: wsock.Answer, Data: bytes}
					if err := websocket.JSON.Send(ws, msg); err != nil {
						logger.ROUTE.Error("writing answer", "err", err)
						return
					}
					logger.WRTC.Debug("answer relayed", "from", req.To)
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

					msg := wsock.Message{Type: wsock.ICEAnswer, Data: bytes}
					if err := websocket.JSON.Send(ws, msg); err != nil {
						logger.ROUTE.Error("writing answer candidate", "err", err)
						return
					}
					logger.WRTC.Debug("candidate relayed", "from", req.To)
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
			// when we get an offer from a new joiner, it must be relayed to self.
			bytes, err := json.Marshal(offer)
			if err != nil {
				logger.ROUTE.Error("encoding candidate", "err", err)
				return
			}

			msg := wsock.Message{Type: wsock.Offer, Data: bytes}
			if err := websocket.JSON.Send(ws, msg); err != nil {
				logger.ROUTE.Error("relaying new offer", "from", offer.From, "to", offer.To, "err", err)
			}
			logger.WRTC.Debug("offer relayed", "from", offer.From)
		// NOTE: all funcs in this need to run async, since chan is unbuffered and blocks on sends
		case msg := <-msgChan:
			// TODO: put this switch into its own func. run it in its own goroutine. it should use a waitgroup defined before this
			// event loop "msgHandlerWg". 'tick' will report the wg's counter (manually increment a top level int).
			switch msg.Type {
			// TODO: try to combine offer and answer handlers with additional property in messages.Candidate
			case wsock.ICEOffer:
				data, err := parseCandidate(ws, msg.Data)
				if err != nil {
					logger.ROUTE.Error("parsing ice-offer candidate", "err", err)
					return
				}

				conn, err := roomUser.PendingConnections.Get(data.UserId)
				if err != nil {
					// this probably means signalling has completed. should be able to figure this out with sync primitives
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
			case wsock.ICEAnswer:
				data, err := parseCandidate(ws, msg.Data)
				if err != nil {
					logger.ROUTE.Error("parsing ice-answer candidate", "err", err)
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
			case wsock.Answer:
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

							msg := wsock.Message{Type: wsock.ICEOffer, Data: bytes}
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
			case wsock.Offer, wsock.Connected:
				logger.ROUTE.Error("unexpected message", "type", msg.Type, "data", msg.Data)
			}
		}
	}
}
