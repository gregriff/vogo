package wrtc

import (
	"fmt"
	"log"

	"github.com/gregriff/vogo/cli/internal/audio"
	"github.com/pion/webrtc/v4"
)

const RecvMTU = 1_200

var opusCodec = webrtc.RTPCodecCapability{
	MimeType:     webrtc.MimeTypeOpus,
	ClockRate:    audio.SampleRate,
	Channels:     audio.NumChannels,
	SDPFmtpLine:  "", // "minptime=10;useinbandfec=1",
	RTCPFeedback: nil,
}

// NewAudioPeerConnection creates the PeerConnection for a bidirectional audio webrtc connection.
// It also returns the TrackLocalStaticSample used to write microphone audio to, and two channels,
// one for receiving the client's ICE candidates as they're gathered, and the other for signaling
// when the PeerConnection moves to a connected state.
// TODO: create a struct for this retval.
func NewAudioPeerConnection(stunServer string, track *webrtc.TrackLocalStaticSample, exitOnFail bool) (
	*webrtc.PeerConnection,
	<-chan webrtc.ICECandidateInit,
	<-chan webrtc.PeerConnectionState,
	<-chan struct{},
) {
	pc := newPeerConnection(stunServer)
	addAudioTrack(pc, track)

	var (
		// carries this client's ICE candidates as they're gathered
		candidates = make(chan webrtc.ICECandidateInit, 10)

		// notification channel for when the peer connection becomes connected
		connected = make(chan struct{})

		// channel to pass along connection status updates
		updates = make(chan webrtc.PeerConnectionState)
	)
	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		onICECandidate(c, candidates)
	})
	pc.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		onConnectionStateChange(s, updates, connected, exitOnFail)
	})
	return pc, candidates, updates, connected
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
	// interceptorRegistry := &interceptor.Registry{}

	// jitterBufferFactory, err := jitterbuffer.NewInterceptor()
	// if err != nil {
	// 	panic(err)
	// }
	// interceptorRegistry.Add(jitterBufferFactory)

	// Use the default set of Interceptors
	// if err = webrtc.RegisterDefaultInterceptors(mediaEngine, interceptorRegistry); err != nil {
	// 	panic(err)
	// }

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
		_ = ClosePC(pc, true)
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
func addAudioTrack(pc *webrtc.PeerConnection, track *webrtc.TrackLocalStaticSample) {
	audioTrsv, err := pc.AddTransceiverFromKind(
		webrtc.RTPCodecTypeAudio,
		webrtc.RTPTransceiverInit{
			Direction: webrtc.RTPTransceiverDirectionSendrecv,
		},
	)
	if err != nil {
		_ = ClosePC(pc, true)
		log.Panicf("error adding transceiver: %v", err)
	}
	if err = audioTrsv.Sender().ReplaceTrack(track); err != nil {
		_ = ClosePC(pc, true)
		log.Panicf("error replacing track: %v", err)
	}
}

func ClosePC(pc *webrtc.PeerConnection, verbose bool) error {
	if verbose {
		log.Println("closing peer connection")
	}
	if err := pc.Close(); err != nil {
		return fmt.Errorf("cannot close peer connection: %w", err)
	}
	return nil
}
