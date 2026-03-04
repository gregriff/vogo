package audio

import (
	"encoding/binary"
	"fmt"
	"log"
	"sync"

	"github.com/gen2brain/malgo"
)

// playback provides access to the client's speaker, using malgo,
// in order to play opus audio data.
type playback struct {
	ctx    *malgo.AllocatedContext
	wg     *sync.WaitGroup
	device *malgo.Device

	// initialized will be closed when the speaker device is initalized.
	initialized chan struct{}
}

func newPlayback() *playback {
	return &playback{
		ctx:         &malgo.AllocatedContext{},
		wg:          &sync.WaitGroup{},
		device:      &malgo.Device{},
		initialized: make(chan struct{}),
	}
}

// initSpeaker initializes and starts a speaker device for playback.
func (p *playback) init(onSendFrames malgo.DataProc) error {
	config := malgo.DefaultDeviceConfig(malgo.Playback)
	config.Playback.Format = AudioFormat
	config.Playback.Channels = NumChannels
	config.SampleRate = SampleRate
	config.PeriodSizeInMilliseconds = frameDurationMs

	// init playback device
	device, err := malgo.InitDevice(p.ctx.Context, config, malgo.DeviceCallbacks{
		Data: onSendFrames,
	})
	p.device = device
	if err != nil {
		return fmt.Errorf("error creating playback device: %w", err)
	}
	if err = device.Start(); err != nil {
		return fmt.Errorf("error starting playback device: %w", err)
	}
	return nil
}

// uninitPlayback uninitializes the malgo playback device and frees all its resources. Ideally,
// nothing should be writing to the speaker device when this is called. This is ensured by
// closing all PeerConnections beforehand, since their RemoteTrack handlers write to the device.
func (p *playback) uninit() {
	if p.ctx == nil {
		log.Println("playback ctx uninit before init")
		return
	}
	if p.device != nil {
		p.device.Uninit()
	}
	if err := p.ctx.Uninit(); err != nil {
		log.Printf("error uninitializing playback device context: %v", err)
	}
	p.ctx.Free()
	log.Println("uninit and freed playback device")
}

// int16ToBytes converts an int16 slice to a byte slice of PCM audio. TODO: can be reimpl with unsafe.
func int16ToBytes(s []int16) []byte {
	result := make([]byte, len(s)*2)
	for i, v := range s {
		binary.LittleEndian.PutUint16(result[i*2:], uint16(v))
	}
	return result
}
