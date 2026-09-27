package routes

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"runtime/trace"
	"sync"
	"time"
	"uuid"

	"github.com/gregriff/vogo/server/internal/dal"
	"github.com/gregriff/vogo/server/internal/middleware"
	"github.com/gregriff/vogo/server/internal/state"
	"github.com/gregriff/vogo/shared"
	"github.com/gregriff/vogo/shared/requests"
	"github.com/gregriff/vogo/shared/wsock"
	"github.com/gregriff/vogo/shared/wsock/messages"
	"github.com/pion/webrtc/v4"
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
	calls := state.GetPendingCalls()
	// defer calls.PrintAll(logger.STATE)
	// add this call to pending map, using caller's ID since a client can only make one call at a time
	call, err := calls.AddNew(caller.Id, *caller, *recipient, offer.Sd)
	if err != nil {
		logger.STATE.Warn("while adding new Call, overwriting", "err", err)
	}
	defer calls.Delete(caller.Id)
	logger.STATE.Info("call created")

	// websocket message dispatcher
	g, gCtx := errgroup.WithContext(ctx)
	defer cleanupDispatcher(g, cancel, ws, logger.ROUTE)

	msgChan := make(chan wsock.Message)
	g.Go(func() error {
		return dispatchMessages(gCtx, cancel, ws, msgChan, logger.ROUTE)
	})

	// recv messages on websocket until cancelled.
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
					// note: this does not really need to close the ws.
					logger.ROUTE.Error("parsing ice-offer candidate", "err", err)
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

// parseCandidate parses an ICE candidate from data and returns the candidate.
func parseCandidate(ws *websocket.Conn, data json.RawMessage) (messages.Candidate, error) {
	var c messages.Candidate
	err := json.Unmarshal(data, &c)
	return c, err
}

// getUserErrCode returns an http error code for a non-nil
// error returned by dal.GetUser().
func getUserErrCode(err error) int {
	if err == sql.ErrNoRows {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
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

	// recv messages on websocket until cancelled.
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
					// note: this does not really need to close the ws.
					logger.ROUTE.Error("parsing ice-answer candidate", "err", err)
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
			case wsock.Offer, wsock.Answer, wsock.ICEOffer:
				logger.ROUTE.Error("unexpected message", "type", msg.Type, "data", msg.Data)
				_ = ws.WriteClose(http.StatusBadRequest)
				return
			}
		}
	}
}

