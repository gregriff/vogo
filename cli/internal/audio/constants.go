package audio

import (
	"time"

	"github.com/gen2brain/malgo"
)

const (
	NumChannels  = 2
	SampleRate   = 48_000
	samplesPerMs = SampleRate / 1000

	// denotes how many bytes per element
	AudioFormat = malgo.FormatS16

	frameDuration   = 20 * time.Millisecond
	frameDurationMs = 20
	frameSize       = NumChannels * frameDurationMs * samplesPerMs
)
