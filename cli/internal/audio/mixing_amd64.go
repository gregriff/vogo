package audio

import (
	"math"
	"simd/archsimd"
	"unsafe"
)

func (s *streams) mixAVX512(numSamples int) {
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

	const simdW = 16
	const int16Size = unsafe.Sizeof(int16(0))
	scale := archsimd.BroadcastFloat32x16(math.MaxInt16)

	var i = 0
	var offset uintptr
	for ; i+simdW <= numSamples; i += simdW {
		acc := archsimd.BroadcastInt32x16(0)
		offset = uintptr(i) * int16Size
		for j := range numFull {
			ptr := (*[simdW]int16)(unsafe.Add(full[j], offset))
			v32 := archsimd.LoadInt16x16Array(ptr).ExtendToInt32() // extending avoids overflow
			acc = acc.Add(v32)
		}

		saturated := softSaturatePadeAVX512(acc, scale)
		saturated.StoreArray((*[simdW]int16)(s.mixed[i:]))
	}

	// Scalar remainder. could use masked simd but prob not worth it.
	for ; i < numSamples; i++ {
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

func (s *streams) mixAVX2(numSamples int) {
	// get pointers to bufs with at least [numSamples] samples
	full, numFull := [MaxStreams]unsafe.Pointer{}, int32(0)

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

	// maybe this is needed to force simd asm?
	if !archsimd.X86.AVX2() {
		return
	}
	const simdW = 8

	var i = 0
	const int16Size = unsafe.Sizeof(int16(0))
	for ; i+simdW <= numSamples; i += simdW {
		acc := archsimd.BroadcastInt32x8(0)
		for j := range numFull {
			ptr := (*[simdW]int16)(unsafe.Add(full[j], uintptr(i)*int16Size))
			v32 := archsimd.LoadInt16x8Array(ptr).ExtendToInt32()
			acc = acc.Add(v32)
		}
		acc.StoreArray((*[simdW]int32)(summed[i:]))
	}

	// Scalar remainder
	for ; i < numSamples; i++ {
		var sum int32
		for j := range numFull {
			sum += int32(*((*int16)(unsafe.Add(full[j], uintptr(i)*int16Size))))
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

// softSaturatePadeAVX512 is a SIMD version of softSaturatePade.
//
// CPU Feature: AVX-512
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

// padeTanhAVX512 applies a [3/2] Padé tanh approximant on 16 float32s
// using tanh(x) ≈ x*(x²+15) / (6x²+15). It is used during audio mixing,
// where x is the ratio of the sum of PCM samples to a clipping threshold.
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
//
// WRITE TESTS FOR THIS AGAINST math.tanh()
//
// CPU Feature: AVX-512
func padeTanhAVX512(x archsimd.Float32x16) archsimd.Float32x16 {
	// TODO: test not instantiating these until needed.
	var (
		// clamps
		vClampPos = archsimd.BroadcastFloat32x16(4.97)
		vClampNeg = archsimd.BroadcastFloat32x16(-4.97)
		vOne      = archsimd.BroadcastFloat32x16(1.)
		vNegOne   = archsimd.BroadcastFloat32x16(-1.)

		// terms
		vConst = archsimd.BroadcastFloat32x16(15.)
		vCoeff = archsimd.BroadcastFloat32x16(6.)
		// vExp   = archsimd.BroadcastFloat32x16(2.)
	)

	// clamp input first — approximant diverges outside |x| > ~4.97
	// consider using 3 or 4 as clamps.
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
	y := numer.Div(denom)

	// final safety clamp against residual approximation error
	y = y.Max(vNegOne)
	y = y.Min(vOne)

	return y
}
