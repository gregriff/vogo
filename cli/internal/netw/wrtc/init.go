package wrtc

import (
	"fmt"
	"log"

	"github.com/gregriff/vogo/cli/internal/audio"
	"github.com/pion/webrtc/v4"
)

var opusCodec = webrtc.RTPCodecCapability{
	MimeType:     webrtc.MimeTypeOpus,
	ClockRate:    audio.SampleRate,
	Channels:     audio.NumChannels,
	SDPFmtpLine:  "", // "minptime=10;useinbandfec=1",
	RTCPFeedback: nil,
}

// NewAudioPeerConnection creates the PeerConnection for a bidirectional audio webrtc connection.
// It also returns the TrackLocalStaticSample used to write microphone audio to, and two channels,
// one for recieving the client's ICE candidates as they're gathered, and the other for signaling
// when the PeerConnection moves to a connected state.
// TODO: create a struct for this retval
func NewAudioPeerConnection(stunServer, trackID string, exitOnFail bool) (
	*webrtc.PeerConnection,
	*webrtc.TrackLocalStaticSample,
	chan webrtc.ICECandidateInit,
	chan struct{},
	error,
) {
	pc, err := newPeerConnection(stunServer)
	if err != nil {
		ClosePC(pc, true)
		return pc, nil, nil, nil, fmt.Errorf("error creating peer connection %w", err)
	}
	// if _, err = pc.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio); err != nil {
	// 	panic(err)
	// }
	track, err := createAudioTrack(pc, trackID)
	if err != nil {
		ClosePC(pc, true)
		return pc, track, nil, nil, fmt.Errorf("error creating audio track: %w", err)
	}

	var (
		// carries this client's ICE candidates as they're gathered
		candidates = make(chan webrtc.ICECandidateInit, 10)

		// notification channel for when the peer connection becomes connected
		connected = make(chan struct{})
	)
	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		onICECandidate(c, candidates)
	})
	pc.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		onConnectionStateChange(s, connected, exitOnFail)
	})
	return pc, track, candidates, connected, nil
}

// newPeerConnection creates a PeerConnection configured with the Opus audio codec.
// It sets the STUN server and configures the MTU to avoid packet read underruns.
func newPeerConnection(stunServer string) (*webrtc.PeerConnection, error) {
	mediaEngine := &webrtc.MediaEngine{}
	codecParams := webrtc.RTPCodecParameters{
		RTPCodecCapability: opusCodec,
		PayloadType:        111, // should this be negotiated and not hard coded?
	}
	if err := mediaEngine.RegisterCodec(codecParams, webrtc.RTPCodecTypeAudio); err != nil {
		return nil, fmt.Errorf("error registering codec: %w", err)
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
	settingEngine.SetReceiveMTU(3_000)

	api := webrtc.NewAPI(
		webrtc.WithMediaEngine(mediaEngine),
		webrtc.WithSettingEngine(settingEngine),
	)
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{stunServer},
			},
		},
	}
	return api.NewPeerConnection(config)
}

// createAudioTrack configures a PeerConnection with a bidirectional transceiver and creates
// an Opus audio TrackLocalStaticSample, which is returned, to write captured audio to.
func createAudioTrack(pc *webrtc.PeerConnection, trackID string) (*webrtc.TrackLocalStaticSample, error) {
	audioTrsv, err := pc.AddTransceiverFromKind(
		webrtc.RTPCodecTypeAudio,
		webrtc.RTPTransceiverInit{
			Direction: webrtc.RTPTransceiverDirectionSendrecv,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("error adding transceiver: %v", err)
	}

	// setup microphone capture track
	captureTrack, err := webrtc.NewTrackLocalStaticSample(
		opusCodec,
		"captureTrack",
		"captureTrack"+trackID,
	)
	if err != nil {
		return nil, fmt.Errorf("error initalizing capture track: %v", err)
	}
	audioTrsv.Sender().ReplaceTrack(captureTrack)
	return captureTrack, nil
}

func ClosePC(pc *webrtc.PeerConnection, verbose bool) {
	if verbose {
		log.Println("closing peer connection")
	}
	if err := pc.Close(); err != nil {
		fmt.Printf("cannot close peer connection: %v", err)
	}
}
