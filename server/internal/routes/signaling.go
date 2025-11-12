package routes

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gregriff/vogo/server/internal/dal"
	"github.com/gregriff/vogo/server/internal/middleware"
	"github.com/gregriff/vogo/server/internal/schemas"
)

// TODO:
// GET /status: returns to the client any calls or channels currently open and associated with the client
// POST /call: allow client to create a call to one other client. client B will need to accept the call for it to begin. call properties
// 			   stored in memory and wiped when all parties leave it
// POST /channel: allow a client to open a channel. this is persisted to sqlite, including who is invited to join it.
// 				  channels are given a unique name, where the CLI can change properties (who's invited) using the PUT
// PUT /channel: modify channel properties
// DELETE /channel

// Call is used by a client to create a call. The request contains the caller's offer, after
// their ICE gathering has fully completed.
func (h *RouteHandler) Call(w http.ResponseWriter, req *http.Request) {
	rData := schemas.CallRequest{}
	if err := json.NewDecoder(req.Body).Decode(&rData); err != nil {
		panic(err)
	}

	var (
		caller,
		recipient *schemas.User
		sqlErr error
	)
	username := middleware.GetUsername(req)
	if caller, sqlErr = dal.GetUserByUsername(h.db, username); sqlErr != nil {
		log.Println(fmt.Errorf("error fetching caller: %w", sqlErr))
		http.Error(w, "error finding caller", http.StatusInternalServerError)
		return
	}
	if recipient, sqlErr = dal.GetUserByUsername(h.db, rData.RecipientName); sqlErr != nil {
		log.Println(fmt.Errorf("error fetching recipient: %w", sqlErr))
		http.Error(w, "error finding recipient", http.StatusBadRequest)
		return
	}

	log.Println("waiting for call to be answered")
	select {
	case <-req.Context().Done():
		log.Println("call request cancelled by client")
		break // allow cleanup below to run if request is cancelled
	case answererSd := <-schemas.CreateCallAndNotify(*caller, *recipient, rData.Sd):
		log.Println("call has been answered")
		WriteJSON(w, answererSd)

	}
	calls := schemas.GetPendingCalls()
	calls.Delete(caller.Id)
}

// Caller is a GET endpoint that the answerer calls when they want the SD of the caller.
// This returns to the answerer the caller's offer, with all ICE candidates present
func (h *RouteHandler) Caller(w http.ResponseWriter, req *http.Request) {
	callerName := req.PathValue("name")

	log.Println("caller name: ", callerName)

	var (
		caller *schemas.User
		sqlErr error
	)
	username := middleware.GetUsername(req)
	if _, sqlErr = dal.GetUserByUsername(h.db, username); sqlErr != nil {
		log.Println(fmt.Errorf("error fetching recipient: %w", sqlErr))
		http.Error(w, "error finding recipient", http.StatusInternalServerError)
		return
	}
	if caller, sqlErr = dal.GetUserByUsername(h.db, callerName); sqlErr != nil {
		log.Println(fmt.Errorf("error fetching caller: %w", sqlErr))
		http.Error(w, "error finding caller", http.StatusBadRequest)
		return
	}

	calls := schemas.GetPendingCalls()
	call, err := calls.Get(caller.Id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// reply with caller's Sd. client will then use that to resume ICE
	WriteJSON(w, &call.SdFrom)
}

// Answer is used by a client to accept a pending call requested by another client. It sends the caller the
// answerer's answer (with all ICE candidates)
func (h *RouteHandler) Answer(w http.ResponseWriter, req *http.Request) {
	aData := schemas.AnswerRequest{}
	if err := json.NewDecoder(req.Body).Decode(&aData); err != nil {
		panic(err)
	}

	var (
		caller *schemas.User
		sqlErr error
	)
	username := middleware.GetUsername(req)
	if _, sqlErr = dal.GetUserByUsername(h.db, username); sqlErr != nil {
		log.Println(fmt.Errorf("error fetching recipient: %w", sqlErr))
		http.Error(w, "error finding recipient", http.StatusBadRequest)
		return
	}
	if caller, sqlErr = dal.GetUserByUsername(h.db, aData.CallerName); sqlErr != nil {
		log.Println(fmt.Errorf("error fetching caller: %w", sqlErr))
		http.Error(w, "error finding caller", http.StatusInternalServerError)
		return
	}

	if answerErr := schemas.AnswerCall(*caller, aData.Sd); answerErr != nil {
		http.Error(w, answerErr.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(200)
}
