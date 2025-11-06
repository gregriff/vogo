package audio

import (
	"encoding/binary"
	"io"
	"log"

	"github.com/gen2brain/malgo"
	"github.com/pion/webrtc/v4"
	"gopkg.in/hraban/opus.v2"
)

type PlaybackBuffer struct {
	buf,
	decodeBuf []int16
	samplesDecoded int
}

// WritePCM moves PCM data from pb.decodeBuf into the main pb.buf and keeps track of
// the number of samples written. It should be called after decoded PCM has been placed into
// pb.decodeBuf by libopus (this overwrites old data in pb.decodeBuf).
func (pb *PlaybackBuffer) WritePCM(numSamples int) {
	pb.buf = append(pb.buf, pb.decodeBuf[:numSamples]...)
	pb.samplesDecoded += numSamples
}

// Flush writes all samples buffered in the pb to the pcm buffer and resets the state.
func (pb *PlaybackBuffer) Flush(pcm *AudioBuffer) (decoded int) {
	pcm.mu.Lock()
	decoded = pb.samplesDecoded
	pcm.data = append(pcm.data, pb.buf[:pb.samplesDecoded]...)
	pcm.mu.Unlock()

	pb.buf = pb.buf[pb.samplesDecoded:]
	pb.samplesDecoded = 0
	return
}

func SetupPlayback(pc *webrtc.PeerConnection) (*malgo.AllocatedContext, *malgo.Device) {
	// configure playback device
	ctx, _ := malgo.InitContext(nil, malgo.ContextConfig{}, nil)

	deviceConfig := malgo.DefaultDeviceConfig(malgo.Playback)
	deviceConfig.Playback.Format = AudioFormat
	deviceConfig.Playback.Channels = NumChannels
	deviceConfig.SampleRate = SampleRate
	// deviceConfig.Alsa.NoMMap = 1
	deviceConfig.PeriodSizeInMilliseconds = frameSizeMs

	// Buffer for decoded audio
	var pcm AudioBuffer

	// read into output sample buf, for output to speaker device. this fires every X milliseconds
	onSendFrames := func(pOutputSample, _ []byte, framecount uint32) {
		samplesToRead := framecount * deviceConfig.Playback.Channels
		pcm.mu.Lock()
		defer pcm.mu.Unlock()

		// if there isn't yet a full sample in the pcmBuffer sent from the network
		if len(pcm.data) < int(samplesToRead) {
			log.Printf("empty, need=%ds, psiM=%d, framecount=%d",
				samplesToRead, deviceConfig.PeriodSizeInMilliseconds, framecount)
			return
		}

		// write a full sample to the speaker buffer
		copy(pOutputSample, int16ToBytes(pcm.data[:samplesToRead]))
		pcm.data = pcm.data[samplesToRead:] // TODO: probably leaks
		log.Printf(" p=%d, samplesRemaining=%d", samplesToRead, len(pcm.data)/int(deviceConfig.Playback.Channels))
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

	pb := PlaybackBuffer{
		buf:       make([]int16, int(frameSize)*3),
		decodeBuf: make([]int16, frameSize),
	}

	// this is where the decoder writes pcm, and what we use to write to the playback buffer
	pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		// device.Start()
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

			numSamples, decodeErr := decoder.Decode(packet.Payload, pb.decodeBuf)
			if decodeErr != nil {
				log.Println("DECODE ERROR: ", decodeErr.Error())
				continue
			}
			pb.WritePCM(numSamples)

			// if pb.samplesDecoded <= 1000 {
			// continue
			// }
			n := pb.Flush(&pcm)
			log.Print(" r", n)

			// TODO: could use bytes decoded to know how much to read into the playback device
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
