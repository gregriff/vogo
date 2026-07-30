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

	summed := [pcmBufferSize]int32{}

	// avoid bounds checks
	_ = full[numFull-1]      //nolint:gosec // G602: checked in streams.add
	_ = summed[numSamples-1] //nolint:gosec // G602: checked in streams.add

	// sum samples for each buffer
	const int16Size = unsafe.Sizeof(int16(0))
	var offset uintptr
	for i := range numSamples {
		var sum int32
		offset = uintptr(i) * int16Size
		for j := range numFull {
			// use ptr arithmetic for no bounds checks for branchless SIMD.
			sum += int32(*((*int16)(unsafe.Add(full[j], offset))))
		}
		summed[i] = sum
	}

	// actual mixing
	_ = s.mixed[numSamples-1]
	_ = summed[numSamples-1] //nolint:gosec // G602: checked in caller
	for i := range numSamples {
		s.mixed[i] = softSaturate(summed[i], math.MaxInt16)
	}

	// ensure these are cleaned up
	for i := range numFull {
		full[i] = nil
	}
}

// softSaturate takes a summed int32 value and a threshold,
// returns a soft-saturated int16 using tanh.
func softSaturate(sum int32, threshold float64) int16 {
	x := math.Tanh(float64(sum)/threshold) * threshold
	return clampInt16(x)
}

func softSaturatePade(sum int32, threshold float64) int16 {
	x := padeTanhScalar(float64(sum)/threshold) * threshold
	return clampInt16(x)
}

type summedPCMSample interface {
	int32 | float64
}

func clampInt16[S summedPCMSample](val S) int16 {
	return int16(min(max(val, math.MinInt16), math.MaxInt16))
}

func padeTanhScalar(x float64) float64 {
	const (
		clampPos = 4.97
		clampNeg = -4.97
		coeff    = 6
		constant = 15
	)

	if x > clampPos {
		x = clampPos
	} else if x < clampNeg {
		x = clampNeg
	}
	// x = max(min(x, clampPos), clampNeg)

	x2 := x * x
	num := x * (x2 + constant)
	den := coeff*x2 + constant
	y := num / den

	if y > 1 {
		y = 1
	} else if y < -1 {
		y = -1
	}
	// y = max(min(y, 1), -1)

	return y
}
