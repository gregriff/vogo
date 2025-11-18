package audio

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"sync"

	"github.com/gen2brain/malgo"
	"github.com/pion/webrtc/v4"
	"gopkg.in/hraban/opus.v2"
)

// SetupPlayback initializes the playback device with malgo, and defines the callback that is run per remote-track, that
// reads the audio from the network and places it in the buffer for the playback device to read from
func SetupPlayback(pc *webrtc.PeerConnection, wg *sync.WaitGroup) (
	deviceCtx *malgo.AllocatedContext,
	device *malgo.Device,
	err error,
) {
	deviceCtx, device, pcm, err := initPlaybackDevice()
	if err != nil {
		err = fmt.Errorf("error initalizing playback device: %w", err)
		return
	}

	pcmBuffer := make([]int16, pcmBufferSize)
	decoder, err := opus.NewDecoder(SampleRate, NumChannels)
	if err != nil {
		err = fmt.Errorf("decoder init error: %w", err)
		return
	}

	// this func runs for every remote track connected to this peer connection
	// this is where the decoder writes pcm from the network
	// note: realize that this code will run multiple times if more than one remote track is connected (multi-user voice chat)
	// note: this callback should not panic
	// TODO: mix audio here, maybe pull out
	pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		wg.Add(1)
		defer wg.Done()
		for {
			// this blocks until either a packet is fully read or the pc is shutdown (returns an io.EOF err)
			packet, _, readErr := track.ReadRTP()
			if readErr != nil {
				if readErr == io.EOF {
					return // Track closed, exit loop
				}
				log.Println("PACKET READ ERR: ", readErr)
				continue // Temporary error, keep trying
			}

			// TODO: check for 0 samples decoded and call PLC?
			samplesDecoded, decodeErr := decoder.Decode(packet.Payload, pcmBuffer)
			if decodeErr != nil {
				log.Println("DECODE ERROR: ", decodeErr.Error())
				continue
			}

			framesDecoded := samplesDecoded * NumChannels
			// Write decoded PCM to playback buffer, which malgo will pull from for playback
			pcm.mu.Lock()
			pcm.data = append(pcm.data, pcmBuffer[:framesDecoded]...)
			pcm.mu.Unlock()
		}
	})
	return
}

func initPlaybackDevice() (ctx *malgo.AllocatedContext, device *malgo.Device, pcm *AudioBuffer, err error) {
	// configure playback device
	ctx, err = malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		err = fmt.Errorf("error initializing device context: %w", err)
		return
	}

	deviceConfig := malgo.DefaultDeviceConfig(malgo.Playback)
	deviceConfig.Playback.Format = AudioFormat
	deviceConfig.Playback.Channels = NumChannels
	deviceConfig.SampleRate = SampleRate
	deviceConfig.PeriodSizeInMilliseconds = frameDurationMs

	pcm = &AudioBuffer{}

	// read into output sample buf, for output to speaker device. this fires every X milliseconds
	onSendFrames := func(pOutputSample, _ []byte, framecount uint32) {
		samplesToRead := framecount * NumChannels
		pcm.mu.Lock()
		defer pcm.mu.Unlock()

		// if there isn't yet a full sample in the pcmBuffer sent from the network
		if len(pcm.data) < int(samplesToRead) {
			return
		}

		// write a full sample to the speaker buffer
		copy(pOutputSample, int16ToBytes(pcm.data[:samplesToRead]))
		pcm.data = pcm.data[samplesToRead:] // TODO: probably leaks
	}

	// init playback device
	device, err = malgo.InitDevice(ctx.Context, deviceConfig, malgo.DeviceCallbacks{
		Data: onSendFrames,
	})
	if err != nil {
		err = fmt.Errorf("error creating playback device: %w", err)
		return
	}
	if err := device.Start(); err != nil {
		err = fmt.Errorf("error starting playback device: %w", err)
	}
	return
}

// int16ToBytes converts an int16 slice to a byte slice of PCM audio. TODO: can be reimpl with unsafe
func int16ToBytes(s []int16) []byte {
	result := make([]byte, len(s)*2)
	for i, v := range s {
		binary.LittleEndian.PutUint16(result[i*2:], uint16(v))
	}
	return result
}
