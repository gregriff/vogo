package pcm

import (
	"math"
	"unsafe"
)

const (
	// [3/2] approximant diverges from tanh by 5% at x=3.
	// clamp inputs to +/- 3. Note: it exceeds 1.0 at x=2.32.
	padeMaxInput = 3.
	padeMinInput = -3.

	padeConst = 15.
	padeCoeff = 6.
)

// preMix is the first step of the mixing routine. It returns an array where
// each element points to a pcm buffer that contains at least numSamples samples.
// If zero or only one have enough samples, it stops the mixing routine, because
// there is none to do. preMix is also responsible for zeroing the mixing sink,
// since miniaudio has been configured to not do it itself.
func (s *Streams) preMix(numSamples int) ([MaxStreams]unsafe.Pointer, int, bool) {
	full, numFull, done := [MaxStreams]unsafe.Pointer{}, 0, false

	// ensure previous mixed pcm is erased
	// TODO: is this even needed? Just clear the tail?
	clear(s.mixed[:])

	// get pointers to bufs with at least [numSamples] samples
	// TODO: research if you could have simd kernel read straight from RB to avoid these copies
	// to the temp arrays, and full could point to the rb itself at the right read index.
	// Also s.data could be a []*ringbuffer. but if you do that you need to write tests to ensure that
	// removing/replacing one of them (someone leaves/joins a call) works properly. would need to add a
	// uuid field to ringbuffer struct, and remove/replaceRB() methods to streams.
	for _, rb := range s.data {
		if rb != nil && rb.Len() >= numSamples {
			// copy the full pcm buf so we can vectorize access easily.
			_ = rb.Read(s.writeBufs[numFull][:])

			// since we're in the lock and ensured length, we can use unsafe access.
			full[numFull] = unsafe.Pointer(&(s.writeBufs[numFull])[0])
			numFull++
		}
	}

	switch numFull {
	case 0:
		done = true
		return full, numFull, done
	case 1:
		// if only one other person in the room, don't mix, just write their pcm
		copy(s.mixed[:], s.writeBufs[0][:numSamples])
		full[0] = nil
		done = true
	}
	return full, numFull, done
}

// mix takes all [s.bufs] that have at least [numSamples] samples and mixes their pcm data
// using a Pade tanh approximant, writing the result to [s.mixed]. It must be run within
// a mutex lock. If [full] is empty due to network conditions, or [s.bufs] is empty due
// to none being added, the caller can still write [s.mixed] to the speaker because it
// is zeroed, and the speaker will play silence. Assumes numSamples <= cap(s.mixed)
// and len(s.bufs) <= maxStreams. This function is the reference spec for Pade mixing
// functions and is not used outside of testing due to faster SIMD variants.
func (s *Streams) mix(numSamples int) {
	full, numFull, done := s.preMix(numSamples)
	if done {
		return
	}

	// avoid bounds checks
	_ = full[numFull-1]       //nolint:gosec // G602: checked in streams.add
	_ = s.mixed[numSamples-1] //nolint:gosec // G602: checked in streams.add

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
		s.mixed[i] = softSaturate(sum, math.MaxInt16)
	}

	// ensure these are cleaned up
	for i := range numFull {
		full[i] = nil
	}
}

// softSaturate takes the sum of multiple int16s and returns
// a value saturated to threshold using a Padé tanh approximant.
// Used to mix PCM samples and prevent hard clipping.
func softSaturate(sum int32, threshold float32) int16 {
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
//
// Notes:
//   - if this hard-clips too much when there are many pcm streams,
//     consider a [5/4] approximant, or custom-fitting a rational
//     function with least-squares. (minimax, use scipy for this)
//
// https://mathr.co.uk/blog/2017-09-06_approximating_hyperbolic_tangent.html
//
// Alternative implementations:
//
// - LUT: https://jtomschroeder.com/blog/approximating-tanh/#k-tanh
//
// - FP Approx: https://jtomschroeder.com/blog/approximating-tanh/#schraudolph
//
// - SIMD Examples: https://yaikhom.com/2020-04-28-localised-approximation-of-hyperbolic-tangents.html
func padeTanhScalar(x float32) float32 {
	const (
		coeff    = 6.
		constant = 15.
	)
	x = max(min(x, padeMaxInput), padeMinInput)
	x2 := x * x
	numer := x * (x2 + constant)
	denom := coeff*x2 + constant

	// clamping to [-1,1] is not needed here because the caller will
	// always convert to int16 with signed saturation
	return numer / denom
}
