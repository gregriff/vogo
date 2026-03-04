package wrtc

import (
	"fmt"
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

// CreateAnswer sets the remote description of the caller given their offer,
// creates an answer, sets the local description and starts ice gathering.
// It returns the answer, but the up-to-date local description should probably
// be accessed directly from the pc.
func CreateAnswer(pc *webrtc.PeerConnection, offer *webrtc.SessionDescription) (answer webrtc.SessionDescription, err error) {
	if err = pc.SetRemoteDescription(*offer); err != nil {
		err = fmt.Errorf("error setting remote description: %v", err)
		return
	}

	if answer, err = pc.CreateAnswer(nil); err != nil {
		err = fmt.Errorf("error starting pc or generating local description: %v", err)
		return
	}

	// starts ICE gathering and UDP listeners
	if err = pc.SetLocalDescription(answer); err != nil {
		err = fmt.Errorf("error setting local description: %v", err)
	}
	return
}
