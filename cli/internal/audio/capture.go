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

// AudioBuffer is a shared buffer that is written to/from the network and read/written by malgo for playback
type AudioBuffer struct {
	mu   sync.Mutex
	data []int16
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
	deviceConfig.PeriodSizeInMilliseconds = frameDurationMs

	// for storing int16 PCM from the mic
	var pcm AudioBuffer

	// read into capture buffer, to write to network. this fires every X milliseconds
	onRecvFrames := func(_, pInputSample []byte, framecount uint32) {
		pcm.mu.Lock()
		pcm.data = append(pcm.data, bytesToInt16(pInputSample)...)
		pcm.mu.Unlock()
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

	opusBuffer := make([]byte, opusBufferSize)
	encoder, encErr := opus.NewEncoder(SampleRate, NumChannels, opus.AppVoIP)
	if encErr != nil {
		panic(encErr)
	}
	// complexity, _ := encoder.Complexity()

	// TODO: shorten this?
	ticker := time.NewTicker(frameDuration)
	defer ticker.Stop()

	// loop to encode buffered PCM into opus and send to network
	for range ticker.C {
		if ctx.Err() == context.Canceled {
			break // stop recording and teardown mic context and device
		}

		pcm.mu.Lock()

		// Need at least one frame worth of data
		if len(pcm.data) < frameSize {
			pcm.mu.Unlock()
			continue // wait for more data
		}

		// Extract one frame and remove it from the buffer
		frameData := pcm.data[:frameSize]
		pcm.data = pcm.data[frameSize:] // TODO: this may leak
		pcm.mu.Unlock()

		// encode to opus
		bytesEncoded, err := encoder.Encode(frameData, opusBuffer)
		if err != nil {
			log.Println("OPUS ENCODE ERROR:", err)
			continue
		}

		// write to webrtc track
		wErr := track.WriteSample(media.Sample{
			Data:     opusBuffer[:bytesEncoded], // only the first N bytes are opus data.
			Duration: frameDuration,
		})
		if wErr != nil {
			log.Println("WriteSample error:", err)
			return
		}
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
