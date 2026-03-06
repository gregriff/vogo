package wrtc

import (
	"log"

	"github.com/pion/webrtc/v4"
)

// CreateOffer creates a webrtc offer, sets the local description and starts ice gathering.
func CreateOffer(pc *webrtc.PeerConnection) (offer webrtc.SessionDescription) {
	var err error
	if offer, err = pc.CreateOffer(nil); err != nil {
		log.Panicf("error creating offer: %v", err)
	}

	// starts ICE gathering and UDP listeners
	if err = pc.SetLocalDescription(offer); err != nil {
		log.Panicf("error setting local description: %v", err)
	}
	return
}
