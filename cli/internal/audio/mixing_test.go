package audio

import (
	"fmt"
	"math"
	"math/rand/v2"
	"testing"
	"unsafe"

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

	b.Run("pade_n=1", func(b *testing.B) {
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
			s.mixPade(pcmBufferSize)
		}
	})

	b.Run("pade_n=2", func(b *testing.B) {
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
			s.mixPade(pcmBufferSize)
		}
	})

	b.Run("pade_n=4", func(b *testing.B) {
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
			s.mixPade(pcmBufferSize)
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
	// _, _ = full[numFull-1], s.mixed[numSamples-1]

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

func (s *streams) mixPade(numSamples int) {
	// get pointers to bufs with at least [numSamples] samples
	full, numFull := [MaxStreams]unsafe.Pointer{}, 0

	for _, rb := range s.data {
		if rb.Len() >= numSamples {
			// copy the full pcm buf so we can vectorize access easily.
			_ = rb.Read(s.writeBufs[numFull][:])

			// since we're in the lock and ensured length, we can use unsafe access.
			full[numFull] = unsafe.Pointer(&(s.writeBufs[numFull])[0])
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
	_ = full[numFull-1] //nolint:gosec // G602: checked in streams.add

	// sum samples for each buffer
	const int16Size = unsafe.Sizeof(int16(0))
	var offset uintptr
	for i := range numSamples {
		var sum int32
		offset = uintptr(i) * int16Size
		for j := range numFull {
			sum += int32(*((*int16)(unsafe.Add(full[j], offset))))
		}
		s.mixed[i] = softSaturatePade(sum, math.MaxInt16)
	}

	// ensure these are cleaned up
	for i := range numFull {
		full[i] = nil
	}
}

// TestSoftSaturate_PadeVsTanh compares softSaturate (real tanh) against
// softSaturatePade (Padé approximant) across the realistic range of
// summed PCM samples (2-5 int16s), using the fixed threshold math.MaxInt16.
func TestSoftSaturate_PadeVsTanh(t *testing.T) {
	const threshold = math.MaxInt16

	// Sums of 2-5 int16 samples: range roughly [-5*32768, 5*32767].
	const maxSum = 5 * math.MaxInt16

	var (
		maxAbsDiff    int32
		maxDiffSum    int32
		sumAbsDiff    int64
		count         int64
		diffHistogram = map[int32]int{}
	)

	// Step by a coarse amount for full sweep; refine (lower step) to
	// hit narrow edge-case regressions.
	step := int32(7) // coprime-ish odd step to hit varied values

	for sum := int32(-maxSum); sum <= maxSum; sum += step {
		tanh := softSaturate(sum, threshold)
		pade := softSaturatePade(sum, threshold)

		diff := int32(tanh) - int32(pade)
		if diff < 0 {
			diff = -diff
		}

		sumAbsDiff += int64(diff)
		count++
		diffHistogram[diff]++

		if diff > maxAbsDiff {
			maxAbsDiff = diff
			maxDiffSum = sum
		}
	}

	meanAbsDiff := float64(sumAbsDiff) / float64(count)

	t.Logf("compared %d samples (threshold=%.0f)", count, float32(threshold))
	t.Logf("mean abs diff: %.4f", meanAbsDiff)
	t.Logf("max abs diff: %d (sum=%d)", maxAbsDiff, maxDiffSum)

	// Print histogram sorted-ish for quick eyeballing.
	for d := int32(0); d <= maxAbsDiff; d++ {
		if c, ok := diffHistogram[d]; ok {
			t.Logf("diff=%d count=%d", d, c)
		}
	}

	// Adjust tolerance to whatever is acceptable for your use case.
	const maxAllowedDiff = 50 // in int16 units
	if maxAbsDiff > maxAllowedDiff {
		t.Errorf("max abs diff %d exceeds allowed tolerance %d (sum=%d)",
			maxAbsDiff, maxAllowedDiff, maxDiffSum)
	}

	const maxAllowedMeanDiff = 5.0
	if meanAbsDiff > maxAllowedMeanDiff {
		t.Errorf("mean abs diff %.4f exceeds allowed tolerance %.4f", meanAbsDiff, maxAllowedMeanDiff)
	}
}

// TestSoftSaturate_PadeVsTanh_Table spot-checks specific edge and
// typical sum values (max/min sums, 2-5 sample sums) for
// readability/debugging when the sweep test fails. Threshold is fixed
// at math.MaxInt16, matching production usage.
func TestSoftSaturate_PadeVsTanh_Table(t *testing.T) {
	const thr = float64(math.MaxInt16)

	sums := []int32{
		0,
		32767,
		-32768,
		65535, // 2 samples summed near max
		-65536,
		98301,  // 3 samples summed near max
		163835, // 5 samples summed near max
		-163840,
		1000,
		-1000,
		5000,
		-5000,
		15000,
		-15000,
		25000,
		-25000,
		30000,
		-30000,
		32000,
		-32000,
	}

	for _, sum := range sums {
		t.Run(fmt.Sprintf("sum=%d", sum), func(t *testing.T) {
			tanh := softSaturate(sum, thr)
			pade := softSaturatePade(sum, float32(thr))
			diff := int32(tanh) - int32(pade)
			t.Logf("tanh=%d pade=%d diff=%d", tanh, pade, diff)
		})
	}
}
