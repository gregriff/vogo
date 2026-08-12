package audio

import (
	"math"
	"simd/archsimd"
	"unsafe"
)

// stub for compilation on amd64
func (s *streams) mixNEON(int) {
	panic("not implemented")
}

func (s *streams) mixAVX(numSamples int, avx512 bool) {
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

	const int16Size = unsafe.Sizeof(int16(0))
	var i int
	if avx512 {
		i = s.doMixAVX512(i, numFull, numSamples, int16Size, full)
	} else {
		i = s.doMixAVX2(i, numFull, numSamples, int16Size, full)
	}

	// Scalar remainder.
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

func (s *streams) doMixAVX512(
	i, numFull, count int,
	int16Size uintptr,
	full [MaxStreams]unsafe.Pointer,
) int {
	const w = 16 // AVX512 int16 width
	vThreshold := archsimd.BroadcastFloat32x16(math.MaxInt16)

	// avoid bounds checks
	_ = full[numFull-1]  //nolint:gosec // G602: checked in streams.add
	_ = s.mixed[count-1] //nolint:gosec // G602: checked in streams.add

	for ; i+w <= count; i += w {
		acc := archsimd.BroadcastInt32x16(0)
		offset := uintptr(i) * int16Size
		for j := range numFull {
			samples := (*[w]int16)(unsafe.Add(full[j], offset))
			vSamples := archsimd.LoadInt16x16Array(samples)
			acc = acc.Add(vSamples.ExtendToInt32())
		}
		saturated := softSaturatePadeAVX512(acc, vThreshold)
		saturated.StoreArray((*[w]int16)(s.mixed[i:]))
	}
	return i
}

func (s *streams) doMixAVX2(
	i, numFull, count int,
	int16Size uintptr,
	full [MaxStreams]unsafe.Pointer,
) int {
	const w = 8 // AVX2 int16 width
	vThreshold := archsimd.BroadcastFloat32x8(math.MaxInt16)

	// avoid bounds checks
	_ = full[numFull-1]  //nolint:gosec // G602: checked in streams.add
	_ = s.mixed[count-1] //nolint:gosec // G602: checked in streams.add

	for ; i+w <= count; i += w {
		acc := archsimd.BroadcastInt32x8(0)
		offset := uintptr(i) * int16Size
		for j := range numFull {
			samples := (*[w]int16)(unsafe.Add(full[j], offset))
			vSamples := archsimd.LoadInt16x8Array(samples)
			acc = acc.Add(vSamples.ExtendToInt32())
		}
		saturated := softSaturatePadeAVX2(acc, vThreshold)
		saturated.StoreArray((*[w]int16)(s.mixed[i:]))
	}
	return i
}

// softSaturatePadeAVX512 is a SIMD version of softSaturatePade.
//
// try passing in threshold as a float32
//
// CPU Feature: AVX512
func softSaturatePadeAVX512(acc archsimd.Int32x16, threshold archsimd.Float32x16) archsimd.Int16x16 {
	f := acc.ConvertToFloat32()

	// todo: could approx this with a precomputed reciprocal
	v := f.Div(threshold)
	approx := padeTanhAVX512(v)

	// scale back to int16 range and saturate
	scaled := approx.Mul(threshold)
	ints := scaled.ConvertToInt32()
	return ints.SaturateToInt16()
}

// softSaturatePadeAVX2 is a SIMD version of softSaturatePade.
//
// try passing in threshold as a float32
//
// CPU Feature: AVX2
func softSaturatePadeAVX2(acc archsimd.Int32x8, threshold archsimd.Float32x8) archsimd.Int16x8 {
	f := acc.ConvertToFloat32()

	// todo: could approx this with a precomputed reciprocal
	v := f.Div(threshold)
	approx := padeTanhAVX2(v)

	// scale back to int16 range and saturate
	scaled := approx.Mul(threshold)
	ints := scaled.ConvertToInt32()
	return ints.SaturateToInt16()
}

// padeTanhAVX512 is a simd version of `padeTanhScalar`
//
// CPU Feature: AVX512
func padeTanhAVX512(x archsimd.Float32x16) archsimd.Float32x16 {
	// TODO: test not instantiating these until needed.
	var (
		vClampPos = archsimd.BroadcastFloat32x16(padeMaxInput)
		vClampNeg = archsimd.BroadcastFloat32x16(padeMinInput)
		vConst    = archsimd.BroadcastFloat32x16(15.)
		vCoeff    = archsimd.BroadcastFloat32x16(6.)
		// vExp   = archsimd.BroadcastFloat32x16(2.)
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

	// can try reciprocal approximation + one Newton-Raphson step instead of division
	// r0 := denom.Reciprocal()
	// t := denom.Mul(r0)
	// t = vExp.Sub(t) // (2 - denom*r0)
	// r1 := r0.Mul(t) //  refined reciprocal
	// y := numer.Mul(r1)

	// clamping to [-1,1] is not needed here because the caller will
	// always convert to int16 with signed saturation
	return numer.Div(denom)
}

// padeTanhAVX2 is a simd version of `padeTanhScalar`
//
// CPU Feature: AVX2
func padeTanhAVX2(x archsimd.Float32x8) archsimd.Float32x8 {
	var (
		vClampPos = archsimd.BroadcastFloat32x8(padeMaxInput)
		vClampNeg = archsimd.BroadcastFloat32x8(padeMinInput)
		vConst    = archsimd.BroadcastFloat32x8(15.)
		vCoeff    = archsimd.BroadcastFloat32x8(6.)
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
