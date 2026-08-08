package audio

import (
	"math"
	"unsafe"
)

// mix takes all [s.bufs] that have at least [numSamples] samples and mixes their pcm data, writing the result to
// [s.mixed]. It must be run within a mutex lock. If [full] is empty due to network conditions,
// or [s.bufs] is empty due to none being added, the caller can still write [s.mixed]
// to the speaker because it is zeroed, and the speaker will play silence.
// Assumes numSamples <= cap(s.mixed) and len(s.bufs) <= maxStreams.
func (s *streams) mix(numSamples int) {
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
		s.mixed[i] = softSaturate(sum, math.MaxInt16)
	}

	// ensure these are cleaned up
	for i := range numFull {
		full[i] = nil
	}
}

// softSaturate takes a sum of multiple int16s and returns
// a value saturated to threshold using tanh. Used to mix
// PCM samples and prevent hard clipping.
func softSaturate(sum int32, threshold float64) int16 {
	x := math.Tanh(float64(sum)/threshold) * threshold
	return clampInt16(x)
}

// softSaturatePade takes the sum of multiple int16s and returns
// a value saturated to threshold using a Padé tanh approximant.
// Used to mix PCM samples and prevent hard clipping.
func softSaturatePade(sum int32, threshold float32) int16 {
	x := padeTanhScalar(float32(sum)/threshold) * threshold
	return clampInt16(x)
}

type summedPCMSample interface {
	int32 | float64 | float32
}

// clampInt16 converts its input to int16 with signed saturation.
func clampInt16[S summedPCMSample](val S) int16 {
	return int16(min(max(val, math.MinInt16), math.MaxInt16))
}

// padeTanhScalar applies a [3/2] Padé tanh approximant on a
// float32 using tanh(x) ≈ x*(x²+15) / (6x²+15). It is used
// during audio mixing, where x is the ratio of the sum of
// multiple PCM samples to a clipping threshold.
func padeTanhScalar(x float32) float32 {
	const (
		clampPos = 4.97
		clampNeg = -4.97
		coeff    = 6.
		constant = 15.
	)
	x = max(min(x, clampPos), clampNeg)
	x2 := x * x
	numer := x * (x2 + constant)
	denom := coeff*x2 + constant

	// clamping to [-1,1] is not needed here because the caller will
	// always convert to int16 with signed saturation
	return numer / denom
}
