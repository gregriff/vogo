package configs

import (
	"fmt"

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

func NewPeerConnection(stunServer string) (*webrtc.PeerConnection, error) {
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

func CreateAudioTrack(pc *webrtc.PeerConnection, trackID string) (*webrtc.TrackLocalStaticSample, error) {
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
