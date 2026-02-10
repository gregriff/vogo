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

type InviteFriendRequest struct {
	ChannelName,
	FriendName string
}

// CallRequest is the request data used to create a call from one client to another.
type CallRequest struct {
	RecipientName string
	Sd            webrtc.SessionDescription
}

type AnswerNotificationMessage = CallRequest

type CandidateMessage struct {
	Username  string
	Candidate webrtc.ICECandidateInit
}

// AnswerRequest is the request data used to answer a 1:1 voice call.
type AnswerRequest struct {
	CallerName string
	Sd         webrtc.SessionDescription
}

type AnswerConnectionMessage = AnswerRequest

type CreateChannelRequest struct {
	Name,
	Description string
	Capacity int
}

type JoinRoomRequest struct {
	RoomName,
	OwnerName string
}

// BulkConnectionMessage is sent from the client when they are joining a room and
// have prepared offers for all users currently in the channel. Data maps the usernames
// to the offer being made to them.
type BulkConnectionMessage struct {
	Data map[string]webrtc.SessionDescription
}
