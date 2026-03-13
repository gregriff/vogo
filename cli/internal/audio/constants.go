// package audio contains abstractions over microphone and speaker hardware, PCM buffer manipulation routines,
// and higher-level, voice-call and voice-channel structs for controlling audio in those configurations.
package audio

import (
	"time"

	"github.com/gen2brain/malgo"
)

const (
	NumChannels  = 1
	SampleRate   = 48_000
	samplesPerMs = SampleRate / 1000

	// denotes how many bytes per element of pcm.
	AudioFormat = malgo.FormatS16

	// the frameDuration is used for webrtc metadata and for packetizing the correct amount of pcm into opus.
	frameDuration   = 20 * time.Millisecond
	frameDurationMs = 20

	// frameSize is the number of samples per frame.
	frameSize = NumChannels * frameDurationMs * samplesPerMs

	// size of buffer to hold encoded opus to be written to packets.
	opusBufferSize = frameSize / 2

	opusBitrate = 64_000

	// size of buffer to hold decoded PCM from the network.
	pcmBufferSize = frameSize
)
