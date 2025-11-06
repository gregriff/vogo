package audio

import (
	"encoding/binary"
	"io"
	"log"

	"github.com/gen2brain/malgo"
	"github.com/pion/webrtc/v4"
	"gopkg.in/hraban/opus.v2"
)

func SetupPlayback(pc *webrtc.PeerConnection) (*malgo.AllocatedContext, *malgo.Device) {
	// configure playback device
	ctx, _ := malgo.InitContext(nil, malgo.ContextConfig{}, nil)

	deviceConfig := malgo.DefaultDeviceConfig(malgo.Playback)
	deviceConfig.Playback.Format = AudioFormat
	deviceConfig.Playback.Channels = NumChannels
	deviceConfig.SampleRate = SampleRate
	deviceConfig.PeriodSizeInMilliseconds = frameDurationMs

	// Buffer for decoded audio
	var pcm AudioBuffer

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
	device, deviceErr := malgo.InitDevice(ctx.Context, deviceConfig, malgo.DeviceCallbacks{
		Data: onSendFrames,
	})
	if deviceErr != nil {
		panic(deviceErr)
	}
	device.Start()

	decoder, decErr := opus.NewDecoder(SampleRate, NumChannels)
	if decErr != nil {
		panic(decErr)
	}

	pcmBuffer := make([]int16, int(frameSize)*4)

	// this is where the decoder writes pcm from the network
	pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		for { // Read RTP packets
			// track.SetReadDeadline(time.Now().Add(1*time.Second))
			packet, _, readErr := track.ReadRTP()
			if readErr != nil {
				if readErr == io.EOF {
					break // Track closed, exit loop
				}
				log.Println("packet read err: ", readErr)
				// TODO: check if context is cancelled, break if so
				continue // Temporary error, keep trying
			}

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
			// log.Print(" r", framesDecoded)
		}
	})
	return ctx, device
}

// int16ToBytes converts an int16 slice to a byte slice of PCM audio. TODO: can be reimpl with unsafe
func int16ToBytes(s []int16) []byte {
	result := make([]byte, len(s)*2)
	for i, v := range s {
		binary.LittleEndian.PutUint16(result[i*2:], uint16(v))
	}
	return result
}
