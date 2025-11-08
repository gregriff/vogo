package audio

import (
	"time"

	"github.com/gen2brain/malgo"
)

const (
	NumChannels  = 2
	SampleRate   = 48_000
	samplesPerMs = SampleRate / 1000

	// denotes how many bytes per element of pcm
	AudioFormat = malgo.FormatS16

	// the frameDuration is used for webrtc metadata and for packetizing the correct amount of pcm into opus
	frameDuration   = 20 * time.Millisecond
	frameDurationMs = 20

	// frameSize is the number of samples per frame
	frameSize = NumChannels * frameDurationMs * samplesPerMs

	// size of buffer to hold encoded opus to be written to packets
	opusBufferSize = frameSize / 2

	// size of buffer to hold decoded PCM from the network
	pcmBufferSize = frameSize

	// time until a packet read is aborted, and the parent context is checked to see whether
	// to shutdown and teardown playback device
	ReadPacketDeadline = 250 * time.Millisecond
)
