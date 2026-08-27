//go:build cgo

package audio

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gen2brain/malgo"
	"github.com/gregriff/vogo/cli/internal/audio/pcm"
)

// speaker provides access to the client's speaker in order
// to play opus audio data.
type speaker struct {
	ctx    *malgo.AllocatedContext
	device *malgo.Device

	// initialized will be closed when the speaker device is initialized.
	initialized chan struct{}

	// the malgo context will be sent over this chan
	ctxChan chan *malgo.AllocatedContext
}

func newSpeaker() speaker {
	return speaker{
		ctx:         &malgo.AllocatedContext{},
		device:      &malgo.Device{},
		initialized: make(chan struct{}),
		ctxChan:     make(chan *malgo.AllocatedContext),
	}
}

// Init initializes and starts a speaker device for playback.
// It requires a callback that will run every [frameDurationMs], that is
// responsible for writing audio PCM to the speaker's buffer.
func (s *speaker) Init(ctx context.Context, onSendFrames malgo.DataProc) error {
	select {
	case <-ctx.Done():
		return nil
	case s.ctx = <-s.ctxChan:
		break
	}

	start := time.Now()
	defer func() {
		log.Printf("speaker initialized in %v", time.Since(start))
	}()

	cfg := malgo.DefaultDeviceConfig(malgo.Playback)
	cfg.Playback.Format = pcm.AudioFormat
	cfg.Playback.Channels = pcm.NumChannels
	cfg.SampleRate = pcm.SampleRate
	cfg.NoClip = 1
	cfg.NoPreSilencedOutputBuffer = 1
	cfg.PeriodSizeInMilliseconds = pcm.PlaybackPeriodMs

	device, err := malgo.InitDevice(s.ctx.Context, cfg, malgo.DeviceCallbacks{
		Data: onSendFrames, // note: could also include a stop callback
	})
	s.device = device
	if err != nil {
		return fmt.Errorf("error creating speaker: %w", err)
	}
	close(s.initialized)
	return nil
}

func (s *speaker) Start() error {
	return s.device.Start()
}

// Uninit uninitializes the malgo speaker device and frees all its resources. Ideally,
// nothing should be writing to the speaker device when this is called. This is ensured by
// closing all PeerConnections beforehand, since their RemoteTrack handlers write to the device.
func (s *speaker) Uninit() {
	if s.device != nil {
		s.device.Uninit()
	}
	log.Println("uninit and freed speaker")
}

// Initialized returns the channel to notify the caller when the speaker
// is fully initialized. Since speaker initialization is slow, this allows the caller to do
// it asynchronously and wait for its completion.
func (s *speaker) Initialized() <-chan struct{} {
	return s.initialized
}
