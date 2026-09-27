// package wrtc contains functions that initialize pion webrtc functionality, as well as functions to
// read RTCP packets
package wrtc

import (
	"log"

	"github.com/gregriff/vogo/cli/internal/audio/pcm"
	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/stats"
	"github.com/pion/webrtc/v4"
)

const RecvMTU = 1_200

var opusCodec = webrtc.RTPCodecCapability{
	MimeType:     webrtc.MimeTypeOpus,
	ClockRate:    pcm.SampleRate,
	Channels:     pcm.NumChannels,
	SDPFmtpLine:  "", // "minptime=10",
	RTCPFeedback: nil,
}

// NewAudioPeerConnection creates the PeerConnection for a bidirectional audio webrtc connection.
// It also returns the RTP sender and receiver, so the caller can access sender and receiver reports
// for the peer connection.
func NewAudioPeerConnection(stunServer string, track *webrtc.TrackLocalStaticSample) (
	*webrtc.PeerConnection,
	*webrtc.RTPSender,
	*webrtc.RTPReceiver,
) {
	pc := newPeerConnection(stunServer)
	sender, receiver := addAudioTrack(pc, track)
	if sender == nil {
		log.Panicf("RTPSender is nil")
	}
	if receiver == nil {
		log.Panicf("RTPReceiver is nil")
	}
	return pc, sender, receiver
}

// newPeerConnection creates a PeerConnection configured with the Opus audio codec.
// It sets the STUN server and configures the MTU to avoid packet read underruns.
// TODO: config creation and PeerConnection creation needs to be split into separate functions for room use case.
func newPeerConnection(stunServer string) *webrtc.PeerConnection {
	mediaEngine := &webrtc.MediaEngine{}
	codecParams := webrtc.RTPCodecParameters{
		RTPCodecCapability: opusCodec,
		PayloadType:        111, // should this be negotiated and not hard coded?
	}
	if err := mediaEngine.RegisterCodec(codecParams, webrtc.RTPCodecTypeAudio); err != nil {
		log.Panicf("error registering codec: %v", err)
	}

	// Create a InterceptorRegistry. This is the user configurable RTP/RTCP Pipeline.
	// This provides NACKs, RTCP Reports and other features. If you use `webrtc.NewPeerConnection`
	// this is enabled by default. If you are manually managing You MUST create a InterceptorRegistry
	// for each PeerConnection.
	interceptorRegistry := &interceptor.Registry{}

	// TODO: to impl jitter buffer (with custom minpacketcount), will need to create custom receiver_interceptor
	// that stores a jitterbuffer.New(jitterbuffer.WithMinimumPacketCount(2)) in it.
	// https://github.com/pion/interceptor/blob/master/pkg/jitterbuffer/receiver_interceptor.go
	// jitterBufferFactory, err := jitterbuffer.NewInterceptor()
	// if err != nil {
	// panic(err)
	// }
	// interceptorRegistry.Add(jitterBufferFactory)

	// Use the default set of Interceptors
	if err := webrtc.RegisterDefaultInterceptors(mediaEngine, interceptorRegistry); err != nil {
		panic(err)
	}

	statsFactory, err := stats.NewInterceptor()
	if err != nil {
		panic(err)
	}

	interceptorRegistry.Add(statsFactory)

	// not sure if this should be avoided but this prevents packet size overruns
	settingEngine := webrtc.SettingEngine{}
	settingEngine.SetReceiveMTU(RecvMTU)

	// Possible todo: ICE renomination
	// For advanced use with a custom generator and interval.
	// interval := 2 * time.Second
	// customGen := func() uint32 { return uint32(time.Now().UnixNano()) } // example

	// if err := se.SetICERenomination(
	// 	webrtc.WithRenominationGenerator(customGen),
	// 	webrtc.WithRenominationInterval(interval),
	// ); err != nil {
	// 	log.Println(err)
	// }

	api := webrtc.NewAPI(
		webrtc.WithMediaEngine(mediaEngine),
		webrtc.WithSettingEngine(settingEngine),
	)
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{stunServer}},
		},
	}
	pc, err := api.NewPeerConnection(config)
	if err != nil {
		log.Panicf("error creating PeerConnection: %v", err)
	}
	return pc
}

// CreateAudioTrack creates the opus audio track.
func CreateAudioTrack(trackId string) *webrtc.TrackLocalStaticSample {
	t, err := webrtc.NewTrackLocalStaticSample(opusCodec, "captureTrack", trackId)
	if err != nil {
		log.Panicf("error initializing capture track: %v", err)
	}
	return t
}

// addAudioTrack configures a PeerConnection with a bidirectional transceiver and adds the track to it.
func addAudioTrack(pc *webrtc.PeerConnection, track *webrtc.TrackLocalStaticSample) (*webrtc.RTPSender, *webrtc.RTPReceiver) {
	audioTrsv, err := pc.AddTransceiverFromKind(
		webrtc.RTPCodecTypeAudio,
		webrtc.RTPTransceiverInit{
			Direction: webrtc.RTPTransceiverDirectionSendrecv,
		},
	)
	if err != nil {
		log.Panicf("error adding transceiver: %v", err)
	}
	sender := audioTrsv.Sender()
	if err = sender.ReplaceTrack(track); err != nil {
		log.Panicf("error replacing track: %v", err)
	}
	return sender, audioTrsv.Receiver()
}
