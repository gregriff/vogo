package audio

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"time"

	"github.com/gen2brain/malgo"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"gopkg.in/hraban/opus.v2"
)

// microphone provides access to the device's
// microphone, to record opus audio data.
type microphone struct {
	ctx    *malgo.AllocatedContext
	device *malgo.Device

	// the webrtc track that we use write opus to
	track *webrtc.TrackLocalStaticSample

	// this is where malgo writes microphone pcm data
	pcm *callStream

	// initialized will be closed when the microphone device is initalized.
	initialized chan struct{}
}

func newMicrophone(track *webrtc.TrackLocalStaticSample) *microphone {
	return &microphone{
		ctx:         &malgo.AllocatedContext{},
		device:      &malgo.Device{},
		track:       track,
		initialized: make(chan struct{}),
	}
}

func (m *microphone) init() error {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	m.ctx = ctx
	if err != nil {
		return fmt.Errorf("error initializing device context: %w", err)
	}

	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = AudioFormat
	deviceConfig.Capture.Channels = NumChannels
	deviceConfig.SampleRate = SampleRate
	deviceConfig.PeriodSizeInMilliseconds = frameDurationMs

	m.pcm = &callStream{}

	// read into capture buffer, to write to network. this fires every X milliseconds
	onRecvFrames := func(_, pInputSample []byte, framecount uint32) {
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
		return fmt.Errorf("error creating capture device: %w", err)
	}
	// if err = device.Start(); err != nil {
	// 	return pcm, fmt.Errorf("error starting capture device: %w", err)
	// }
	close(m.initialized)
	return nil
}

func (m *microphone) start(ctx context.Context) error {
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
			failedPeers := m.track.WriteSample(media.Sample{
				Data:     opusBuffer[:bytesEncoded], // only the first N bytes are opus data.
				Duration: frameDuration,
			})
			if failedPeers != nil {
				log.Println("WriteSample error, contains failed peers:", err)
				continue
			}
		}
	}
}

func (m *microphone) uninit() error {
	if m.device != nil {
		m.device.Uninit()
	}
	if err := m.ctx.Uninit(); err != nil {
		return fmt.Errorf("error uninitializing capture device context: %w", err)
	}
	m.ctx.Free()
	log.Println("uninit and freed capture device")
	return nil
}

// bytesToInt16 turns a byte slice of PCM audio into an int16 slice for the opus encoder to use.
// TODO: can replace this with an unsafe alternative that reinterprets the memory.
func bytesToInt16(b []byte) []int16 {
	result := make([]int16, len(b)/2)
	for i := range result {
		result[i] = int16(binary.LittleEndian.Uint16(b[i*2:]))
	}
	return result
}
