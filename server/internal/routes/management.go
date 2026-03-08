package routes

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gregriff/vogo/server/internal/crypto"
	"github.com/gregriff/vogo/server/internal/dal"
	"github.com/gregriff/vogo/server/internal/middleware"
	"github.com/gregriff/vogo/server/internal/validation"
	"github.com/gregriff/vogo/shared"
	"github.com/gregriff/vogo/shared/requests"
	"github.com/gregriff/vogo/shared/responses"
)

func (h *RouteHandler) Register(w http.ResponseWriter, req *http.Request) {
	logger := h.loggers.forRequest(req)
	data := requests.NewUser{}
	if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	logger.ROUTE.Info("new user", "name", data.Name, "inviteCode", data.InviteCode)

	statusCode, err := validation.CheckRegistrationCredentials(h.db, data.InviteCode, data.Name, data.Password)
	if err != nil {
		http.Error(w, err.Error(), statusCode)
		return
	}

	hashedPassword, err := crypto.HashPassword(data.Password)
	if err != nil {
		logger.ROUTE.Error("hashing password", "err", err)
		err = errors.New("password error")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	username, err := dal.CreateUser(h.db, data.Name, hashedPassword, data.InviteCode)
	if err != nil {
		logger.ROUTE.Error("creating new user", "err", err)
		err = errors.New("error creating new user")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, &username)
}

// Status writes a response containing the user's friends and any channels they are a member of.
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
	res := responses.Status{Friends: friends, Channels: channels}
	writeJSON(w, &res)
}

// CreateChannel creates a persistent voice-chat channel.
func (h *RouteHandler) CreateChannel(w http.ResponseWriter, req *http.Request) {
	username := middleware.GetUsername(req)

	user, err := dal.GetUser(h.db, username)
	if err != nil {
		err = fmt.Errorf("error getting user: %w", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	channel := requests.CreateChannel{}
	if err := json.NewDecoder(req.Body).Decode(&channel); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if len(channel.Name) == 0 {
		err = errors.New("no channel name specified")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if channel.Capacity < 2 {
		channel.Capacity = shared.ChannelCapacity
	}

	dbChannel, err := dal.CreateChannel(h.db, user.Id, channel)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, &dbChannel)
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

	data := requests.AddFriend{}
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

	writeJSON(w, &friend)
}

// InviteFriend invites a friend to an existing channel. Currently they join immediately (without having to accept).
func (h *RouteHandler) InviteFriend(w http.ResponseWriter, req *http.Request) {
	username := middleware.GetUsername(req)

	user, err := dal.GetUser(h.db, username)
	if err != nil {
		err = fmt.Errorf("error getting user: %w", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := requests.InviteFriend{}
	if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if len(data.ChannelName) == 0 {
		err = errors.New("no channel name specified")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(data.FriendName) == 0 {
		err = errors.New("no friend name specified")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	friend, err := dal.InviteFriend(h.db, user.Id, data.ChannelName, data.FriendName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, &friend)
}