// JoinRoom lets a user join a room they are a member of, given its name and owner's name,
// and creates the in-memory representation of that room if no members are currently connected
// to it. This is a websocket endpoint that will stay open until the user disconnects. When
// another member joins the room, this endpoint will send their Sd to the user, to facilitate
// the webrtc signaling for all members connected to the room.
func (h *RouteHandler) JoinRoom(ws *websocket.Conn) {
	ctx, cancel := context.WithCancel(ws.Request().Context())
	defer cancel()

	logger := h.loggers.forRequest(ws.Request())

	ctx, task := trace.NewTask(ctx, "Join")
	defer task.End()

	username := middleware.GetUsernameWS(ws)
	user, err := dal.GetUser(h.db, username)
	if err != nil {
		logger.ROUTE.Error("querying user", "err", err)
		_ = ws.WriteClose(getUserErrCode(err))
		return
	}
	trace.Log(ctx, "db", "got user")

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
	trace.Log(ctx, "ws", "received join room request")

	owner, err := dal.GetUser(h.db, req.OwnerName)
	if err != nil {
		logger.ROUTE.Error("querying room owner", "err", err)
		_ = ws.WriteClose(getUserErrCode(err))
		return
	}
	trace.Log(ctx, "db", "got owner")

	// ensure user is friends with owner.
	friends, err := user.HasFriend(h.db, owner.Id)
	if err != nil {
		logger.ROUTE.Error("querying friendship status", "with", owner, "err", err)
		_ = ws.WriteClose(http.StatusInternalServerError)
		return
	}
	if !friends && user.Id != owner.Id {
		logger.ROUTE.Error("not friends with owner", "owner", owner, "err", err)
		_ = ws.WriteClose(http.StatusBadRequest)
		return
	}
	trace.Log(ctx, "db", "verified friendship")

	// TODO: removeFriend and blockFriend endpoints should remove user
	// from relevant rooms in the same DB transaction
	c, err := dal.GetChannelOfMember(h.db, req.RoomName, user.Id, owner.Id)
	if err != nil {
		logger.ROUTE.Error("getting channel of member", "err", err)
		_ = ws.WriteClose(http.StatusInternalServerError)
		return
	}
	c.Owner = owner.Name
	trace.Log(ctx, "db", "got channel")

	// create or join room
	roomUser := state.NewRoomUser(user)
	room, err := state.CreateOrJoinRoom(c, roomUser, logger.Root())
	if err != nil {
		logger.STATE.Error("creating or joining room", "err", err)
		_ = ws.WriteClose(http.StatusInternalServerError)
		return
	}
	defer func() {
		// TODO: this never runs sometimes, so a goroutine leaks and never hits this when the user leaves.
		if err := room.Leave(c.Id, user.Id); err != nil {
			log.Panicf("room %s not in roomMap, while %s is attempting to leave it", room.Name, user.Name)
		}
	}()
	trace.Log(ctx, "state", "created or joined room")

	// notify client of all existing room users so it can send offers.
	users := room.Users(roomUser.Id)
	if err := sendBulkConnectionMsg(ws, users); err != nil {
		logger.ROUTE.Error("sending BulkConnection msg", "err", err)
		return
	}
	trace.Log(ctx, "ws", "sent bulk connection message")

	// note: parallelize this
	// TODO: if joiner could also include 1..n ice candidates in their initial offer msgs to each
	// existing user that would also make things quicker.
	var offers messages.BulkConnection
	if err = wsock.ReceiveJSON(ctx, ws, &offers); err != nil {
		if err == io.EOF {
			return
		}
		logger.ROUTE.Error("reading offers from ws", "err", err)
		_ = ws.WriteClose(http.StatusBadRequest)
		return
	}
	trace.Log(ctx, "ws", "received bulk connection response")

	if len(offers.Data) == 0 && len(users) > 0 {
		logger.ROUTE.Error("no offers received from ws", "users_in_room", len(users))
		_ = ws.WriteClose(http.StatusBadRequest)
		return
	}
	// use up-to-date user count
	if len(offers.Data) > 0 && len(room.Users(roomUser.Id)) == 0 {
		logger.STATE.Warn("offers received but users have left the room", "num_offers", len(offers.Data))
		_ = ws.WriteClose(http.StatusNotFound)
		return
	}

	// TODO: figure out how to use tracing regions or tasks with these contexts...
	var signalingWg sync.WaitGroup
	var signalingCtx, cancelSignaling = context.WithCancel(ctx)
	defer func() {
		cancelSignaling()
		signalingWg.Wait()
	}()

	// todo: parallelize
	for recipientId, offer := range offers.Data {
		signalingWg.Go(func() {
			ctx, cancel := context.WithTimeout(signalingCtx, 15*time.Second)
			defer trace.StartRegion(ctx, fmt.Sprintf("init-connection-%s", offer.To)).End()
			defer cancel()

			// TODO: there is some race going on below that prevents PendingConnections from being accurate at all times.
			recipient := dal.User{
				Id:   recipientId,
				Name: offer.To,
			}
			offerCh := users[recipientId].Offers
			beginSignaling(ctx, ws, roomUser, offer.Sd, *user, recipient, offerCh, logger)
		})
	}

	var listenWg sync.WaitGroup
	var listenCtx, cancelListen = context.WithCancel(ctx)
	var msgChan = make(chan wsock.Message)
	defer func() {
		cancelListen()
		listenWg.Wait()
	}()
	listenWg.Go(func() {
		defer trace.StartRegion(listenCtx, "listen").End()
		defer cancel() // if websocket closes, end all goroutines
		if err := wsock.Listen(listenCtx, ws, msgChan); err != nil {
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

	// waitgroups for message handling.
	var msgWg sync.WaitGroup
	var iceWg sync.WaitGroup
	defer func() {
		_ = ws.Close()
		msgWg.Wait()
		iceWg.Wait()
	}()

	// this is logic that needs to run for the duration of the session/ws.
	// listen for room events. when another user joins, this logic must begin signaling with that user
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-msgChan:
			// TODO: could return (error, bool), signaling an abort with the bool.
			// could also group retryable errors into a custom error, and also group client-side errors
			// to send back to the client. could also send all errors back to client for detailed notifications
			msgWg.Go(func() {
				defer trace.StartRegion(ctx, fmt.Sprintf("handle-%s", msg.Type)).End()
				if err := handleMsg(ctx, &iceWg, ws, msg, logger, room, roomUser); err != nil {
					logger.ROUTE.Error("handling message", "type", msg.Type, "err", err)
				}
			})
		case offer := <-roomUser.Offers:
			// when we get an offer from a new joiner, it must be relayed to self.
			bytes, err := json.Marshal(offer)
			if err != nil {
				logger.ROUTE.Error("encoding new offer", "from", offer.From, "err", err)
				continue
			}

			msg := wsock.Message{Type: wsock.Offer, Data: bytes}
			if err := websocket.JSON.Send(ws, msg); err != nil {
				logger.ROUTE.Error("relaying new offer", "from", offer.From, "to", offer.To, "err", err)
			}
			logger.WRTC.Debug("offer relayed", "from", offer.From)
			trace.Log(ctx, "webrtc", fmt.Sprintf("%s's offer relayed", offer.From))
		}
	}
}

// sendBulkConnectionMsg notifies the client of all the users it needs to send offers to.
func sendBulkConnectionMsg(ws *websocket.Conn, recipients map[uuid.UUID]state.RoomUser) error {
	newConns := requests.BulkConnection{
		Users: make(map[uuid.UUID]string, shared.ChannelCapacity-1),
	}
	for id, user := range recipients {
		newConns.Users[id] = user.Name
	}

	return websocket.JSON.Send(ws, newConns)
}

// beginSignaling initalizes the connection state between two users. Using the client's Sd,
// it creates an offer and sends it to the recipient's offer channel. It then starts a listening
// loop that relays the recipient's candidates and answer to the client.
func beginSignaling(
	ctx context.Context,
	ws *websocket.Conn,
	roomUser state.RoomUser,
	sd webrtc.SessionDescription,
	user,
	recipient dal.User,
	offerCh chan requests.ConnectionWithId,
	logger routeLoggers,
) {

	conn, err := roomUser.PendingConnections.AddNew(recipient.Id, user, recipient, sd)
	if err != nil {
		logger.STATE.Warn("while adding new PendingConnection, overwriting", "err", err)
	}
	defer roomUser.PendingConnections.Delete(recipient.Id)
	// note: its important to ensure that none of the below goroutines never die and
	// keep holding references to this connection^^ (could set it to nil to debug?)

	// create offer and send to recipient's chan.
	offer := requests.ConnectionWithId{
		Connection: requests.Connection{
			From: user.Name, // this is the caller's name
			To:   recipient.Name,
			Sd:   conn.From.Sd,
		},
		FromId: user.Id,
		ToId:   recipient.Id,
	}
	offerCh <- offer
	logger.WRTC.Debug("conn created, offer sent, signaling beginning", "with", recipient.Name)
	trace.Log(ctx, "webrtc", "sent offer")

	rErr := recvEvents(ctx, ws, conn.To, conn.Answer, logger.WRTC)
	if rErr != nil {
		logger.ROUTE.Error("while signaling or connecting", "err", rErr)
	}
}

// recvEvents relays candidates, and the answer from the recipient to
// the client until the answer has been relayed, or the ctx is cancelled.
//
// NOTE: candidate chan is no longer closed, so this func only ends when the context is canceled,
// in order to enable ICE restart. THIS CTX CANCELS AFTER 15 SECONDS! will need to start a new
// goroutine listening on the can chan if ICE restart is desired
func recvEvents(
	ctx context.Context,
	ws *websocket.Conn,
	from state.ClientInfo,
	answerCh <-chan webrtc.SessionDescription,
	logger *slog.Logger,
) error {
	for {
		select {
		case <-ctx.Done():
			return nil

		// recv answer from the recipient.
		case answerSd, ok := <-answerCh:
			if !ok {
				// if only one answer ever is sent, no need for closed check, just set to nil.
				answerCh = nil
				continue
			}
			logger.Debug("received answer from chan", "answer_from", from.Name)
			trace.Log(ctx, "webrtc", "received answer")

			bytes, err := json.Marshal(requests.Connection{
				To: from.Name, // this is the recipient
				Sd: answerSd,
			})
			if err != nil {
				return fmt.Errorf("encoding answer: %w", err)
			}

			msg := wsock.Message{Type: wsock.Answer, Data: bytes}
			if err := websocket.JSON.Send(ws, msg); err != nil {
				return fmt.Errorf("writing answer: %w", err)
			}
			logger.Debug("answer relayed", "from", from.Name)
			trace.Log(ctx, "webrtc", "relayed answer")

		// recv answer candidates from the recipient
		case candidate := <-from.Candidates:
			bytes, err := json.Marshal(messages.Candidate{
				UserId:    from.Id,
				Username:  from.Name,
				Candidate: candidate,
			})
			if err != nil {
				return fmt.Errorf("encoding candidate: %w", err)
			}

			msg := wsock.Message{Type: wsock.ICEAnswer, Data: bytes}
			if err := websocket.JSON.Send(ws, msg); err != nil {
				return fmt.Errorf("writing answer candidate: %w", err)
			}
			logger.Debug("candidate relayed", "from", from.Name)
			trace.Log(ctx, "webrtc", "relayed candidate")
		}
	}
}

// errors used to end the caller func. now none do. so any error that occurs while processing a msg
// will not end the join endpoint. TODO: if this behavior is kept, the errors should be sent back
// to the client so they know if they should terminate the connection. terminal errors should be
// handled by the caller and SHOULD terminate the entire connection.
func handleMsg(
	ctx context.Context,
	iceWg *sync.WaitGroup,
	ws *websocket.Conn,
	msg wsock.Message,
	logger routeLoggers,
	room *state.Room,
	roomUser state.RoomUser,
) error {
	switch msg.Type {
	// TODO: try to combine offer and answer handlers with additional property in messages.Candidate
	case wsock.ICEOffer:
		data, err := parseCandidate(ws, msg.Data)
		if err != nil {
			return fmt.Errorf("parsing ice-offer candidate: %w", err)
		}

		// TODO: this may run before the bulk connection has been recvd from the client? meaning that the conn
		// is not yet present. ICE msgs cannot be processed until connection has been created. add a chan.
		conn, err := roomUser.PendingConnections.Get(data.UserId)
		if err != nil {
			// this probably means signalling has completed. should be able to figure this out with sync primitives
			logger.STATE.Error("unable to get conn in ice-offer handler", "conn-owner", data.Username)
			return nil
		}
		if data.Candidate.Candidate == "" {
			logger.WRTC.Debug("ice gather completed (caller)", "caller", data.Username)
			trace.Log(ctx, "webrtc", "detected ice gather completed (caller)")
			break
		}
		conn.From.Candidates <- data.Candidate
		trace.Log(ctx, "webrtc", "relayed ice-offer candidate")
	case wsock.ICEAnswer:
		data, err := parseCandidate(ws, msg.Data)
		if err != nil {
			return fmt.Errorf("parsing ice-answer candidate: %w", err)
		}

		caller, found := room.GetUser(data.UserId)
		if !found {
			logger.STATE.Error("unable to find caller in room while handling ice-answer message", "caller", data.Username)
			return nil
		}
		conn, err := caller.PendingConnections.Get(roomUser.Id)
		if err != nil {
			logger.STATE.Error("unable to get conn in ice-answer handler", "conn_owner", data.Username)
			return nil
		}
		if data.Candidate.Candidate == "" {
			logger.WRTC.Debug("ice gather completed (answerer)", "answerer", data.Username)
			trace.Log(ctx, "webrtc", "detected ice gather completed (answerer)")
			return nil
		}
		conn.To.Candidates <- data.Candidate
		trace.Log(ctx, "webrtc", "relayed ice-answer candidate")
	// this is when the client answers a new user's offer
	case wsock.Answer:
		var answer requests.ConnectionWithId
		if err := json.Unmarshal(msg.Data, &answer); err != nil {
			return fmt.Errorf("unmarshalling answer: %w", err)
		}

		if answer.Sd.SDP == "" { // TODO: retry
			logger.WRTC.Debug("empty answer", "from", answer.From)
			return nil
		}
		logger.WRTC.Debug("answer prepared", "for", answer.To)

		caller, found := room.GetUser(answer.ToId)
		if !found {
			logger.STATE.Error("caller not found in room while handling answer message", "caller", answer.From)
			return nil
		}
		conn, err := caller.PendingConnections.Get(roomUser.Id)
		if err != nil {
			logger.STATE.Warn("unable to get conn in answer handler", "conn_owner", caller.Name)
			return nil
		}
		conn.Answer <- answer.Sd
		close(conn.Answer)
		logger.WRTC.Debug("answer sent", "to", answer.To)
		trace.Log(ctx, "webrtc", "answer sent")

		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		iceWg.Go(func() {
			defer trace.StartRegion(ctx, fmt.Sprintf("relay %s's candidates", conn.From.Name)).End()
			defer cancel()
			if err := relayIceCandidates(ctx, ws, caller, conn.From.Candidates, logger.WRTC); err != nil {
				logger.ROUTE.Error("while relaying ICE candidates", "err", err)
			}
		})
	case wsock.Offer, wsock.Connected:
		return fmt.Errorf("unexpected message: %v", msg.Data)
	}
	return nil
}

// NOTE: relaying caller's ice candidates to the client (user that was already in the room) may
// be able to be done BEFORE client's answer is relayed to caller. client could buffer candidates...
// check webrtc spec.
//
// NOTE: candidate chan is no longer closed, so this func only ends when the context is canceled,
// in order to enable ICE restart.
func relayIceCandidates(
	ctx context.Context,
	ws *websocket.Conn,
	caller state.RoomUser,
	ch <-chan webrtc.ICECandidateInit,
	logger *slog.Logger,
) error {
	for {
		select {
		case <-ctx.Done():
			logger.Debug("answer handler ice ctx cancelled. stopping listening for caller candidates")
			return nil
		// forwards caller's candidates to client
		case candidate := <-ch:
			bytes, err := json.Marshal(messages.Candidate{
				UserId:    caller.Id,
				Username:  caller.Name,
				Candidate: candidate,
			})
			if err != nil {
				return fmt.Errorf("encoding caller candidates to relay: %w", err)
			}

			msg := wsock.Message{Type: wsock.ICEOffer, Data: bytes}
			if err := websocket.JSON.Send(ws, msg); err != nil {
				return fmt.Errorf("writing caller's candidate to client ws: %w", err)
			}
			logger.Debug("ice-offer relayed", "from", caller.Name)
			trace.Log(ctx, "webrtc", "relayed ice-offer candidate")
		}
	}
}
