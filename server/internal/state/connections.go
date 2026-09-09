// package state contains core logic and state of the server.
package state

import (
	"fmt"
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

// AddNew creates a new connection struct between caller and recipient. It can be retrieved from the connMap with id.
func (m *connMap) AddNew(id uuid.UUID, caller, recipient dal.User, callerSd webrtc.SessionDescription) (connection, error) {
	conn := newConnection(caller, recipient, callerSd)
	m.mu.Lock()
	defer m.mu.Unlock()

	if c, ok := m.conns[id]; ok {
		return *conn, fmt.Errorf("connection already exists: %#v", c)
	}

	m.conns[id] = conn
	return *conn, nil
}

// Get returns a copy of a connection for a given id, returning an error if not found.
// Updating a connection should be done with CallMap.Update.
func (m *connMap) Get(id uuid.UUID) (*connection, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.conns[id]; ok {
		return c, nil
	}
	return &connection{}, fmt.Errorf("connection not found")
}

// Delete removes a call entry from the PendingCalls map. If retry functionality is
// added in the future, this function may not need to be called in most places.
func (m *connMap) Delete(id uuid.UUID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.conns, id)
}

// func (m *connMap) PrintAll(logger *slog.Logger) {
// 	m.mu.Lock()
// 	defer m.mu.Unlock()

// 	logger.Info("PRINTING CONNS:")
// 	for k, v := range m.conns {
// 		logger.Info(fmt.Sprintf("conn %v", k),
// 			"from", v.From.User.Name,
// 			"to", v.To.User.Name,
// 		)
// 	}
// }

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

// connection is the struct that stores the signaling state between two webrtc peers.
type connection struct {
	From,
	To ClientInfo

	CreatedAt time.Time

	// recipient sends their answer here
	Answer chan webrtc.SessionDescription
}

// ClientInfo is the information about a webrtc client needed to create a call or a room.
// It stores data used during the signaling process.
type ClientInfo struct {
	Id   uuid.UUID
	Name string

	// encapsulates the offer or answer of the client
	Sd webrtc.SessionDescription

	// websockets will wait read from these to facilitate ICE trickle
	Candidates chan webrtc.ICECandidateInit
}

// newConnection creates a struct encapsulating a pending connection that is stored in memory
// until the caller and recipient exchange all their ICE candidates. Channels in this
// struct facilitate offer/answer and ICE exchance between the /call and /answer endpoints,
// or when a user joins a room and needs to connect to the existing members.
func newConnection(caller, recipient dal.User, callerSd webrtc.SessionDescription) *connection {
	const maxICECandidates = 15 // should be enough?
	var (
		answerChan          = make(chan webrtc.SessionDescription, 1)
		callerCandidates    = make(chan webrtc.ICECandidateInit, maxICECandidates)
		recipientCandidates = make(chan webrtc.ICECandidateInit, maxICECandidates)
	)
	// TODO: these user attrs could prob be avoided, and prevent a db hit in JoinRoom
	callerClient := ClientInfo{
		Id:         caller.Id,
		Name:       caller.Name,
		Sd:         callerSd,
		Candidates: callerCandidates,
	}
	recipientClient := ClientInfo{
		Id:         recipient.Id,
		Name:       recipient.Name,
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
