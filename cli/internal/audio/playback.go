package audio

import (
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
	deviceConfig.Alsa.NoMMap = 1

	// Buffer for decoded audio
	playbackBuffer := NewRingBuffer(rbCapacity)

	// sizeInBytes := uint32(malgo.SampleSizeInBytes(CaptureFormat))

	// read into output sample, for output to speaker device. this fires every X milliseconds
	onSendFrames := func(pOutputSample, _ []byte, framecount uint32) {
		// samplesToRead := framecount * deviceConfig.Playback.Channels * sizeInBytes
		n := playbackBuffer.Read(pOutputSample)
		if n > 0 {
			log.Print("p")
		}
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

	pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		// this is where the decoder writes pcm, and what we use to write to the playback buffer (ringbuffer)

		// TODO: pull out and reuse
		pcmBuffer := make([]int16, int(frameSize)*4)

		for { // Read RTP packets
			packet, _, readErr := track.ReadRTP()
			if readErr != nil {
				if readErr == io.EOF {
					break // Track closed, exit loop
				}
				log.Println("packet read err: ", readErr)
				continue // Temporary error, keep trying
			}

			bytesDecoded, decodeErr := decoder.Decode(packet.Payload, pcmBuffer)
			if decodeErr != nil {
				log.Println("DECODE ERROR: ", decodeErr.Error())
				// pcmBuffer = pcmBuffer[bytesDecoded:]
				continue
			}

			// Write decoded PCM to ring buffer
			// Malgo will pull from this buffer to play
			playbackBuffer.Write(pcmBuffer[:bytesDecoded])
			log.Print("r")
			// pcmBuffer = pcmBuffer[bytesDecoded:]

			// TODO: could use bytes decoded to know how much to read into the playback device
		}
	})
	return ctx, device
}
