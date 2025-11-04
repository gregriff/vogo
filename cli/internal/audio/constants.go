package audio

import (
	"time"

	"github.com/gen2brain/malgo"
)

const (
	NumChannels = 1
	SampleRate  = 48_000

	// denotes how many bytes per element
	AudioFormat = malgo.FormatS16

	rbCapacity = 50_000
	// pcmBufCap   = 23040

	frameDuration = 20 * time.Millisecond
	frameSizeMs   = 20 // if you don't know, go with 60 ms.

	// determines size of pcmBuffer
	frameSize = NumChannels * frameSizeMs * (SampleRate / 1000)

	samplesPerFrame = int(SampleRate * 0.02)
	bytesPerFrame   = samplesPerFrame * int(AudioFormat)
)
