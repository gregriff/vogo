package pcm

import (
	"testing"
)

// tests the AVX versions of mix() with more than one stream, since one stream
// skips the actual mixing routine.

func BenchmarkMixAMD64(b *testing.B) {
	b.Run("pade_AVX2_n=2", func(b *testing.B) {
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
			// s.mixAVX2(BufferSize)
			s.mixAVX(BufferSize, false)
		}
	})

	b.Run("pade_AVX2_n=4", func(b *testing.B) {
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
			// s.mixAVX2(BufferSize)
			s.mixAVX(BufferSize, false)
		}
	})

	b.Run("pade_AVX512_n=2", func(b *testing.B) {
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
			// s.mixAVX2(BufferSize)
			s.mixAVX(BufferSize, true)
		}
	})

	b.Run("pade_AVX512_n=4", func(b *testing.B) {
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
			// s.mixAVX2(BufferSize)
			s.mixAVX(BufferSize, true)
		}
	})
}
