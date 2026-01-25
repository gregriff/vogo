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
	call := schemas.CreateCall(caller, recipient, offer.Sd)
	calls := schemas.GetPendingCalls()
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

	calls := schemas.GetPendingCalls()
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

// JoinChannel lets a user join a channel they are a member of, given its name and owner's name,
// and creates the in-memory representation of that channel if no members are currently connected
// to it. This is a websocket endpoint that will stay open until the user leaves the channel call
// or is disconnected. When another member joins the channel, this endpoint will send their Sd to
// the user, to facilitate the webrtc signalling for all members connected to the channel.
func (h *RouteHandler) JoinChannel(ws *websocket.Conn) {
	ctx, cancel := context.WithCancel(ws.Request().Context())
	defer cancel()

	username := middleware.GetUsernameWS(ws)
	user, err := dal.GetUser(h.db, username)
	if err != nil {
		log.Println(fmt.Errorf("error fetching user: %w", err))
		_ = ws.WriteClose(http.StatusInternalServerError)
		return
	}

	var req schemas.JoinChannelRequest
	err = receiveWithContext(ctx, ws, &req)
	if err != nil {
		if err == io.EOF {
			return
		}
		log.Printf("error reading offer from ws: %v", err)
		_ = ws.WriteClose(http.StatusBadRequest)
		return
	}
	if req.Sd.SDP == "" {
		log.Println("empty offer")
		_ = ws.WriteClose(http.StatusBadRequest)
		return
	}
	log.Println("joinChannel: req recieved")
	owner, err := dal.GetUser(h.db, req.OwnerName)
	if err != nil {
		log.Println(fmt.Errorf("error fetching channel owner: %w", err))
		_ = ws.WriteClose(http.StatusBadRequest)
		return
	}

	// add areFriends check? TODO: removeFriend and blockFriend endpoints should remove user
	// from relevant channels in the same DB transaction
	c, err := dal.GetChannelOfMember(h.db, req.ChannelName, user.Id, owner.Id)
	if err != nil {
		log.Println(fmt.Errorf("error getting channel of member: %w", err))
		_ = ws.WriteClose(http.StatusInternalServerError)
		return
	}
	c.Owner = owner.Name

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
		gatherDoneWg sync.WaitGroup
		closed       = make(chan struct{}, 1)
	)
	defer gatherDoneWg.Wait()
	gatherDoneWg.Go(func() {
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

	// create or join room
	// TODO:
	// - if room already exists, obtain list of user object that we need to connect to,
	//   and send candidates to them as soon as they're available
	roomUser := schemas.NewRoomUser(user, &req.Sd)
	room, err := schemas.CreateOrJoinRoom(c, roomUser)
	if err != nil {
		log.Println(fmt.Errorf("error creating or joining channel: %w", err))
		_ = ws.WriteClose(http.StatusInternalServerError)
		return
	}
	log.Println("room created")
	defer room.Leave(roomUser)

	var (
		eventListener             sync.WaitGroup
		listenerCtx, cancelListen = context.WithCancel(ctx)
	)
	defer func() {
		cancelListen()
		_ = ws.Close()
		eventListener.Wait()
	}()
	// NOTE: this should probably be top-level, not in a wg/its own goroutine, as this is the
	// logic that needs to run for the duration of the session/ws.
	// listen for room events. when another user joins, this logic must begin signalling with that user,
	// using this user's candidates that have been gathered.
	eventListener.Go(func() {
		for {
			select {
			case <-listenerCtx.Done():
				return
			case event := <-roomUser.Events:
				fmt.Printf("user %s recieved event [type=%s, user=%s]", roomUser.Name, event.Type, event.User.Name)
				switch event.Type {
				case schemas.JoinEvent:
					// TODO: trigger call flow, ice exchange etc.
					// - subscribe to new user's ICE candidates, and also their answer
					// recipient := &schemas.User{Id: event.User.Id, Name: event.User.Name}
					// call := schemas.CreateCall(user, recipient, *roomUser.Sd)
				case schemas.ExitEvent:
					_ = 0
				default:
					log.Printf("ERROR: unhandled event: %s", event.Type)
				}
			}
		}
	})

	// i think most of this should be pulled out into a function, as the answerCandidates will
	// be gathered for each existing and future member of the room.
	// The caller candidate portion will need to be pulled out into its own goroutine?
	// as it now needs to be done seperately, as soon as the channel is joined.
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
			// TODO: for := subscribers { send candidate }
			call.From.Candidates <- callerCandidate
			fmt.Println("caller candidate sent")
		}
	}
}
