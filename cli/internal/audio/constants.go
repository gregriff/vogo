// package audio contains abstractions over microphone and speaker hardware, PCM buffer manipulation routines,
// and higher-level, voice-call and voice-channel structs for controlling audio in those configurations.
package audio

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

	// the frameDuration is used for webrtc metadata and for packetizing the correct amount of pcm into opus.
	frameDuration = frameDurationMs * time.Millisecond

	// max Ms of audio to extrapolate using PLC.
	maxPLCDurationMs = 60

	// max frames to extrapolate using PLC.
	maxPLCFrames = maxPLCDurationMs / frameDurationMs

	// sets the period size in ms for miniaudio mic and speaker.
	capturePeriodMs  = frameDurationMs
	playbackPeriodMs = frameDurationMs // Note: multiply this by 2 if playback glitches happen

	// frameSize is the number of samples per frame.
	frameSize = NumChannels * frameDurationMs * samplesPerMs

	// size of buffer to hold opus encoded from mic, to be written to packets.
	opusBufferSize = frameSize

	// opusBitrate sets the bitrate of the opus encoder.
	opusBitrate = 64_000

	// size of buffer to hold decoded PCM from the network.
	pcmBufferSize = frameSize * (playbackPeriodMs / capturePeriodMs)
)

// for mixing
const (
	// [3/2] approximant diverges from tanh by 5% at x=3.
	// clamp inputs to +/- 3. Note: it exceeds 1.0 at x=2.32.
	padeMaxInput = 3.
	padeMinInput = -3.
)
