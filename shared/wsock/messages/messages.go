// package messages contains event-driven data sent over websocket.
package messages

import (
	"github.com/google/uuid"
	"github.com/gregriff/vogo/shared/requests"
	"github.com/pion/webrtc/v4"
)

// BulkConnection is sent from the client when they are joining a room and
// have prepared offers for all users currently in the channel. Data maps the usernames
// to the offer being made to them.
type BulkConnection struct {
	Data map[uuid.UUID]requests.Connection
}

// Candidate is an ICE Candidate. The Message that wraps it contains a Type field
// indicating whether its an offer or answer ICE candidate.
type Candidate struct {
	UserId    uuid.UUID
	Username  string
	Candidate webrtc.ICECandidateInit
}
