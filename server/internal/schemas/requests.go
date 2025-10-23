package schemas

import (
	"github.com/pion/webrtc/v4"
)

type NewUserRequest struct {
	Username,
	Password,
	InviteCode string
}

// CallRequest is created from the request body of the POST /call endpoint
type CallRequest struct {
	RecipientName string
	Sd            webrtc.SessionDescription
}

type AnswerRequest struct {
	CallerName string
	Sd         webrtc.SessionDescription
}
