// package wrtc wraps pion webrtc capabilities
package wrtc

import (
	"log"
	"os"

	"github.com/pion/webrtc/v4"
)

func onICECandidate(candidate *webrtc.ICECandidate, ch chan<- webrtc.ICECandidateInit) {
	addr := "nil!"
	if candidate != nil {
		addr = candidate.Address
	}
	log.Printf("ICE candidate gathered: %s", addr)

	if candidate == nil {
		close(ch)
		return
	}
	ch <- candidate.ToJSON()
}

func onSignalingStateChange(state webrtc.SignalingState, from, to string) {
	log.Printf("%s's connection with %s signaling state changed to %s", from, to, state.String())
}

func onConnectionStateChange(
	state webrtc.PeerConnectionState,
	ch chan<- webrtc.PeerConnectionState,
	connected chan<- struct{},
	exitOnFail bool,
) {
	ch <- state

	if state == webrtc.PeerConnectionStateConnected {
		close(connected)
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
