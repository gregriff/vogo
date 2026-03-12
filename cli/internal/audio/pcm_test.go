package audio

import (
	"math"
	"math/rand/v2"
	"testing"
)

func TestStreamsMix(t *testing.T) {
	t.Run("single full stream", func(t *testing.T) {
		numSamples := 5
		s := newStreams()
		buf := []int16{10, 20, 30, 40, 50}
		s.add("a", &buf)
		s.mix(numSamples)
		for i, want := range buf {
			if s.mixed[i] != want {
				t.Errorf("sample[%d]: got %d, want %d", i, s.mixed[i], want)
			}
		}
		count := 0
		for _, v := range s.mixed {
			if v == 0 {
				break
			}
			count++
		}
		if count != numSamples {
			t.Errorf("num mixed samples in mixed buf (%d) != numSamples arg (%d)", numSamples, numSamples)
		}
	})

	// when using softSaturate() this won't pass
	// t.Run("two streams averaged correctly", func(t *testing.T) {
	// 	s := newStreams()
	// 	a := []int16{100}
	// 	b := []int16{200}
	// 	s.add("a", &a)
	// 	s.add("b", &b)
	// 	s.mix(1)
	// 	if s.mixed[0] != 150 {
	// 		t.Errorf("sample mix err: got %d, want %d", s.mixed[0], 150)
	// 	}
	// })

	t.Run("clamps overflow", func(t *testing.T) {
		s := newStreams()
		a := []int16{math.MaxInt16, math.MaxInt16}
		b := []int16{math.MaxInt16, math.MaxInt16}
		s.add("a", &a)
		s.add("b", &b)
		s.mix(2)
		if s.mixed[0] > math.MaxInt16 {
			t.Errorf("sample overflowed int16: %d", s.mixed[0])
		}
	})
	t.Run("clamps underflow", func(t *testing.T) {
		s := newStreams()
		a := []int16{math.MaxInt16, math.MaxInt16}
		b := []int16{math.MaxInt16, math.MaxInt16}
		s.add("a", &a)
		s.add("b", &b)
		s.mix(2)
		if s.mixed[0] < math.MinInt16 {
			t.Errorf("sample underflow int16: %d", s.mixed[0])
		}
	})

	t.Run("silence streams mix to silence", func(t *testing.T) {
		s := newStreams()
		a := []int16{0, 0, 0}
		b := []int16{0, 0, 0}
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
		buf := []int16{1, 2, 3}
		s.add("a", &buf)
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
	b.Run("branchless-v1_s=1", func(b *testing.B) {
		samples := make([]int16, pcmBufferSize)
		for i := range samples {
			samples[i] = randomPCMSample(pcmAmplitude)
		}

		s := newStreams()
		s1 := make([]int16, 0, pcmBufferSize)
		s.add("s1", &s1)

		for b.Loop() {
			// reset + copy — no allocations (capacity already exists)
			s1 = append(s1[:0], samples...)
			s.mix(pcmBufferSize)
		}
	})

	b.Run("branchless-v1_s=2", func(b *testing.B) {
		samples := make([]int16, pcmBufferSize)
		for i := range samples {
			samples[i] = randomPCMSample(pcmAmplitude)
		}

		s := newStreams()
		s1 := make([]int16, 0, pcmBufferSize)
		s2 := make([]int16, 0, pcmBufferSize)
		s.add("s1", &s1)
		s.add("s2", &s2)

		for b.Loop() {
			b.StopTimer()
			s1 = append(s1[:0], samples...)
			s2 = append(s2[:0], samples...)
			b.StartTimer()
			s.mix(pcmBufferSize)
		}
	})

	b.Run("branchless-v1_s=3", func(b *testing.B) {
		samples := make([]int16, pcmBufferSize)
		for i := range samples {
			samples[i] = randomPCMSample(pcmAmplitude)
		}

		s := newStreams()
		s1 := make([]int16, 0, pcmBufferSize)
		s2 := make([]int16, 0, pcmBufferSize)
		s3 := make([]int16, 0, pcmBufferSize)
		s.add("s1", &s1)
		s.add("s2", &s2)
		s.add("s3", &s3)

		for b.Loop() {
			b.StopTimer()
			s1 = append(s1[:0], samples...)
			s2 = append(s2[:0], samples...)
			s3 = append(s3[:0], samples...)
			b.StartTimer()
			s.mix(pcmBufferSize)
		}
	})

	b.Run("noBufView_s=1", func(b *testing.B) {
		samples := make([]int16, pcmBufferSize)
		for i := range samples {
			samples[i] = randomPCMSample(pcmAmplitude)
		}

		s := newStreams()
		s1 := make([]int16, 0, pcmBufferSize)
		s.add("s1", &s1)

		for b.Loop() {
			// reset + copy — no allocations (capacity already exists)
			s1 = append(s1[:0], samples...)
			s.mixNoBufView(pcmBufferSize)
		}
	})

	b.Run("noBufView_s=2", func(b *testing.B) {
		samples := make([]int16, pcmBufferSize)
		for i := range samples {
			samples[i] = randomPCMSample(pcmAmplitude)
		}

		s := newStreams()
		s1 := make([]int16, 0, pcmBufferSize)
		s2 := make([]int16, 0, pcmBufferSize)
		s.add("s1", &s1)
		s.add("s2", &s2)

		for b.Loop() {
			b.StopTimer()
			s1 = append(s1[:0], samples...)
			s2 = append(s2[:0], samples...)
			b.StartTimer()
			s.mixNoBufView(pcmBufferSize)
		}
	})

	b.Run("noBufView_s=3", func(b *testing.B) {
		samples := make([]int16, pcmBufferSize)
		for i := range samples {
			samples[i] = randomPCMSample(pcmAmplitude)
		}

		s := newStreams()
		s1 := make([]int16, 0, pcmBufferSize)
		s2 := make([]int16, 0, pcmBufferSize)
		s3 := make([]int16, 0, pcmBufferSize)
		s.add("s1", &s1)
		s.add("s2", &s2)
		s.add("s3", &s3)

		for b.Loop() {
			b.StopTimer()
			s1 = append(s1[:0], samples...)
			s2 = append(s2[:0], samples...)
			s3 = append(s3[:0], samples...)
			b.StartTimer()
			s.mixNoBufView(pcmBufferSize)
		}
	})

}

func (s *streams) mixNoBufView(numSamples int) {
	// get pointers to bufs with at least [numSamples] samples
	full, numFull := [maxStreams]*[]int16{}, int32(0)
	for _, buf := range s.bufs {
		if len(*buf) >= numSamples {
			// since we're in the lock and ensured length, we can use unsafe access
			full[numFull] = buf
			numFull++
		}
	}

	// ensure previous mixed pcm is erased
	for i := range s.mixed {
		s.mixed[i] = 0
	}
	if numFull == 0 || len(s.bufs) == 0 {
		return
	}

	// if only one other person in the room, don't mix, just write their pcm
	if numFull == 1 {
		src := *full[0]

		// avoid bounds checks
		// _ = s.mixed[numSamples-1]
		// _ = src[numSamples-1]

		for i := range numSamples {
			s.mixed[i] = src[i]
		}
		// remove samples from stream that was written.
		*full[0] = src[numSamples:]
		full[0] = nil
		return
	}

	// avoid bounds checks
	// _ = s.mixed[numSamples-1]
	// _ = full[numFull-1]

	// mix full pcm bufs and write to s.mixed
	// NOTE: for SIMD, it's prob better to create an array of all the sums,
	// then do the mixing on those sums after arr is full.
	const zero = int32(0)
	var sum int32
	for i := range numSamples {
		sum = zero
		for j := range numFull { // TODO: use SIMD
			sum += int32((*full[j])[i])
		}
		// s.mixed[i] = clampInt16(sum / numFull)
		s.mixed[i] = softSaturate(sum, math.MaxInt16)
	}

	// remove samples from streams that were just mixed.
	for i := range numFull {
		*full[i] = (*full[i])[numSamples:]
		full[i] = nil
	}
}
