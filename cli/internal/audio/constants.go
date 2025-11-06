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

	frameDuration = 20 * time.Millisecond
	frameSizeMs   = 20
	frameSize     = NumChannels * frameSizeMs * (SampleRate / 1000)
)
