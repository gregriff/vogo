//go:build cgo

// package audio contains abstractions over microphone and speaker hardware, PCM buffer manipulation routines,
// and higher-level, voice-call and voice-channel structs for controlling audio in those configurations.
// package audio
package audio

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gen2brain/malgo"
	"github.com/gregriff/vogo/cli/internal/audio/pcm"
	"github.com/gregriff/vogo/shared"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"

	"github.com/hraban/opus"
)

const (
	// size of buffer to hold opus encoded from mic, to be written to packets.
	opusBufferSize = pcm.FrameSize

	// opusBitrate sets the bitrate of the opus encoder.
	opusBitrate = 64_000
)

// microphone provides access to the device's
// microphone, to record opus audio data.
type microphone struct {
	ctx    *malgo.AllocatedContext
	device *malgo.Device

	// this is where malgo writes microphone pcm data
	pcm pcm.Stream

	// the webrtc track that we use write opus to
	track *webrtc.TrackLocalStaticSample

	// initialized will be closed when the microphone device is initialized.
	initialized chan struct{}

	// mic will send failed PC Ids on this chan
	failedPeers chan error

	// the malgo context will be sent over this chan.
	ctxChan chan *malgo.AllocatedContext
}

func newMicrophone(track *webrtc.TrackLocalStaticSample) microphone {
	return microphone{
		ctx:         &malgo.AllocatedContext{},
		device:      &malgo.Device{},
		track:       track,
		pcm:         pcm.NewStream(),
		initialized: make(chan struct{}),
		failedPeers: make(chan error, shared.ChannelCapacity-1),
		ctxChan:     make(chan *malgo.AllocatedContext),
	}
}

func (m *microphone) Init(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return nil
	case m.ctx = <-m.ctxChan:
		break
	}

	start := time.Now()
	defer func() {
		log.Printf("mic initialized in %v", time.Since(start))
	}()

	cfg := malgo.DefaultDeviceConfig(malgo.Capture)
	cfg.Capture.Format = pcm.AudioFormat
	cfg.Capture.Channels = pcm.NumChannels
	cfg.SampleRate = pcm.SampleRate
	cfg.PeriodSizeInMilliseconds = pcm.CapturePeriodMs

	// this controls quality?
	// config.Resampling.Linear.LpfOrder = ?
	// config.Resampling.Algorithm = malgo.ResampleAlgorithmSpeex

	// write mic pcm to capture buffer, to send to network. this fires every X milliseconds
	onRecvFrames := func(_, pInputSample []byte, _ uint32) { // uint32 is framecount
		m.pcm.WriteFrame(pcm.BytesToInt16(pInputSample))
	}

	// init playback device
	device, err := malgo.InitDevice(m.ctx.Context, cfg, malgo.DeviceCallbacks{
		Data: onRecvFrames,
	})
	m.device = device
	if err != nil {
		return fmt.Errorf("error creating mic device: %w", err)
	}
	close(m.initialized)
	return nil
}

// Start starts the microphone, and runs a loop that writes the mic data to the TrackLocal,
// until ctx is cancelled.
func (m *microphone) Start(ctx context.Context) error {
	if err := m.device.Start(); err != nil {
		return err
	}

	// TODO: this should prob be an array
	frameBuffer := make([]int16, pcm.FrameSize)
	opusBuffer := make([]byte, opusBufferSize)
	encoder, err := opus.NewEncoder(pcm.SampleRate, pcm.NumChannels, opus.AppVoIP)
	if err != nil {
		return fmt.Errorf("encoder error: %w", err)
	}
	if err := encoder.SetMaxBandwidth(opus.Fullband); err != nil {
		log.Panicf("error setting max bandwidth: %v", err)
	}
	if err := encoder.SetBitrate(opusBitrate); err != nil {
		log.Panicf("error setting bitrate: %v", err)
	}
	if err := encoder.SetDTX(true); err != nil {
		log.Panicf("error setting DTX: %v", err)
	}
	// complexity, _ := encoder.Complexity()  // currently it's == 9, 10 max

	ticker := time.NewTicker(pcm.FrameDuration)
	defer ticker.Stop()

	// loop to encode buffered PCM into opus and send to network
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			// NOTE: may need to ensure this read is limited
			if n := m.pcm.ReadFrame(frameBuffer); n == 0 {
				continue
			}

			// encode to opus
			bytesEncoded, err := encoder.Encode(frameBuffer, opusBuffer)
			if err != nil {
				log.Println("OPUS ENCODE ERROR:", err)
				continue
			}

			// write to webrtc track
			failed := m.track.WriteSample(media.Sample{
				Data:     opusBuffer[:bytesEncoded], // only the first N bytes are opus data.
				Duration: pcm.FrameDuration,
			})

			if failed != nil {
				log.Println("WriteSample error, contains failed peers:", err)
				m.failedPeers <- failed
				continue
			}
		}
	}
}

func (m *microphone) Uninit() {
	if m.device != nil {
		m.device.Uninit()
	}
	log.Println("uninit and freed mic")
}

// Track returns the webrtc Track where microphone audio is written.
func (m *microphone) Track() *webrtc.TrackLocalStaticSample {
	return m.track
}

// FailedPeers returns a channel will be sent errors containing information about
// PeerConnections that failed to have audio packets written to them.
func (m *microphone) FailedPeers() <-chan error {
	return m.failedPeers
}

// Initialized returns the channel to notify the caller when the microphone
// is fully initialized. Since microphone initialization is slow, this allows the caller to do
// it asynchronously and wait for its completion.
func (m *microphone) Initialized() <-chan struct{} {
	return m.initialized
}
