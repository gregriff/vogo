package audio

import (
	"math"
	"simd/archsimd"
	"unsafe"
)

// stubs for compilation on arm64

func (s *streams) mixAVX512(int) {
	panic("not implemented")
}
func (s *streams) mixAVX2(int) {
	panic("not implemented")
}

func (s *streams) mixNEON(numSamples int) {
	// get pointers to bufs with at least [numSamples] samples
	full, numFull := [MaxStreams]unsafe.Pointer{}, 0

	// research if you could have simd kernel read straight from RB to avoid these copies
	// to the temp arrays, and full could point to the rb itself at the right read index.
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
	// TODO: is this even needed? Just clear the tail?
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
	_ = full[numFull-1]       //nolint:gosec // G602: checked in streams.add
	_ = s.mixed[numSamples-1] //nolint:gosec // G602: checked in streams.add

	const simdWidth = 4
	const int16Size = unsafe.Sizeof(int16(0))
	vThreshold := archsimd.BroadcastFloat32x4(math.MaxInt16)

	var i = 0
	for ; i+simdWidth <= numSamples; i += simdWidth {
		acc := archsimd.BroadcastInt32x4(0)
		offset := uintptr(i) * int16Size
		for j := range numFull {
			// we are only processing 4 nums at a time, but
			// we can't load only 4 int16s into NEON registers.
			samples := (*[8]int16)(unsafe.Add(full[j], offset))
			vSamples := archsimd.LoadInt16x8Array(samples)
			acc = acc.Add(vSamples.ExtendLo4ToInt32())
		}
		saturated := softSaturatePadeNEON(acc, vThreshold)

		dst := s.mixed[i:]
		if len(dst) > 4 {
			// fast path, true until the last iteration.
			// this writes 8 elems, only 4 are pcm, rest are zeroes.
			// next iter overwrites those zeroes.
			saturated.StoreArray((*[8]int16)(dst))
		} else {
			saturated.StorePart(dst)
		}

		// this is copied from archsimd.Int16x8.StorePart. This should
		// only store 4 elems into dst.
		// t := unsafe.Slice((*uint16)(unsafe.Pointer(&dst[0])), len(dst))
		// uints := saturated.ToBits()
		// ptr := (*uint64)(unsafe.Pointer(&t[0]))
		// *ptr = uints.ReshapeToUint64s().GetElem(0)
		// uints.StorePart(t)
	}

	// Scalar remainder. could use masked simd but prob not worth it.
	for ; i < numSamples; i++ {
		var sum int32
		offset := uintptr(i) * int16Size
		for j := range numFull {
			sample := (*int16)(unsafe.Add(full[j], offset))
			sum += int32(*(sample))
		}
		s.mixed[i] = softSaturatePade(sum, math.MaxInt16)
	}

	// ensure these are cleaned up
	for i := range numFull {
		full[i] = nil
	}
}

// softSaturatePadeNEON is a SIMD version of softSaturatePade.
//
// try passing in threshold as a float32
//
// CPU Feature: NEON
func softSaturatePadeNEON(acc archsimd.Int32x4, threshold archsimd.Float32x4) archsimd.Int16x8 {
	f := acc.ConvertToFloat32()

	// todo: could approx this with a precomputed reciprocal
	v := f.Div(threshold)
	approx := padeTanhNEON(v)

	// scale back to int16 range and saturate
	scaled := approx.Mul(threshold)
	ints := scaled.ConvertToInt32()
	return ints.SaturateToInt16()
}

// padeTanhNEON is a simd version of `padeTanhScalar`
//
// CPU Feature: NEON
func padeTanhNEON(x archsimd.Float32x4) archsimd.Float32x4 {
	var (
		vClampPos = archsimd.BroadcastFloat32x4(padeMaxInput)
		vClampNeg = archsimd.BroadcastFloat32x4(padeMinInput)
		vConst    = archsimd.BroadcastFloat32x4(15.)
		vCoeff    = archsimd.BroadcastFloat32x4(6.)
	)

	// clamp before divergence from tanh.
	x = x.Max(vClampNeg)
	x = x.Min(vClampPos)

	x2 := x.Mul(x)

	// numerator
	numer := x2.Add(vConst)
	numer = numer.Mul(x) // maybe do this after denom?

	// denominator
	denom := x2.MulAdd(vCoeff, vConst)

	// clamping to [-1,1] is not needed here because the caller will
	// always convert to int16 with signed saturation
	return numer.Div(denom)
}
