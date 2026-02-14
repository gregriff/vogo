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

// ConnectionRequest encapsulates offers and answers. It can optionally contain
// the username of the sender and/or the recipient, depending on if additional context is needed.
type ConnectionRequest struct {
	From,
	To string
	Sd webrtc.SessionDescription
}

// ConnectionRequestWithId extends ConnectionRequest with user Ids.
type ConnectionRequestWithId struct {
	ConnectionRequest
	FromId,
	ToId uuid.UUID
}

type CandidateMessage struct {
	UserId    uuid.UUID
	Username  string
	Candidate webrtc.ICECandidateInit
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
	Data map[uuid.UUID]ConnectionRequest
}
