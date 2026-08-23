package pcm

import (
	"testing"
)

// tests the NEON version of mix() with more than one stream,
// since one stream skips the actual mixing routine.

func BenchmarkMixARM64(b *testing.B) {
	const bufSize = BufferSize * 8

	b.Run("pade_NEON_n=2", func(b *testing.B) {
		samples := make([]int16, BufferSize)
		for i := range samples {
			samples[i] = randomPCMSample(pcmAmplitude)
		}

		s := NewStreams()
		_ = s.AddNew("s1")
		_ = s.AddNew("s2")

		for b.Loop() {
			b.StopTimer()
			s.WriteFrame("s1", samples)
			s.WriteFrame("s2", samples)
			b.StartTimer()
			s.mixNEON(BufferSize)
		}
	})

	b.Run("pade_NEON_n=4", func(b *testing.B) {
		samples := make([]int16, BufferSize)
		for i := range samples {
			samples[i] = randomPCMSample(pcmAmplitude)
		}

		s := NewStreams()
		_ = s.AddNew("s1")
		_ = s.AddNew("s2")
		_ = s.AddNew("s3")
		_ = s.AddNew("s4")

		for b.Loop() {
			b.StopTimer()
			s.WriteFrame("s1", samples)
			s.WriteFrame("s2", samples)
			s.WriteFrame("s3", samples)
			s.WriteFrame("s4", samples)
			b.StartTimer()
			s.mixNEON(BufferSize)
		}
	})
}
