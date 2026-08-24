package pcm

import (
	"fmt"
	"log"
	"math"
	"math/rand/v2"
	"testing"
	"unsafe"

	"github.com/gregriff/vogo/cli/internal/audio/ringbuffer"
)

// this file tests per-arch SIMD implementations of the mixing algorithm and
// compares them to the reference scalar implementations.
//
// best pre-refactor: 2=360ns, 4=460ns

func TestMix(t *testing.T) {
	t.Run("single full stream", func(t *testing.T) {
		numSamples := 5
		dst := [15]byte{}
		s := NewStreams()
		a := ringbuffer.New(numSamples)
		a.Write([]int16{10, 20, 30, 40, 50})
		s.ids[0] = "a"
		s.data[0] = &a
		s.MixAndWrite(dst[:], numSamples)
		ints := BytesToInt16(dst[:])

		count := 0
		for _, v := range ints {
			if v == 0 {
				break
			}
			count++
		}
		if count != numSamples {
			t.Errorf("incorrect number of mixed samples. got %d want %d", count, numSamples)
		}
	})

	t.Run("silence streams mix to silence", func(t *testing.T) {
		dst := [10]byte{}
		s := NewStreams()
		pcm := []int16{0, 0, 0}
		a := ringbuffer.New(3)
		b := ringbuffer.New(3)
		a.Write(pcm)
		b.Write(pcm)

		s.ids[0] = "a"
		s.data[0] = &a
		s.ids[1] = "b"
		s.data[1] = &b

		s.MixAndWrite(dst[:], 3)
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
		dst := [15]byte{}
		s := NewStreams()
		a := ringbuffer.New(3)
		a.Write([]int16{1, 2, 3})
		s.ids[0] = "a"
		s.data[0] = &a
		_ = s.Remove("a")
		s.MixAndWrite(dst[:], 3)
		if s.mixed[0] == 1 {
			t.Error("expected s.mix to not use removed buffer")
		}
	})
}

func randomPCMSample(amplitude float64) int16 {
	sample := rand.NormFloat64() * amplitude
	return clampInt16(sample)
}

// for generating normally distributed numbers that mock PCM data.
// don't do more than like 15k
const pcmAmplitude = 12_000

// dst buffer is []byte, so must be twice the int16 buffer size
const dstBufSize = BufferSize * 2.5

func BenchmarkMix(b *testing.B) {
	b.Run("pade_simd_n=1", func(b *testing.B) {
		samples := make([]int16, BufferSize)
		for i := range samples {
			samples[i] = randomPCMSample(pcmAmplitude)
		}

		dst := [dstBufSize]byte{}
		s := NewStreams()
		s.AddNew("s1")

		// NOTE: this will not give an accurate time measurement unless
		// you uncomment the timer calls, but doing that will make it run extremely slow.
		for b.Loop() {
			// b.StopTimer()
			s.WriteFrame("s1", samples)
			// b.StartTimer()
			s.MixAndWrite(dst[:], BufferSize)
		}
	})

	b.Run("pade_simd_n=2", func(b *testing.B) {
		samples := make([]int16, BufferSize)
		for i := range samples {
			samples[i] = randomPCMSample(pcmAmplitude)
		}

		dst := [dstBufSize]byte{}
		s := NewStreams()
		_ = s.AddNew("s1")
		_ = s.AddNew("s2")

		for b.Loop() {
			b.StopTimer()
			s.WriteFrame("s1", samples)
			s.WriteFrame("s2", samples)
			b.StartTimer()
			s.MixAndWrite(dst[:], BufferSize)
		}
	})

	b.Run("pade_simd_n=4", func(b *testing.B) {
		samples := make([]int16, BufferSize)
		for i := range samples {
			samples[i] = randomPCMSample(pcmAmplitude)
		}

		dst := [dstBufSize]byte{}
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
			s.MixAndWrite(dst[:], BufferSize)
		}
	})

	b.Run("pade_scalar_n=2", func(b *testing.B) {
		samples := make([]int16, BufferSize)
		for i := range samples {
			samples[i] = randomPCMSample(pcmAmplitude)
		}

		dst := [dstBufSize]byte{}
		s := NewStreams()
		_ = s.AddNew("s1")
		_ = s.AddNew("s2")

		for b.Loop() {
			b.StopTimer()
			s.WriteFrame("s1", samples)
			s.WriteFrame("s2", samples)
			b.StartTimer()
			s.MixAndWriteScalar(dst[:], BufferSize)
		}
	})

	b.Run("pade_scalar_n=4", func(b *testing.B) {
		samples := make([]int16, BufferSize)
		for i := range samples {
			samples[i] = randomPCMSample(pcmAmplitude)
		}

		dst := [dstBufSize]byte{}
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
			s.MixAndWriteScalar(dst[:], BufferSize)
		}
	})

	b.Run("pade_clean_n=2", func(b *testing.B) {
		samples := make([]int16, BufferSize)
		for i := range samples {
			samples[i] = randomPCMSample(pcmAmplitude)
		}

		dst := [dstBufSize]byte{}
		s := NewStreams()
		_ = s.AddNew("s1")
		_ = s.AddNew("s2")

		for b.Loop() {
			b.StopTimer()
			s.WriteFrame("s1", samples)
			s.WriteFrame("s2", samples)
			b.StartTimer()
			s.MixAndWriteIdiomatic(dst[:], BufferSize)
		}
	})

	b.Run("pade_clean_n=4", func(b *testing.B) {
		samples := make([]int16, BufferSize)
		for i := range samples {
			samples[i] = randomPCMSample(pcmAmplitude)
		}

		dst := [dstBufSize]byte{}
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
			s.MixAndWriteIdiomatic(dst[:], BufferSize)
		}
	})

	b.Run("tanh_n=2", func(b *testing.B) {
		samples := make([]int16, BufferSize)
		for i := range samples {
			samples[i] = randomPCMSample(pcmAmplitude)
		}

		dst := [dstBufSize]byte{}
		s := NewStreams()
		_ = s.AddNew("s1")
		_ = s.AddNew("s2")

		for b.Loop() {
			b.StopTimer()
			s.WriteFrame("s1", samples)
			s.WriteFrame("s2", samples)
			b.StartTimer()
			s.MixAndWriteTanh(dst[:], BufferSize)
		}
	})

	b.Run("tanh_n=4", func(b *testing.B) {
		samples := make([]int16, BufferSize)
		for i := range samples {
			samples[i] = randomPCMSample(pcmAmplitude)
		}

		dst := [dstBufSize]byte{}
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
			s.MixAndWriteTanh(dst[:], BufferSize)
		}
	})
}

