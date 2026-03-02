package requests

// contains http requests used in webrtc signaling processes.

import (
	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
)

// Connection encapsulates offers and answers. It can optionally contain
// the username of the sender and/or the recipient, depending on if additional context is needed.
type Connection struct {
	From,
	To string
	Sd webrtc.SessionDescription
}

// ConnectionWithId extends requests.Connection with user Ids.
type ConnectionWithId struct {
	Connection
	FromId,
	ToId uuid.UUID
}

type JoinChannel struct {
	RoomName,
	OwnerName string
}

type JoinRoom JoinChannel

// BulkConnection is sent to the client when they need to start connecting with multiple
// users in a room. This happens when a client joins a room.
// Users may be empty if noone is in the room.
type BulkConnection struct {
	Users map[uuid.UUID]string
}
