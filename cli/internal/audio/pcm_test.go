package audio

import (
	"math"
	"math/rand/v2"
	"testing"
	"unsafe"

	"github.com/gregriff/vogo/cli/internal/audio/ringbuffer"
)

func TestStreamsMix(t *testing.T) {
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
		pcm := []int16{math.MaxInt16, math.MaxInt16}
		a := ringbuffer.New(2)
		b := ringbuffer.New(2)
		a.Write(pcm)
		b.Write(pcm)
		s.add("a", &a)
		s.add("b", &b)
		s.mix(2)
		if s.mixed[0] > math.MaxInt16 {
			t.Errorf("sample overflowed int16: %d", s.mixed[0])
		}
	})

	t.Run("clamps underflow", func(t *testing.T) {
		s := newStreams()
		pcm := []int16{math.MaxInt16, math.MaxInt16}
		a := ringbuffer.New(2)
		b := ringbuffer.New(2)
		a.Write(pcm)
		b.Write(pcm)
		s.add("a", &a)
		s.add("b", &b)
		s.mix(2)
		if s.mixed[0] < math.MinInt16 {
			t.Errorf("sample underflow int16: %d", s.mixed[0])
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

	b.Run("branchless-v1_s=1", func(b *testing.B) {
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

	b.Run("branchless-v1_s=2", func(b *testing.B) {
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

	b.Run("branchless-v1_s=3", func(b *testing.B) {
		samples := make([]int16, pcmBufferSize)
		for i := range samples {
			samples[i] = randomPCMSample(pcmAmplitude)
		}

		s := newStreams()
		s1 := ringbuffer.New(bufSize)
		s2 := ringbuffer.New(bufSize)
		s3 := ringbuffer.New(bufSize)
		s.add("s1", &s1)
		s.add("s2", &s2)
		s.add("s3", &s3)

		for b.Loop() {
			b.StopTimer()
			s1.Write(samples)
			s2.Write(samples)
			s3.Write(samples)
			b.StartTimer()
			s.mix(pcmBufferSize)
		}
	})

	b.Run("slow_s=1", func(b *testing.B) {
		samples := make([]int16, pcmBufferSize)
		for i := range samples {
			samples[i] = randomPCMSample(pcmAmplitude)
		}

		s := newStreams()
		s1 := ringbuffer.New(bufSize)
		s.add("s1", &s1)

		for b.Loop() {
			s1.Write(samples)
			s.mixSlow(pcmBufferSize)
		}
	})

	b.Run("slow_s=2", func(b *testing.B) {
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
			s.mixSlow(pcmBufferSize)
		}
	})

	b.Run("slow_s=3", func(b *testing.B) {
		samples := make([]int16, pcmBufferSize)
		for i := range samples {
			samples[i] = randomPCMSample(pcmAmplitude)
		}

		s := newStreams()
		s1 := ringbuffer.New(bufSize)
		s2 := ringbuffer.New(bufSize)
		s3 := ringbuffer.New(bufSize)
		s.add("s1", &s1)
		s.add("s2", &s2)
		s.add("s3", &s3)

		for b.Loop() {
			b.StopTimer()
			s1.Write(samples)
			s2.Write(samples)
			s3.Write(samples)
			b.StartTimer()
			s.mixSlow(pcmBufferSize)
		}
	})

}

func (s *streams) mixSlow(numSamples int) {

	// get pointers to bufs with at least [numSamples] samples
	full, numFull := [maxStreams]bufView{}, int32(0)
	for _, rb := range s.data {
		if rb.Len() >= numSamples {
			// since we're in the lock and ensured length, we can use unsafe access
			_ = rb.Read(s.writeBufs[numFull][:])
			full[numFull] = newBufView(&s.writeBufs[numFull])
			numFull++
		}
	}

	// ensure previous mixed pcm is erased
	for i := range s.mixed {
		s.mixed[i] = 0
	}
	if numFull == 0 || len(s.data) == 0 {
		return
	}

	// if only one other person in the room, don't mix, just write their pcm
	if numFull == 1 {
		src := *full[0].buf

		// avoid bounds checks
		// _ = s.mixed[numSamples-1]
		// _ = src[numSamples-1]

		// TODO: replace with copy() and test
		// TODO: can call rb.Read(s.mixed[:numSamples]) here BUT Read() is already called above
		for i := range numSamples {
			s.mixed[i] = src[i]
		}
		// copy(s.mixed[:], src[:numSamples])
		// remove samples from stream that was written.
		// *full[0].buf = src[numSamples:]  // unness currently
		full[0].buf = nil
		full[0].ptr = nil
		return
	}

	// avoid bounds checks
	// _ = s.mixed[numSamples-1]
	// _ = full[numFull-1]

	// mix full pcm bufs and write to s.mixed
	// NOTE: for SIMD, it's prob better to create an array of all the sums,
	// then do the mixing on those sums after arr is full.
	const zero = int32(0)
	const int16Size = unsafe.Sizeof(int16(0))
	var sum int32
	var offset uintptr
	for i := range numSamples {
		sum = zero
		offset = uintptr(i) * int16Size
		for j := range numFull { // TODO: use SIMD
			// use ptr arithmetic for no bounds checks for branchless SIMD.
			sum += int32(*((*int16)(unsafe.Add(full[j].ptr, offset))))
		}
		// s.mixed[i] = clampInt16(sum / numFull)
		s.mixed[i] = softSaturate(sum, math.MaxInt16)
	}

	// ensure these are cleaned up
	for i := range numFull {
		full[i].buf = nil
		full[i].ptr = nil
	}
}