func (s *Streams) MixAndWriteScalar(dst []byte, numSamples int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if numSamples > len(s.mixed) {
		log.Panicf("samplesToRead > cap(mixed)")
	}

	full, numFull := s.fullStreams(numSamples)
	defer clear(full[:])

	switch numFull {
	case 0:
		return // nothing to write
	case 1:
		// if only one other person in the room, don't mix, just write their pcm
		ints := ringbuffer.Int16ToBytes(s.writeBufs[0][:numSamples])
		copy(dst, ints)
		return
	}

	// write a full mixed sample to the speaker buffer
	s.mixScalar(full, numFull, numSamples)
	mixed := ringbuffer.Int16ToBytes(s.mixed[:numSamples])
	copy(dst, mixed)
	clear(s.mixed[:])
}

func (s *Streams) MixAndWriteIdiomatic(dst []byte, numSamples int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if numSamples > len(s.mixed) {
		log.Panicf("samplesToRead > cap(mixed)")
	}

	full, numFull := s.fullStreamsIdiomatic(numSamples)
	defer clear(full[:])

	switch numFull {
	case 0:
		return // nothing to write
	case 1:
		// if only one other person in the room, don't mix, just write their pcm
		ints := ringbuffer.Int16ToBytes(s.writeBufs[0][:numSamples])
		copy(dst, ints)
		return
	}

	// write a full mixed sample to the speaker buffer
	s.mixIdiomatic(full, numFull, numSamples)
	mixed := ringbuffer.Int16ToBytes(s.mixed[:numSamples])
	copy(dst, mixed)
	clear(s.mixed[:])
}

func (s *Streams) MixAndWriteTanh(dst []byte, numSamples int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if numSamples > len(s.mixed) {
		log.Panicf("samplesToRead > cap(mixed)")
	}

	full, numFull := s.fullStreams(numSamples)
	defer clear(full[:])

	switch numFull {
	case 0:
		return // nothing to write
	case 1:
		// if only one other person in the room, don't mix, just write their pcm
		ints := ringbuffer.Int16ToBytes(s.writeBufs[0][:numSamples])
		copy(dst, ints)
		return
	}

	// write a full mixed sample to the speaker buffer
	s.mixTanh(full, numFull, numSamples)
	mixed := ringbuffer.Int16ToBytes(s.mixed[:numSamples])
	copy(dst, mixed)
	clear(s.mixed[:])
}

