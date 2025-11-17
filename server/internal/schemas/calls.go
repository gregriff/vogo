package schemas

import (
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
)

// CallMap stores signaling information for pending calls.
// (the time from when a call is created until it is answered).
// Entries are deleted when the recipient answers or if the call fails.
// Takes a caller's UUID as a key
type CallMap struct {
	mu    sync.Mutex
	calls map[uuid.UUID]Call
}

// Update inserts or updates a call for a given id
func (m *CallMap) Update(id uuid.UUID, call Call) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls[id] = call
}

// Get returns a copy of a call Call for a given id, returning an error if not found.
// Updating a call should be done with CallMap.Update
func (m *CallMap) Get(id uuid.UUID) (Call, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if call, exists := m.calls[id]; exists {
		return call, nil
	} else {
		return Call{}, errors.New("call not found")
	}
}

// Delete removes a call entry from the PendingCalls map
func (m *CallMap) Delete(id uuid.UUID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.calls, id)
}

var (
	pendingCalls    CallMap
	createCallStore sync.Once
)

// GetPendingCalls returns a singleton storing pending calls. Once migrated to websockets, this will be obsolete?
func GetPendingCalls() *CallMap {
	createCallStore.Do(func() {
		pendingCalls = CallMap{calls: make(map[uuid.UUID]Call, 10)}
	})
	return &pendingCalls
}

// Call is the struct that stores information about a Call
type Call struct {
	// this will be generated when a call is created. not to be created by caller
	Id uuid.UUID

	From,
	To ClientInfo

	CreatedAt time.Time

	// recipient sends their answer here
	Answer chan webrtc.SessionDescription
}

// ClientInfo is the information about a webrtc client needed to create a call or a channel.
// It stores data used during the signaling process.
type ClientInfo struct {
	user *User

	// encapsulates the offer or answer of the client
	Sd webrtc.SessionDescription

	// websockets will wait read from these to facilitate ICE trickle
	Candidates chan webrtc.ICECandidateInit
}

// Create Call creates a struct encapsulating a pending call that is stored in memory
// until the caller and recipient exchange all their ICE candidates. Channels in this
// struct facilitate offer/answer and ICE exchance between the /call and /answer endpoints.
func CreateCall(caller, recipient *User, callerSd webrtc.SessionDescription) *Call {
	const maxICECandidates = 10 // should be enough?
	var (
		// TODO: with channel rooms, these chans will need to be per-client
		answerChan          = make(chan webrtc.SessionDescription, 1)
		callerCandidates    = make(chan webrtc.ICECandidateInit, maxICECandidates)
		recipientCandidates = make(chan webrtc.ICECandidateInit, maxICECandidates)
	)
	callerClient := ClientInfo{
		user:       caller,
		Sd:         callerSd,
		Candidates: callerCandidates,
	}
	recipientClient := ClientInfo{
		user:       recipient,
		Sd:         webrtc.SessionDescription{},
		Candidates: recipientCandidates,
	}

	newCall := Call{
		From:      callerClient,
		To:        recipientClient,
		CreatedAt: time.Now(),
		Answer:    answerChan,
	}
	// add this call to pending map, using caller's ID since a client can only make one call at a time
	calls := GetPendingCalls()
	calls.Update(caller.Id, newCall)
	return &newCall
}
