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
// Entries are deleted when the recipient answers. Takes a caller's UUID as a key
type CallMap map[uuid.UUID]Call

var (
	pendingCalls    CallMap
	createCallStore sync.Once
)

// GetPendingCalls returns a singleton storing pending calls. Once migrated to websockets, this will be obsolete
func GetPendingCalls() CallMap {
	createCallStore.Do(func() {
		pendingCalls = make(CallMap, 10)
	})
	return pendingCalls
}

// Call is the struct that stores information about a Call
type Call struct {
	// this will be generated when a call is created. not to be created by caller
	Id uuid.UUID

	From,
	To User

	// These are used to communicate the sessionDescriptions during signaling
	SdFrom,
	SdTo webrtc.SessionDescription

	CreatedAt time.Time

	// used to notify the event of the recipient answering the call
	AnswerChan chan webrtc.SessionDescription
}

func CreateCallAndNotify(caller, recipient User, callerSd webrtc.SessionDescription) <-chan webrtc.SessionDescription {
	newCall := Call{
		From:       caller,
		To:         recipient,
		SdFrom:     callerSd,
		CreatedAt:  time.Now(),
		AnswerChan: make(chan webrtc.SessionDescription),
	}
	// add this call to pending map, using caller's ID since a client can only make one call at a time
	calls := GetPendingCalls()
	calls[caller.Id] = newCall

	// return the channel that will return the answerer's Sd
	return newCall.AnswerChan
}

// AnswerCall notifies the blocked caller goroutine that the call has been answered and sends the
// answerer's sessionDescription.
func AnswerCall(caller User, answererSd webrtc.SessionDescription) error {
	calls := GetPendingCalls()

	var (
		call  Call
		found bool
	)
	if call, found = calls[caller.Id]; !found {
		return errors.New("call not found")
	}

	// update call with recipient's SD
	call.SdTo = answererSd
	call.AnswerChan <- answererSd
	calls[caller.Id] = call
	return nil
}

// DeleteCall removes a call entry from the PendingCalls map
func DeleteCall(id uuid.UUID) {
	calls := GetPendingCalls()
	delete(calls, id)
}