// Mix takes all pcm bufs that have at least [numSamples] samples and mixes their pcm data
// using a Pade tanh approximant, writing the result to [s.mixed]. It must be run within
// a mutex lock. If [full] is empty due to network conditions, or [s.bufs] is empty due
// to none being added, the caller can still write [s.mixed] to the speaker because it
// is zeroed, and the speaker will play silence. Assumes numSamples <= cap(s.mixed).
// This function is the reference spec for Pade mixing
// functions and is not used outside of testing due to faster SIMD variants.
func (s *Streams) mixScalar(full [MaxStreams]unsafe.Pointer, numFull, numSamples int) {
	// avoid bounds checks
	_ = full[numFull-1]       //nolint:gosec // G602: checked in streams.AddNew
	_ = s.mixed[numSamples-1] //nolint:gosec // G602: checked in streams.AddNew

	// sum samples for each buffer
	const int16Size = unsafe.Sizeof(int16(0))
	var offset uintptr
	var sample *int16
	for i := range numSamples {
		var sum int32
		offset = uintptr(i) * int16Size
		for j := range numFull {
			sample = (*int16)(unsafe.Add(full[j], offset))
			sum += int32(*(sample))
		}
		s.mixed[i] = softSaturateScalar(sum, math.MaxInt16)
	}
}

// mixIdiomatic mixes pcm streams with a Pade approximant without using unsafe, pointer
// arithmetic or other things that are not go idiomatic. Used as a control in benchmarks.
func (s *Streams) mixIdiomatic(full [MaxStreams]*[BufferSize]int16, numFull, numSamples int) {
	// sum samples for each buffer
	for i := range numSamples {
		var sum int32
		for j := range numFull {
			sum += int32(full[j][i])
		}
		s.mixed[i] = softSaturateScalar(sum, math.MaxInt16)
	}
}

// mixTanh is a legacy scalar mixing pcm mixing algorithm that has since been replaced
// by simd implementations that use approximations for improved speed. This func, now
// used a control for testing the newer variants, uses math.Tanh to soft-saturate
// the mixed PCM.
func (s *Streams) mixTanh(full [MaxStreams]unsafe.Pointer, numFull, numSamples int) {
	// avoid bounds checks
	_ = full[numFull-1]       //nolint:gosec // G602: checked in streams.AddNew
	_ = s.mixed[numSamples-1] //nolint:gosec // G602: checked in streams.AddNew

	// sum the samples across all pcm streams. Uses pointer arithmetic
	// to access each element to avoid bounds checks.
	const int16Size = unsafe.Sizeof(int16(0))
	for i := range numSamples {
		var sum int32
		offset := uintptr(i) * int16Size
		for j := range numFull {
			sample := (*int16)(unsafe.Add(full[j], offset))
			sum += int32(*(sample))
		}
		s.mixed[i] = softSaturateTanh(sum, math.MaxInt16)
	}
}

// softSaturateTanh takes a sum of multiple int16s and returns
// a value saturated to threshold using tanh. Used to mix
// PCM samples and prevent hard clipping.
func softSaturateTanh(sum int32, threshold float64) int16 {
	x := math.Tanh(float64(sum)/threshold) * threshold
	return clampInt16(x)
}

func (s *Streams) fullStreamsIdiomatic(numSamples int) ([MaxStreams]*[BufferSize]int16, int) {
	full, numFull := [MaxStreams]*[BufferSize]int16{}, 0
	for _, rb := range s.data {
		if rb != nil && rb.Len() >= numSamples {
			_ = rb.Read(s.writeBufs[numFull][:])
			full[numFull] = &s.writeBufs[numFull]
			numFull++
		}
	}
	return full, numFull
}

// TestSoftSaturate_PadeVsTanh compares softSaturate (real tanh) against
// softSaturate (Padé approximant) across the realistic range of
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
		tanh := softSaturateTanh(sum, threshold)
		pade := softSaturateScalar(sum, threshold)

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

	// TODO: should mock real voice chat pcm, and only check the max/mean diff
	// in the normal ranges of 2+ people speaking.
	const maxAllowedDiff = 1000 // in int16 units
	if maxAbsDiff > maxAllowedDiff {
		t.Errorf("max abs diff %d exceeds allowed tolerance %d (sum=%d)",
			maxAbsDiff, maxAllowedDiff, maxDiffSum)
	}

	const maxAllowedMeanDiff = 150
	if meanAbsDiff > maxAllowedMeanDiff {
		t.Errorf("mean abs diff %.4f exceeds allowed tolerance %d", meanAbsDiff, maxAllowedMeanDiff)
	}
}

// TestSoftSaturate_PadeVsTanh_Table spot-checks specific edge and
// typical sum values (max/min sums, 2-5 sample sums) for
// readability/debugging when the sweep test fails. Threshold is fixed
// at math.MaxInt16, matching production usage.
func TestSoftSaturate_PadeVsTanh_Table(t *testing.T) {
	const threshold = math.MaxInt16

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
			tanh := softSaturateTanh(sum, threshold)
			pade := softSaturateScalar(sum, threshold)
			diff := int32(tanh) - int32(pade)
			t.Logf("tanh=%d pade=%d diff=%d", tanh, pade, diff)
		})
	}
}
