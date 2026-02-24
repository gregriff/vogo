package audio

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"time"

	"github.com/gen2brain/malgo"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"gopkg.in/hraban/opus.v2"
)

func StartCapture(ctx context.Context, track *webrtc.TrackLocalStaticSample) error {
	deviceCtx, device, pcm, initErr := initCaptureDevice()
	log.Println("capture device created")
	defer uninitCapture(deviceCtx, device)
	if initErr != nil {
		return fmt.Errorf("error initalizing capture device: %w", initErr)
	}

	opusBuffer := make([]byte, opusBufferSize)
	encoder, encErr := opus.NewEncoder(SampleRate, NumChannels, opus.AppVoIP)
	if encErr != nil {
		return fmt.Errorf("encoder error: %w", encErr)
	}
	// complexity, _ := encoder.Complexity()
	// encoder.SetInBandFEC(true)  // adds latency, probably use PLC

	// TODO: shorten this?
	ticker := time.NewTicker(frameDuration)
	defer ticker.Stop()

	// loop to encode buffered PCM into opus and send to network
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			pcm.mu.Lock()

			// Need at least one frame worth of data
			if len(pcm.buf) < frameSize {
				pcm.mu.Unlock()
				continue // wait for more data
			}

			// Extract one frame and remove it from the buffer
			frameData := pcm.buf[:frameSize]
			pcm.buf = pcm.buf[frameSize:] // TODO: this may leak
			pcm.mu.Unlock()

			// encode to opus
			bytesEncoded, err := encoder.Encode(frameData, opusBuffer)
			if err != nil {
				log.Println("OPUS ENCODE ERROR:", err)
				continue
			}

			// write to webrtc track
			failedPeers := track.WriteSample(media.Sample{
				Data:     opusBuffer[:bytesEncoded], // only the first N bytes are opus data.
				Duration: frameDuration,
			})
			if failedPeers != nil {
				log.Println("WriteSample error, contains failed peers:", err)
				continue
			}
		}
	}
}

func initCaptureDevice() (ctx *malgo.AllocatedContext, device *malgo.Device, pcm *CallStream, err error) {
	// configure playback device
	ctx, err = malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		err = fmt.Errorf("error initializing device context: %w", err)
		return
	}

	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = AudioFormat
	deviceConfig.Capture.Channels = NumChannels
	deviceConfig.SampleRate = SampleRate
	deviceConfig.PeriodSizeInMilliseconds = frameDurationMs

	pcm = &CallStream{}

	// read into capture buffer, to write to network. this fires every X milliseconds
	onRecvFrames := func(_, pInputSample []byte, framecount uint32) {
		pcm.mu.Lock()
		pcm.buf = append(pcm.buf, bytesToInt16(pInputSample)...)
		pcm.mu.Unlock()
	}

	// init playback device
	device, err = malgo.InitDevice(ctx.Context, deviceConfig, malgo.DeviceCallbacks{
		Data: onRecvFrames,
	})
	if err != nil {
		err = fmt.Errorf("error creating capture device: %w", err)
		return
	}
	if err := device.Start(); err != nil {
		err = fmt.Errorf("error starting capture device: %w", err)
	}
	return
}

func uninitCapture(ctx *malgo.AllocatedContext, device *malgo.Device) {
	if device != nil {
		device.Uninit()
	}
	if err := ctx.Uninit(); err != nil {
		fmt.Printf("error uninitializing capture device context: %v", err)
	}
	ctx.Free()
	fmt.Println("uninit and freed capture device")
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
