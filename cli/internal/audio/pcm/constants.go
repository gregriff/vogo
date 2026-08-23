package pcm

import (
	"time"

	"github.com/gen2brain/malgo"
)

const (
	NumChannels  = 1
	SampleRate   = 48_000 // kHz
	samplesPerMs = SampleRate / 1000

	// denotes how many bytes per element of pcm.
	AudioFormat = malgo.FormatS16

	// frameDurationMs sets the period size for the mic and speaker callbacks.
	frameDurationMs = 10

	// the FrameDuration is used for webrtc metadata and for packetizing the correct amount of pcm into opus.
	FrameDuration = frameDurationMs * time.Millisecond

	// max Ms of audio to extrapolate using PLC.
	maxPLCDurationMs = 60

	// max frames to extrapolate using PLC.
	MaxPLCFrames = maxPLCDurationMs / frameDurationMs

	// sets the period size in ms for miniaudio mic and speaker.
	CapturePeriodMs  = frameDurationMs
	PlaybackPeriodMs = frameDurationMs // Note: multiply this by 2 if playback glitches happen

	// FrameSize is the number of samples per frame.
	FrameSize = NumChannels * frameDurationMs * samplesPerMs

	// size of buffer to hold decoded PCM from the network.
	BufferSize = FrameSize * (PlaybackPeriodMs / CapturePeriodMs)
)
