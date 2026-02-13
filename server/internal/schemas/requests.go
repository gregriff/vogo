package schemas

import (
	"github.com/google/uuid"
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

// TODO:
// ConnectionRequest struct { Username string, Sd webrtc.Sd }
// offer := ConnectionRequest
// answer := ConnectionRequest
type AnswerNotificationMessage = CallRequest

type CandidateMessage struct {
	UserId    uuid.UUID
	Username  string
	Candidate webrtc.ICECandidateInit
}

// AnswerRequest is the request data used to answer a 1:1 voice call.
type AnswerRequest struct {
	CallerName string
	Sd         webrtc.SessionDescription
}

type AnswerConnectionMessage = AnswerRequest

type AnswerRoomUserRequest struct {
	AnswerRequest
	CallerId uuid.UUID
}

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
	Data map[uuid.UUID]CallRequest
}
