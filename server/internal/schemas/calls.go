// package schemas contains structs representing database records, http request data,
// or core internal state of the server.
package schemas

import (
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
)

// callMap stores signaling information for pending calls.
// (the time from when a call is created until it is answered).
// Entries are deleted when the recipient answers or if the call fails.
// Takes a caller's UUID as a key
type callMap struct {
	mu    sync.Mutex
	calls map[uuid.UUID]call
}

// update inserts or updates a call for a given id
func (m *callMap) update(id uuid.UUID, call call) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls[id] = call
}

// Get returns a copy of a call Call for a given id, returning an error if not found.
// Updating a call should be done with CallMap.Update
func (m *callMap) Get(id uuid.UUID) (call, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, exists := m.calls[id]; exists {
		return c, nil
	} else {
		return call{}, errors.New("call not found")
	}
}

// Delete removes a call entry from the PendingCalls map
func (m *callMap) Delete(id uuid.UUID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.calls, id)
}

var (
	pendingCalls    callMap
	createCallStore sync.Once
)

// GetPendingCalls returns a singleton storing pending calls. Once migrated to websockets, this will be obsolete?
func GetPendingCalls() *callMap {
	createCallStore.Do(func() {
		pendingCalls = callMap{calls: make(map[uuid.UUID]call, 10)}
	})
	return &pendingCalls
}

// call is the struct that stores information about a call
type call struct {
	// this will be generated when a call is created. not to be created by caller
	Id uuid.UUID

	From,
	To clientInfo

	CreatedAt time.Time

	// recipient sends their answer here
	Answer chan webrtc.SessionDescription
}

// clientInfo is the information about a webrtc client needed to create a call or a channel.
// It stores data used during the signaling process.
type clientInfo struct {
	user *User

	// encapsulates the offer or answer of the client
	Sd webrtc.SessionDescription

	// websockets will wait read from these to facilitate ICE trickle
	Candidates chan webrtc.ICECandidateInit
}

// Create Call creates a struct encapsulating a pending call that is stored in memory
// until the caller and recipient exchange all their ICE candidates. Channels in this
// struct facilitate offer/answer and ICE exchance between the /call and /answer endpoints.
func CreateCall(caller, recipient *User, callerSd webrtc.SessionDescription) *call {
	const maxICECandidates = 10 // should be enough?
	var (
		// TODO: with channel rooms, these chans will need to be per-client
		answerChan          = make(chan webrtc.SessionDescription, 1)
		callerCandidates    = make(chan webrtc.ICECandidateInit, maxICECandidates)
		recipientCandidates = make(chan webrtc.ICECandidateInit, maxICECandidates)
	)
	callerClient := clientInfo{
		user:       caller,
		Sd:         callerSd,
		Candidates: callerCandidates,
	}
	recipientClient := clientInfo{
		user:       recipient,
		Sd:         webrtc.SessionDescription{},
		Candidates: recipientCandidates,
	}

	newCall := call{
		From:      callerClient,
		To:        recipientClient,
		CreatedAt: time.Now(),
		Answer:    answerChan,
	}
	// add this call to pending map, using caller's ID since a client can only make one call at a time
	calls := GetPendingCalls()
	calls.update(caller.Id, newCall)
	return &newCall
}
