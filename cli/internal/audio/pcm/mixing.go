package pcm

import (
	"math"
	"unsafe"
)

// mixing.go defines architecture-agnostic functions for mixing pcm buffers.

// mixing algorithm constants
const (
	// [3/2] approximant diverges from tanh by 5% at x=3.
	// clamp inputs to +/- 3. Note: it exceeds 1.0 at x=2.32.
	padeMaxInput = 3.
	padeMinInput = -3.

	padeConst = 15.
	padeCoeff = 6.
)

// fullStreams returns an array of pointers to pcm streams with at least numSamples samples.
// If any 3 streams are full, it returns [p1, p2, p3, nil, nil]. It must be run inside s's mutex.
// When a full stream is found, it is read from the ringbuffer into one of s.writeBufs, to ensure
// the pcm is in contiguous memory and able to be accessed via pointer arithmetic in the mixing algo.
func (s *Streams) fullStreams(numSamples int) ([MaxStreams]unsafe.Pointer, int) {
	full, numFull := [MaxStreams]unsafe.Pointer{}, 0

	// TODO: research if you could have simd kernel read straight from RB to avoid these copies
	// to the temp arrays, and full could point to the rb itself at the right read index.
	for _, rb := range s.data {
		if rb != nil && rb.Len() >= numSamples {
			// copy the full pcm buf so we can vectorize access easily.
			_ = rb.Read(s.writeBufs[numFull][:])

			// since we're in the lock and ensured length, we can use unsafe access.
			full[numFull] = unsafe.Pointer(&(s.writeBufs[numFull])[0])
			numFull++
		}
	}
	return full, numFull
}

// softSaturateScalar takes the sum of multiple int16s and returns
// a value saturated to threshold using a Padé tanh approximant.
// Used to mix PCM samples and prevent hard clipping.
func softSaturateScalar(sum int32, threshold float32) int16 {
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
	x = max(min(x, padeMaxInput), padeMinInput)
	x2 := x * x
	numer := x * (x2 + padeConst)
	denom := padeCoeff*x2 + padeConst

	// clamping to [-1,1] is not needed here because the caller will
	// always convert to int16 with signed saturation
	return numer / denom
}
