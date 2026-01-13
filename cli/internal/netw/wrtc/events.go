// package wrtc wraps pion webrtc capabilities
package wrtc

import (
	"fmt"
	"log"
	"os"

	"github.com/pion/webrtc/v4"
)

func onICECandidate(candidate *webrtc.ICECandidate, ch chan<- webrtc.ICECandidateInit) {
	addr := "nil!"
	if candidate != nil {
		addr = candidate.Address
	}
	log.Printf("ICE candidate recieved: %s", addr)

	if candidate == nil {
		close(ch)
		return
	}
	ch <- candidate.ToJSON()
}

func onConnectionStateChange(state webrtc.PeerConnectionState, ch chan<- struct{}, exitOnFail bool) {
	fmt.Printf("Peer Connection State has changed: %s\n", state.String())

	if state == webrtc.PeerConnectionStateConnected {
		ch <- struct{}{}
	}

	if !exitOnFail {
		return
	}

	if state == webrtc.PeerConnectionStateFailed {
		// Wait until PeerConnection has had no network activity for 30 seconds or another failure.
		// It may be reconnected using an ICE Restart.
		// Use webrtc.PeerConnectionStateDisconnected if you are interested in detecting faster timeout.
		// Note that the PeerConnection may come back from PeerConnectionStateDisconnected.
		os.Exit(0)
	}
	// if PeerConnection was explicitly closed, this usually happens from a DTLS CloseNotify
	if state == webrtc.PeerConnectionStateClosed {
		os.Exit(0)
	}
}
