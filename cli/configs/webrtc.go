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

// TODO : call this to get rid of packetio errors
// // SetReceiveMTU sets the size of read buffer that copies incoming packets. This is optional.
// Leave this 0 for the default receiveMTU
// func (e *SettingEngine) SetReceiveMTU(receiveMTU uint) {
// e.receiveMTU = receiveMTU
// }
func NewWebRTC() *webrtc.API {
	// Create a MediaEngine object to configure the supported codec
	mediaEngine := &webrtc.MediaEngine{}

	// setup opus codec
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
