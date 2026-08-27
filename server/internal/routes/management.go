package routes

import (
	"database/sql"
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
		err = fmt.Errorf("password error")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	username, err := dal.CreateUser(h.db, data.Name, hashedPassword, data.InviteCode)
	if err != nil {
		logger.ROUTE.Error("creating new user", "err", err)
		http.Error(w, "error creating new user", http.StatusInternalServerError)
		return
	}

	writeJSON(w, &username)
}

// Status writes a response containing the user's friends and any channels they are a member of.
func (h *RouteHandler) Status(w http.ResponseWriter, req *http.Request) {
	logger := h.loggers.forRequest(req)
	username := middleware.GetUsername(req)

	user, err := dal.GetUser(h.db, username)
	if err != nil {
		httpCode := http.StatusInternalServerError
		httpErr := "error getting user"
		if err == sql.ErrNoRows {
			httpCode = http.StatusBadRequest
			httpErr += ": user not found"
		}
		logger.ROUTE.Error("querying user", "err", err)
		http.Error(w, httpErr, httpCode)
		return
	}

	userId := user.Id.String()
	friends, err := user.Friends(h.db, true)
	if err != nil {
		logger.ROUTE.Error("querying friends", "err", err)
		http.Error(w, "error getting friends", http.StatusInternalServerError)
		return
	}

	channels, err := dal.GetChannels(h.db, userId)
	if err != nil {
		logger.ROUTE.Error("querying channels", "err", err)
		http.Error(w, "error getting channels", http.StatusInternalServerError)
		return
	}

	// TODO: check for any pending calls with callMap
	res := responses.Status{Friends: friends, Channels: channels}
	writeJSON(w, &res)
}

// CreateChannel creates a persistent voice-chat channel.
func (h *RouteHandler) CreateChannel(w http.ResponseWriter, req *http.Request) {
	logger := h.loggers.forRequest(req)
	username := middleware.GetUsername(req)

	user, err := dal.GetUser(h.db, username)
	if err != nil {
		httpCode := http.StatusInternalServerError
		httpErr := "error getting user"
		if err == sql.ErrNoRows {
			httpCode = http.StatusBadRequest
			httpErr += ": user not found"
		}
		logger.ROUTE.Error("querying user", "err", err)
		http.Error(w, httpErr, httpCode)
		return
	}

	channel := requests.CreateChannel{}
	if err := json.NewDecoder(req.Body).Decode(&channel); err != nil {
		logger.ROUTE.Error("decoding create channel request", "err", err)
		http.Error(w, "incorrect data in request", http.StatusBadRequest)
		return
	}

	if len(channel.Name) == 0 {
		logger.ROUTE.Error("user did not specify a channel name")
		http.Error(w, "no channel name specified", http.StatusBadRequest)
		return
	}

	if channel.Capacity < 2 {
		channel.Capacity = shared.ChannelCapacity
	}

	dbChannel, err := dal.CreateChannel(h.db, user.Id, channel)
	if err != nil {
		httpCode := http.StatusInternalServerError
		httpErr := "error creating channel"
		if errors.Is(err, sql.ErrNoRows) {
			httpCode = http.StatusBadRequest
			httpErr += ": channel already exists"
		}
		logger.ROUTE.Error("creating channel", "err", err)
		http.Error(w, httpErr, httpCode)
		return
	}

	writeJSON(w, &dbChannel)
}

// AddFriend creates or accepts a friend request with another user.
func (h *RouteHandler) AddFriend(w http.ResponseWriter, req *http.Request) {
	logger := h.loggers.forRequest(req)
	username := middleware.GetUsername(req)

	user, err := dal.GetUser(h.db, username)
	if err != nil {
		httpCode := http.StatusInternalServerError
		httpErr := "error getting user"
		if err == sql.ErrNoRows {
			httpCode = http.StatusBadRequest
			httpErr += ": user not found"
		}
		logger.ROUTE.Error("querying user", "err", err)
		http.Error(w, httpErr, httpCode)
		return
	}

	data := requests.AddFriend{}
	if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
		logger.ROUTE.Error("decoding add friend request", "err", err)
		http.Error(w, "incorrect data in request", http.StatusBadRequest)
		return
	}

	if len(data.Name) == 0 {
		logger.ROUTE.Error("user did not specify a friend's name")
		http.Error(w, "friend's name not specified", http.StatusBadRequest)
		return
	}

	friend, err := user.AddFriend(h.db, data.Name)
	if err != nil {
		httpCode := http.StatusInternalServerError
		httpErr := "error adding friend"
		if errors.Is(err, sql.ErrNoRows) {
			httpCode = http.StatusBadRequest
			httpErr += fmt.Sprintf(": user with name %s not found", data.Name)
		}
		logger.ROUTE.Error("adding friend", "err", err, "name", data.Name)
		http.Error(w, httpErr, httpCode)
		return
	}

	writeJSON(w, &friend)
}

// InviteFriend invites a friend to an existing channel. Currently they join immediately (without having to accept).
func (h *RouteHandler) InviteFriend(w http.ResponseWriter, req *http.Request) {
	logger := h.loggers.forRequest(req)
	username := middleware.GetUsername(req)

	user, err := dal.GetUser(h.db, username)
	if err != nil {
		httpCode := http.StatusInternalServerError
		httpErr := "error getting user"
		if err == sql.ErrNoRows {
			httpCode = http.StatusBadRequest
			httpErr += ": user not found"
		}
		logger.ROUTE.Error("querying user", "err", err)
		http.Error(w, httpErr, httpCode)
		return
	}

	data := requests.InviteFriend{}
	if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
		logger.ROUTE.Error("decoding invite friend request", "err", err)
		http.Error(w, "incorrect data in request", http.StatusBadRequest)
		return
	}

	if len(data.ChannelName) == 0 {
		logger.ROUTE.Error("user did not specify a channel name")
		http.Error(w, "channel name not specified", http.StatusBadRequest)
		return
	}
	if len(data.FriendName) == 0 {
		logger.ROUTE.Error("user did not specify a friend's name")
		http.Error(w, "friend's name not specified", http.StatusBadRequest)
		return
	}

	friend, err := user.InviteFriend(h.db, data.ChannelName, data.FriendName)
	if err != nil {
		httpCode := http.StatusBadRequest
		httpErr := "error inviting friend: "
		if errors.Is(err, sql.ErrNoRows) {
			httpErr += fmt.Sprintf("user with name %s not found", data.FriendName)
		} else if errors.Is(err, dal.ErrChannelNotFound) {
			httpErr += dal.ErrChannelNotFound.Error()
		} else {
			httpCode = http.StatusInternalServerError
		}
		logger.ROUTE.Error("inviting friend", "err", err, "name", data.FriendName)
		http.Error(w, httpErr, httpCode)
		return
	}

	writeJSON(w, &friend)
}
