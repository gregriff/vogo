package configs

import (
	"github.com/pion/webrtc/v4"
)

func NewWebRTC() *webrtc.API {
	// Create a MediaEngine object to configure the supported codec
	mediaEngine := &webrtc.MediaEngine{}

	// setup opus codec
	if err := mediaEngine.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 0, SDPFmtpLine: "", RTCPFeedback: nil,
		},
	}, webrtc.RTPCodecTypeAudio); err != nil {
		panic(err)
	}

	// Create a InterceptorRegistry. This is the user configurable RTP/RTCP Pipeline.
	// This provides NACKs, RTCP Reports and other features. If you use `webrtc.NewPeerConnection`
	// this is enabled by default. If you are manually managing You MUST create a InterceptorRegistry
	// for each PeerConnection.
	// interceptorRegistry := &interceptor.Registry{}

	// Register a intervalpli factory
	// intervalPliFactory, err := intervalpli.NewReceiverInterceptor()
	// if err != nil {
	// 	panic(err)
	// }
	// interceptorRegistry.Add(intervalPliFactory)

	// Use the default set of Interceptors
	// if err = webrtc.RegisterDefaultInterceptors(mediaEngine, interceptorRegistry); err != nil {
	// 	panic(err)
	// }

	// Create the API object with the MediaEngine
	api := webrtc.NewAPI(webrtc.WithMediaEngine(mediaEngine))
	return api
}
