// package state contains core logic and state of the server.
package state

import (
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gregriff/vogo/server/internal/dal"
	"github.com/pion/webrtc/v4"
)

// connMap stores signaling information for pending connections.
// (the time from when a conn is created until it is answered).
// Entries are deleted when the recipient answers or if the connection fails.
// Takes a user's UUID as a key.
type connMap struct {
	mu    sync.Mutex
	conns map[uuid.UUID]*connection
}

// Add inserts or updates a connection for a given user id.
// TODO: move this to a new method? combine with createConnection into a createCall func.
func (m *connMap) Add(id uuid.UUID, call *connection) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.conns[id] = call
}

// Get returns a copy of a connection for a given id, returning an error if not found.
// Updating a connection should be done with CallMap.Update.
func (m *connMap) Get(id uuid.UUID) (*connection, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, exists := m.conns[id]; exists {
		return c, nil
	}
	return &connection{}, errors.New("connection not found")
}

// Delete removes a call entry from the PendingCalls map.
func (m *connMap) Delete(id uuid.UUID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.conns, id)
}

var (
	pendingCalls    connMap
	createCallStore sync.Once
)

// GetPendingCalls returns a singleton storing pending calls.
func GetPendingCalls() *connMap {
	createCallStore.Do(func() {
		pendingCalls = connMap{conns: make(map[uuid.UUID]*connection, 10)}
	})
	return &pendingCalls
}

// connection is the struct that stores the signalling state between two webrtc peers.
type connection struct {
	From,
	To clientInfo

	CreatedAt time.Time

	// recipient sends their answer here
	Answer chan webrtc.SessionDescription
}

// clientInfo is the information about a webrtc client needed to create a call or a room.
// It stores data used during the signaling process.
type clientInfo struct {
	User dal.User

	// encapsulates the offer or answer of the client
	Sd webrtc.SessionDescription

	// websockets will wait read from these to facilitate ICE trickle
	Candidates chan webrtc.ICECandidateInit
}

// CreateConnection creates a struct encapsulating a pending connection that is stored in memory
// until the caller and recipient exchange all their ICE candidates. Channels in this
// struct facilitate offer/answer and ICE exchance between the /call and /answer endpoints,
// or when a user joins a room and needs to connect to the existing members.
func CreateConnection(caller, recipient dal.User, callerSd webrtc.SessionDescription) *connection {
	const maxICECandidates = 15 // should be enough?
	var (
		// TODO: with channel rooms, these chans will need to be per-client
		answerChan          = make(chan webrtc.SessionDescription, 1)
		callerCandidates    = make(chan webrtc.ICECandidateInit, maxICECandidates)
		recipientCandidates = make(chan webrtc.ICECandidateInit, maxICECandidates)
	)
	// TODO: these user attrs could prob be avoided, and prevent a db hit in JoinRoom
	callerClient := clientInfo{
		User:       caller,
		Sd:         callerSd,
		Candidates: callerCandidates,
	}
	recipientClient := clientInfo{
		User:       recipient,
		Sd:         webrtc.SessionDescription{},
		Candidates: recipientCandidates,
	}

	newConn := connection{
		From:      callerClient,
		To:        recipientClient,
		CreatedAt: time.Now(),
		Answer:    answerChan,
	}
	return &newConn
}
