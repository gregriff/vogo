package audio

import (
	"fmt"
	"log"
	"sync"
	"unsafe"

	"github.com/gen2brain/malgo"
)

// speaker provides access to the client's speaker in order
// to play opus audio data.
type speaker struct {
	ctx    *malgo.AllocatedContext
	device *malgo.Device

	// this WaitGroup needs to be incremented for each PeerConnection that
	// writes data to the speaker. It is used to wait for all PC's to stop
	// writing to the speaker so uninit is clean.
	wg *sync.WaitGroup

	// initialized will be closed when the speaker device is initialized.
	initialized chan struct{}
}

func newSpeaker() speaker {
	return speaker{
		ctx:         &malgo.AllocatedContext{},
		device:      &malgo.Device{},
		wg:          &sync.WaitGroup{},
		initialized: make(chan struct{}),
	}
}

// Init initializes and starts a speaker device for playback.
// It requires a callback that will run every [frameDurationMs], that is
// responsible for writing audio PCM to the speaker's buffer.
func (s *speaker) Init(onSendFrames malgo.DataProc) error {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	s.ctx = ctx
	if err != nil {
		return fmt.Errorf("error initializing speaker context: %w", err)
	}

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
func (s *speaker) Uninit() error {
	if s.ctx == nil {
		log.Panic("speaker ctx uninit before init")
	}
	if s.device != nil {
		s.device.Uninit()
	}
	s.wg.Wait() // TODO: check if this is even nessicary.
	if err := s.ctx.Uninit(); err != nil {
		return fmt.Errorf("error uninitializing speaker context: %w", err)
	}
	s.ctx.Free()
	log.Println("uninit and freed speaker")
	return nil
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
