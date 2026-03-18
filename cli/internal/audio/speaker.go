package audio

import (
	"context"
	"fmt"
	"log"
	"time"
	"unsafe"

	"github.com/gen2brain/malgo"
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

	config := malgo.DefaultDeviceConfig(malgo.Playback)
	config.Playback.Format = AudioFormat
	config.Playback.Channels = NumChannels
	config.SampleRate = SampleRate
	config.NoClip = 1
	config.NoPreSilencedOutputBuffer = 1
	config.PeriodSizeInMilliseconds = playbackPeriodMs

	device, err := malgo.InitDevice(s.ctx.Context, config, malgo.DeviceCallbacks{
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

// int16ToBytes reinterprets an int16 slice to a byte slice of PCM audio.
func int16ToBytes(s []int16) []byte {
	if len(s) == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(&s[0])), len(s)*2)
}
