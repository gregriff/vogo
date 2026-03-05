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
	wg     *sync.WaitGroup
	device *malgo.Device

	// initialized will be closed when the speaker device is initalized.
	initialized chan struct{}
}

func newSpeaker() *speaker {
	return &speaker{
		ctx:         &malgo.AllocatedContext{},
		wg:          &sync.WaitGroup{},
		device:      &malgo.Device{},
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

	// init playback device
	device, err := malgo.InitDevice(s.ctx.Context, config, malgo.DeviceCallbacks{
		Data: onSendFrames,
	})
	s.device = device
	if err != nil {
		return fmt.Errorf("error creating playback device: %w", err)
	}
	// if err = device.Start(); err != nil {
	// 	return fmt.Errorf("error starting playback device: %w", err)
	// }
	return nil
}

func (s *speaker) start() error {
	if err := s.device.Start(); err != nil {
		return fmt.Errorf("error starting speaker: %w", err)
	}
	return nil
}

// uninitPlayback uninitializes the malgo playback device and frees all its resources. Ideally,
// nothing should be writing to the speaker device when this is called. This is ensured by
// closing all PeerConnections beforehand, since their RemoteTrack handlers write to the device.
func (s *speaker) uninit() error {
	if s.ctx == nil {
		log.Panic("playback ctx uninit before init")
	}
	if s.device != nil {
		s.device.Uninit()
	}
	if err := s.ctx.Uninit(); err != nil {
		return fmt.Errorf("error uninitializing playback device context: %w", err)
	}
	s.ctx.Free()
	log.Println("uninit and freed playback device")
	return nil
}

// int16ToBytes converts an int16 slice to a byte slice of PCM audio. TODO: can be reimpl with unsafe.
func int16ToBytes(s []int16) []byte {
	result := make([]byte, len(s)*2)
	for i, v := range s {
		binary.LittleEndian.PutUint16(result[i*2:], uint16(v))
	}
	return result
}
