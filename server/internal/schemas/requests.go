package schemas

import (
	"github.com/pion/webrtc/v4"
)

// NewUserRequest is the request data to register a new client with the server.
type NewUserRequest struct {
	Name,
	Password,
	InviteCode string
}

type AddFriendRequest struct {
	Name string
}

// CallRequest is the request data used to create a call from one client to another.
type CallRequest struct {
	RecipientName string
	Sd            webrtc.SessionDescription
}

// AnswerRequest is the request data used to answer a 1:1 voice call.
type AnswerRequest struct {
	CallerName string
	Sd         webrtc.SessionDescription
}

type CreateChannelRequest struct {
	Name,
	Description string
	Capacity int
}
