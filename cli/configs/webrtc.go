package configs

import (
	"github.com/gregriff/vogo/cli/internal/audio"
	"github.com/pion/webrtc/v4"
)

var OpusCodecCapability = webrtc.RTPCodecCapability{
	MimeType:     webrtc.MimeTypeOpus,
	ClockRate:    audio.SampleRate,
	Channels:     audio.NumChannels,
	SDPFmtpLine:  "", // "minptime=10;useinbandfec=1",
	RTCPFeedback: nil,
}

func NewWebRTC() *webrtc.API {
	mediaEngine := &webrtc.MediaEngine{}
	codecParams := webrtc.RTPCodecParameters{
		RTPCodecCapability: OpusCodecCapability,
		PayloadType:        111, // should this be negotiated and not hard coded?
	}
	if err := mediaEngine.RegisterCodec(codecParams, webrtc.RTPCodecTypeAudio); err != nil {
		panic(err)
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
	return api
}
