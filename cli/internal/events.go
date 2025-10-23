package internal

import (
	"fmt"
	"log"
	"os"

	"github.com/pion/webrtc/v4"
)

func OnICECandidate(candidate *webrtc.ICECandidate) {
	var addr string
	if candidate != nil {
		addr = candidate.Address
	} else {
		addr = "nil!"
	}
	log.Printf("ICE candidate recieved: %s", addr)
}

func OnConnectionStateChange(state webrtc.PeerConnectionState) {
	fmt.Printf("Peer Connection State has changed: %s\n", state.String())

	if state == webrtc.PeerConnectionStateFailed {
		// Wait until PeerConnection has had no network activity for 30 seconds or another failure.
		// It may be reconnected using an ICE Restart.
		// Use webrtc.PeerConnectionStateDisconnected if you are interested in detecting faster timeout.
		// Note that the PeerConnection may come back from PeerConnectionStateDisconnected.
		fmt.Println("Peer Connection has gone to failed exiting")
		os.Exit(0)
	}

	if state == webrtc.PeerConnectionStateClosed {
		// PeerConnection was explicitly closed. This usually happens from a DTLS CloseNotify
		fmt.Println("Peer Connection has gone to closed exiting")
		os.Exit(0)
	}
}
