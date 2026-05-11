package audio

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/gregriff/vogo/cli/internal/audio/ringbuffer"
)

func TestMix(t *testing.T) {
	t.Run("single full stream", func(t *testing.T) {
		numSamples := 5
		s := newStreams()
		a := ringbuffer.New(numSamples)
		a.Write([]int16{10, 20, 30, 40, 50})
		s.add("a", &a)
		s.mix(numSamples)

		count := 0
		for _, v := range s.mixed {
			if v == 0 {
				break
			}
			count++
		}
		if count != numSamples {
			t.Errorf("num mixed samples in mixed buf (%d) != numSamples arg (%d)", count, numSamples)
		}
	})

	t.Run("silence streams mix to silence", func(t *testing.T) {
		s := newStreams()
		pcm := []int16{0, 0, 0}
		a := ringbuffer.New(3)
		b := ringbuffer.New(3)
		a.Write(pcm)
		b.Write(pcm)
		s.add("a", &a)
		s.add("b", &b)
		s.mix(3)
		if s.mixed[0] != 0 {
			t.Errorf("expected sample[0]==0, got %d", s.mixed[0])
		}
		for i, v := range s.mixed {
			if v != 0 {
				t.Errorf("sample[%d]: expected 0, got %d", i, v)
			}
		}
	})

	t.Run("remove makes buffer unavailable", func(t *testing.T) {
		s := newStreams()
		a := ringbuffer.New(3)
		a.Write([]int16{1, 2, 3})
		s.add("a", &a)
		s.remove("a")
		s.mix(3)
		if s.mixed[0] == 1 {
			t.Error("expected s.mix to not use removed buffer")
		}
	})
}

func randomPCMSample(amplitude float64) int16 {
	sample := rand.NormFloat64() * amplitude
	return clampInt16(sample)
}

const pcmAmplitude = 3000 // can be up to math.MaxInt16

func BenchmarkMix(b *testing.B) {
	const bufSize = pcmBufferSize * 8

	b.Run("branchless_n=1", func(b *testing.B) {
		samples := make([]int16, pcmBufferSize)
		for i := range samples {
			samples[i] = randomPCMSample(pcmAmplitude)
		}

		s := newStreams()
		s1 := ringbuffer.New(bufSize)
		s.add("s1", &s1)

		// NOTE: this will not give an accurate time measurement unless
		// you uncomment the timer calls, but doing that will make it run extremely slow.
		for b.Loop() {
			// b.StopTimer()
			s1.Write(samples)
			// b.StartTimer()
			s.mix(pcmBufferSize)
		}
	})

	b.Run("branchless_n=2", func(b *testing.B) {
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
			s.mix(pcmBufferSize)
		}
	})

	b.Run("branchless_n=4", func(b *testing.B) {
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
			s.mix(pcmBufferSize)
		}
	})

	b.Run("idiomatic_n=1", func(b *testing.B) {
		samples := make([]int16, pcmBufferSize)
		for i := range samples {
			samples[i] = randomPCMSample(pcmAmplitude)
		}

		s := newStreams()
		s1 := ringbuffer.New(bufSize)
		s.add("s1", &s1)

		for b.Loop() {
			s1.Write(samples)
			s.mixIdiomatic(pcmBufferSize)
		}
	})

	b.Run("idiomatic_n=2", func(b *testing.B) {
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
			s.mixIdiomatic(pcmBufferSize)
		}
	})

	b.Run("idiomatic_n=4", func(b *testing.B) {
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
			s.mixIdiomatic(pcmBufferSize)
		}
	})

}

func (s *streams) mixIdiomatic(numSamples int) {
	// get pointers to bufs with at least [numSamples] samples
	full, numFull := [MaxStreams]*[pcmBufferSize]int16{}, int32(0)
	for _, rb := range s.data {
		if rb.Len() >= numSamples {
			_ = rb.Read(s.writeBufs[numFull][:])
			full[numFull] = &s.writeBufs[numFull]
			numFull++
		}
	}

	// ensure previous mixed pcm is erased
	clear(s.mixed[:])
	if numFull == 0 || len(s.data) == 0 {
		return
	}

	// if only one other person in the room, don't mix, just write their pcm
	if numFull == 1 {
		copy(s.mixed[:], s.writeBufs[0][:numSamples])
		full[0] = nil
		return
	}

	// avoid bounds checks
	_, _ = full[numFull-1], s.mixed[numSamples-1]

	// sum samples for each buffer
	for i := range numSamples {
		var sum int32
		for j := range numFull {
			sum += int32(full[j][i])
		}
		s.mixed[i] = softSaturate(sum, math.MaxInt16)
	}

	// ensure these are cleaned up
	for i := range numFull {
		full[i] = nil
	}
}
