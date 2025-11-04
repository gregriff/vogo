package audio

import (
	"context"
	"encoding/binary"
	"log"
	"sync"
	"time"

	"github.com/gen2brain/malgo"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"gopkg.in/hraban/opus.v2"
)

type AudioBuffer struct {
	mu   sync.Mutex
	data []byte
}

func StartCapture(ctx context.Context, pc *webrtc.PeerConnection, track *webrtc.TrackLocalStaticSample) {
	// configure playback device
	deviceCtx, initErr := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if initErr != nil {
		panic(initErr)
	}
	defer deviceCtx.Uninit()
	defer deviceCtx.Free()

	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = AudioFormat
	deviceConfig.Capture.Channels = NumChannels
	deviceConfig.SampleRate = SampleRate
	// deviceConfig.PeriodSizeInMilliseconds = 10
	deviceConfig.Alsa.NoMMap = 1

	// for storing PCM from the mic, before converting to []int16
	var pcmBuffer AudioBuffer

	// read into capture buffer, to write to network. this fires every X milliseconds
	onRecvFrames := func(_, pInputSample []byte, framecount uint32) {
		pcmBuffer.mu.Lock()
		pcmBuffer.data = append(pcmBuffer.data, pInputSample...)
		pcmBuffer.mu.Unlock()
		log.Print("c")
	}

	// init playback device
	device, deviceErr := malgo.InitDevice(deviceCtx.Context, deviceConfig, malgo.DeviceCallbacks{
		Data: onRecvFrames,
	})
	if deviceErr != nil {
		panic(deviceErr)
	}
	defer device.Uninit()
	if startErr := device.Start(); startErr != nil {
		panic(startErr)
	}

	// sizeInBytes := uint32(malgo.SampleSizeInBytes(AudioFormat))
	encoder, encErr := opus.NewEncoder(SampleRate, NumChannels, opus.AppVoIP)
	if encErr != nil {
		panic(encErr)
	}

	ticker := time.NewTicker(frameDuration)
	defer ticker.Stop()

	var curSample media.Sample
	for range ticker.C {
		if ctx.Err() == context.Canceled {
			break // stop recording and teardown mic context and device
		}

		pcmBuffer.mu.Lock()

		// Need at least one frame worth of data
		if len(pcmBuffer.data) < bytesPerFrame {
			pcmBuffer.mu.Unlock()
			continue // wait for more data
		}

		// Extract one frame
		frameData := pcmBuffer.data[:bytesPerFrame]
		pcmBuffer.data = pcmBuffer.data[bytesPerFrame:] // remove from buffer TODO: this may leak
		pcmBuffer.mu.Unlock()

		curSample = media.Sample{
			Data: make([]byte, bytesPerFrame*2), // TODO: reuse and reslice
			// Timestamp: time.Now(),
			Duration: frameDuration,
		}

		// cast to int16 and encode to opus
		pcm16 := bytesToInt16(frameData)
		_, err := encoder.Encode(pcm16, curSample.Data)
		if err != nil {
			log.Println("Opus Encode error:", err)
			continue
		}

		// write to webrtc track
		wErr := track.WriteSample(curSample)
		if wErr != nil {
			log.Println("WriteSample error:", err)
			return
		}
		log.Print("w")
	}
}

// bytesToInt16 turns a byte slice of PCM audio into an int16 slice for the opus encoder to use.
// TODO: can replace this with an unsafe alternative that reinterprets the memory
func bytesToInt16(b []byte) []int16 {
	result := make([]int16, len(b)/2)
	for i := range result {
		result[i] = int16(binary.LittleEndian.Uint16(b[i*2:]))
	}
	return result
}
