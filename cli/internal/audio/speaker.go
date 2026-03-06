package audio

import (
	"encoding/binary"
	"fmt"
	"log"
	"sync"

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

	// initialized will be closed when the speaker device is initalized.
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

// initSpeaker initializes and starts a speaker device for playback.
func (s *speaker) init(onSendFrames malgo.DataProc) error {
	config := malgo.DefaultDeviceConfig(malgo.Playback)
	config.Playback.Format = AudioFormat
	config.Playback.Channels = NumChannels
	config.SampleRate = SampleRate
	config.PeriodSizeInMilliseconds = frameDurationMs

	device, err := malgo.InitDevice(s.ctx.Context, config, malgo.DeviceCallbacks{
		Data: onSendFrames,
	})
	s.device = device
	if err != nil {
		return fmt.Errorf("error creating device: %w", err)
	}
	return nil
}

func (s *speaker) Start() error {
	return s.device.Start()
}

// uninit uninitializes the malgo speaker device and frees all its resources. Ideally,
// nothing should be writing to the speaker device when this is called. This is ensured by
// closing all PeerConnections beforehand, since their RemoteTrack handlers write to the device.
func (s *speaker) uninit() error {
	if s.ctx == nil {
		log.Panic("speaker ctx uninit before init")
	}
	if s.device != nil {
		s.device.Uninit()
	}
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

// int16ToBytes converts an int16 slice to a byte slice of PCM audio. TODO: can be reimpl with unsafe.
func int16ToBytes(s []int16) []byte {
	result := make([]byte, len(s)*2)
	for i, v := range s {
		binary.LittleEndian.PutUint16(result[i*2:], uint16(v))
	}
	return result
}
