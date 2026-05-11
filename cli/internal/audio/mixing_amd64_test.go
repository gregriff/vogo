package audio

import (
	"testing"

	"github.com/gregriff/vogo/cli/internal/audio/ringbuffer"
)

// tests the AVX versions of mix() with more than one stream, since one stream
// skips the actual mixing routine.

func BenchmarkMixAMD64(b *testing.B) {
	const bufSize = pcmBufferSize * 8

	b.Run("AVX2_n=2", func(b *testing.B) {
		samples := make([]int16, pcmBufferSize)
		for i := range samples {
			samples[i] = randomPCMSample(pcmAmplitude)
		}

		s := newStreams()
		s1 := ringbuffer.New(bufSize)
		s2 := ringbuffer.New(bufSize)
		s.add("s1", &s1)
		s.add("s2", &s2)

		for b.Loop() {
			b.StopTimer()
			s1.Write(samples)
			s2.Write(samples)
			b.StartTimer()
			s.mixAVX2(pcmBufferSize)
		}
	})

	b.Run("AVX2_n=4", func(b *testing.B) {
		samples := make([]int16, pcmBufferSize)
		for i := range samples {
			samples[i] = randomPCMSample(pcmAmplitude)
		}

		s := newStreams()
		s1 := ringbuffer.New(bufSize)
		s2 := ringbuffer.New(bufSize)
		s3 := ringbuffer.New(bufSize)
		s4 := ringbuffer.New(bufSize)
		s.add("s1", &s1)
		s.add("s2", &s2)
		s.add("s3", &s3)
		s.add("s4", &s4)

		for b.Loop() {
			b.StopTimer()
			s1.Write(samples)
			s2.Write(samples)
			s3.Write(samples)
			s4.Write(samples)
			b.StartTimer()
			s.mixAVX2(pcmBufferSize)
		}
	})

	b.Run("AVX512_n=2", func(b *testing.B) {
		samples := make([]int16, pcmBufferSize)
		for i := range samples {
			samples[i] = randomPCMSample(pcmAmplitude)
		}

		s := newStreams()
		s1 := ringbuffer.New(bufSize)
		s2 := ringbuffer.New(bufSize)
		s.add("s1", &s1)
		s.add("s2", &s2)

		for b.Loop() {
			b.StopTimer()
			s1.Write(samples)
			s2.Write(samples)
			b.StartTimer()
			s.mixAVX512(pcmBufferSize)
		}
	})

	b.Run("AVX512_n=4", func(b *testing.B) {
		samples := make([]int16, pcmBufferSize)
		for i := range samples {
			samples[i] = randomPCMSample(pcmAmplitude)
		}

		s := newStreams()
		s1 := ringbuffer.New(bufSize)
		s2 := ringbuffer.New(bufSize)
		s3 := ringbuffer.New(bufSize)
		s4 := ringbuffer.New(bufSize)
		s.add("s1", &s1)
		s.add("s2", &s2)
		s.add("s3", &s3)
		s.add("s4", &s4)

		for b.Loop() {
			b.StopTimer()
			s1.Write(samples)
			s2.Write(samples)
			s3.Write(samples)
			s4.Write(samples)
			b.StartTimer()
			s.mixAVX512(pcmBufferSize)
		}
	})
}
