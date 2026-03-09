package audio

import (
	"context"
	"fmt"
	"log"
	"time"
	"unsafe"

	"github.com/gen2brain/malgo"
	"github.com/gregriff/vogo/shared"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"gopkg.in/hraban/opus.v2"
)

// microphone provides access to the device's
// microphone, to record opus audio data.
type microphone struct {
	ctx    *malgo.AllocatedContext
	device *malgo.Device

	// this is where malgo writes microphone pcm data
	pcm stream

	// the webrtc track that we use write opus to
	track *webrtc.TrackLocalStaticSample

	// initialized will be closed when the microphone device is initialized.
	initialized chan struct{}

	// mic will send failed PC Ids on this chan
	failedPeers chan<- error
}

func newMicrophone(track *webrtc.TrackLocalStaticSample) microphone {
	return microphone{
		ctx:         &malgo.AllocatedContext{},
		device:      &malgo.Device{},
		track:       track,
		pcm:         newStream(),
		initialized: make(chan struct{}),
		failedPeers: make(chan error, shared.ChannelCapacity-1),
	}
}

func (m *microphone) Init() error {
	start := time.Now()
	defer func() {
		log.Printf("mic initialized in %v", time.Since(start))
	}()

	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	m.ctx = ctx
	if err != nil {
		return fmt.Errorf("error initializing mic context: %w", err)
	}

	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = AudioFormat
	deviceConfig.Capture.Channels = NumChannels
	deviceConfig.SampleRate = SampleRate
	deviceConfig.PeriodSizeInMilliseconds = frameDurationMs

	// read into capture buffer, to write to network. this fires every X milliseconds
	onRecvFrames := func(_, pInputSample []byte, _ uint32) { // uint32 is framecount
		m.pcm.mu.Lock()
		m.pcm.buf = append(m.pcm.buf, bytesToInt16(pInputSample)...)
		m.pcm.mu.Unlock()
	}

	// init playback device
	device, err := malgo.InitDevice(ctx.Context, deviceConfig, malgo.DeviceCallbacks{
		Data: onRecvFrames,
	})
	m.device = device
	if err != nil {
		return fmt.Errorf("error creating mic device: %w", err)
	}
	close(m.initialized)
	return nil
}

func (m *microphone) Start(ctx context.Context) error {
	if err := m.device.Start(); err != nil {
		return err
	}

	opusBuffer := make([]byte, opusBufferSize)
	encoder, err := opus.NewEncoder(SampleRate, NumChannels, opus.AppVoIP)
	if err != nil {
		return fmt.Errorf("encoder error: %w", err)
	}
	// complexity, _ := encoder.Complexity()
	// encoder.SetInBandFEC(true)  // adds latency, probably use PLC

	// TODO: shorten this?
	ticker := time.NewTicker(frameDuration)
	defer ticker.Stop()

	// loop to encode buffered PCM into opus and send to network
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			m.pcm.mu.Lock()

			// Need at least one frame worth of data
			if len(m.pcm.buf) < frameSize {
				m.pcm.mu.Unlock()
				continue // wait for more data
			}

			// Extract one frame and remove it from the buffer
			frameData := m.pcm.buf[:frameSize]
			m.pcm.buf = m.pcm.buf[frameSize:] // TODO: this may leak
			m.pcm.mu.Unlock()

			// encode to opus
			bytesEncoded, err := encoder.Encode(frameData, opusBuffer)
			if err != nil {
				log.Println("OPUS ENCODE ERROR:", err)
				continue
			}

			// write to webrtc track
			failed := m.track.WriteSample(media.Sample{
				Data:     opusBuffer[:bytesEncoded], // only the first N bytes are opus data.
				Duration: frameDuration,
			})

			// TODO: send these on a chan?
			if failed != nil {
				log.Println("WriteSample error, contains failed peers:", err)
				m.failedPeers <- failed
				continue
			}
		}
	}
}

func (m *microphone) Uninit() error {
	if m.device != nil {
		m.device.Uninit()
	}
	if err := m.ctx.Uninit(); err != nil {
		return fmt.Errorf("error uninitializing mic context: %w", err)
	}
	m.ctx.Free()
	log.Println("uninit and freed mic")
	return nil
}

// Track returns the webrtc Track where microphone audio is written.
func (m *microphone) Track() *webrtc.TrackLocalStaticSample {
	return m.track
}

// FailedPeers returns a channel will be sent errors containing information about
// PeerConnections that failed to have audio packets written to them.
func (m *microphone) FailedPeers() chan<- error {
	return m.failedPeers
}

// bytesToInt16 reinterprets a byte slice of PCM audio into an int16 slice for the opus encoder to use.
func bytesToInt16(b []byte) []int16 {
	if len(b) == 0 {
		return nil
	}
	return unsafe.Slice((*int16)(unsafe.Pointer(&b[0])), len(b)/2)
}
