package routes

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/gregriff/vogo/server/internal/crypto"
	"github.com/gregriff/vogo/server/internal/dal"
	"github.com/gregriff/vogo/server/internal/middleware"
	"github.com/gregriff/vogo/server/internal/schemas"
	"github.com/gregriff/vogo/server/internal/schemas/public"
	"github.com/gregriff/vogo/server/internal/validation"
)

func (h *RouteHandler) Register(w http.ResponseWriter, req *http.Request) {
	data := schemas.NewUserRequest{}
	if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("new user parsed: %#v", data)

	err, statusCode := validation.CheckRegistrationCredentials(h.db, data.InviteCode, data.Name, data.Password)
	if err != nil {
		http.Error(w, err.Error(), statusCode)
		return
	}

	hashedPassword, err := crypto.HashPassword(data.Password)
	if err != nil {
		log.Println(err.Error())
		err = errors.New("password error")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	username, err := dal.CreateUser(h.db, data.Name, hashedPassword, data.InviteCode)
	if err != nil {
		log.Println(err.Error())
		err = errors.New("error creating new user")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	WriteJSON(w, &username)
}

func (h *RouteHandler) Status(w http.ResponseWriter, req *http.Request) {
	username := middleware.GetUsername(req)

	user, err := dal.GetUser(h.db, username)
	if err != nil {
		err = fmt.Errorf("error getting user: %w", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	userId := user.Id.String()
	friends, err := dal.GetFriends(h.db, userId, true)
	if err != nil {
		err = fmt.Errorf("error getting friends: %w", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	channels, err := dal.GetChannels(h.db, userId)
	if err != nil {
		err = fmt.Errorf("error getting channels: %w", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// TODO: check for any pending calls with callMap
	res := public.StatusResponse{Friends: friends, Channels: channels}
	WriteJSON(w, &res)
}

// AddFriend creates or accepts a friend request with another user.
func (h *RouteHandler) AddFriend(w http.ResponseWriter, req *http.Request) {
	username := middleware.GetUsername(req)

	user, err := dal.GetUser(h.db, username)
	if err != nil {
		err = fmt.Errorf("error getting user: %w", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := schemas.AddFriendRequest{}
	if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if len(data.Name) == 0 {
		err = errors.New("no name specified")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	friend, err := dal.AddFriend(h.db, user.Id, data.Name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	res := public.User{Name: friend.Name}
	WriteJSON(w, &res)
}
