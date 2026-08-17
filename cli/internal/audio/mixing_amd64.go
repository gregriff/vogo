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

// mixAVX is a SIMD implementation of [s.mix].
//
// CPU Feature: AVX2
func (s *streams) mixAVX(numSamples int, avx512 bool) {
	full, numFull, done := s.preMix(numSamples)
	if done {
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
		s.mixed[i] = softSaturate(sum, math.MaxInt16)
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
	const w32 = 16 // AVX512 int32 width
	const w16 = w32 * 2
	vThreshold := archsimd.BroadcastFloat32x16(math.MaxInt16)

	// avoid bounds checks
	_ = full[numFull-1]  //nolint:gosec // G602: checked in streams.add
	_ = s.mixed[count-1] //nolint:gosec // G602: checked in streams.add

	for ; i+w16 <= count; i += w16 {
		accLo := archsimd.BroadcastInt32x16(0)
		accHi := archsimd.BroadcastInt32x16(0)
		offset := uintptr(i) * int16Size

		for j := range numFull {
			p := (*[w16]int16)(unsafe.Add(full[j], offset))
			v := archsimd.LoadInt16x32Array(p)
			accLo = accLo.Add(v.GetLo().ExtendToInt32())
			accHi = accHi.Add(v.GetHi().ExtendToInt32())
		}

		satLo := softSaturateAVX512(accLo, vThreshold)
		satHi := softSaturateAVX512(accHi, vThreshold)
		satLo.StoreArray((*[w32]int16)(s.mixed[i:]))
		satHi.StoreArray((*[w32]int16)(s.mixed[i+16:]))
	}
	return i
}

func (s *streams) doMixAVX2(
	i, numFull, count int,
	int16Size uintptr,
	full [MaxStreams]unsafe.Pointer,
) int {
	const w32 = 8 // AVX2 int32 width
	const w16 = w32 * 2
	const increment = w16 * 2
	vThreshold := archsimd.BroadcastFloat32x8(math.MaxInt16)

	// avoid bounds checks
	_ = full[numFull-1]  //nolint:gosec // G602: checked in streams.add
	_ = s.mixed[count-1] //nolint:gosec // G602: checked in streams.add

	for ; i+increment <= count; i += increment {
		accLo := archsimd.BroadcastInt32x8(0)
		accHi := archsimd.BroadcastInt32x8(0)
		accLo2 := archsimd.BroadcastInt32x8(0)
		accHi2 := archsimd.BroadcastInt32x8(0)
		offset := uintptr(i) * int16Size
		offset2 := uintptr(i+8) * int16Size

		for j := range numFull {
			p := (*[w16]int16)(unsafe.Add(full[j], offset))
			p2 := (*[w16]int16)(unsafe.Add(full[j], offset2))
			v := archsimd.LoadInt16x16Array(p)
			v2 := archsimd.LoadInt16x16Array(p2)

			accLo = accLo.Add(v.GetLo().ExtendToInt32())
			accHi = accHi.Add(v.GetHi().ExtendToInt32())
			accLo2 = accLo2.Add(v2.GetLo().ExtendToInt32())
			accHi2 = accHi2.Add(v2.GetHi().ExtendToInt32())
		}

		// double pump soft saturate and do the saturation in there
		scaledLo := padeNormalizeAVX2(accLo, vThreshold)
		scaledHi := padeNormalizeAVX2(accHi, vThreshold)
		satLo := scaledLo.ConvertToInt32().SaturateToInt16()
		satHi := scaledHi.ConvertToInt32().SaturateToInt16()

		scaledLo2 := padeNormalizeAVX2(accLo2, vThreshold)
		scaledHi2 := padeNormalizeAVX2(accHi2, vThreshold)
		satLo2 := scaledLo2.ConvertToInt32().SaturateToInt16()
		satHi2 := scaledHi2.ConvertToInt32().SaturateToInt16()

		satLo.StoreArray((*[w32]int16)(s.mixed[i:]))
		satHi.StoreArray((*[w32]int16)(s.mixed[i+8:]))
		satLo2.StoreArray((*[w32]int16)(s.mixed[i+16:]))
		satHi2.StoreArray((*[w32]int16)(s.mixed[i+24:]))
	}
	return i
}

// softSaturateAVX512 is a SIMD version of softSaturate.
//
// CPU Feature: AVX512
func softSaturateAVX512(acc archsimd.Int32x16, threshold archsimd.Float32x16) archsimd.Int16x16 {
	f := acc.ConvertToFloat32()

	// todo: could approx this with a precomputed reciprocal
	v := f.Div(threshold)
	approx := padeTanhAVX512(v)

	// scale back to int16 range and saturate
	scaled := approx.Mul(threshold)
	ints := scaled.ConvertToInt32()
	return ints.SaturateToInt16()
}

// padeNormalizeAVX2 normalizes x to threshold using a Pade approximation of tanh.
//
// CPU Feature: AVX2
func padeNormalizeAVX2(x archsimd.Int32x8, threshold archsimd.Float32x8) archsimd.Float32x8 {
	f := x.ConvertToFloat32()

	// todo: could approx this with a precomputed reciprocal
	v := f.Div(threshold)
	approx := padeTanhAVX2(v)

	// scale back to int16 range
	return approx.Mul(threshold)
}

// padeTanhAVX512 is a simd version of `padeTanhScalar`
//
// CPU Feature: AVX512
func padeTanhAVX512(x archsimd.Float32x16) archsimd.Float32x16 {
	// clamp before divergence from tanh.
	x = x.Max(archsimd.BroadcastFloat32x16(padeMinInput))
	x = x.Min(archsimd.BroadcastFloat32x16(padeMaxInput))
	x2 := x.Mul(x)

	vConst := archsimd.BroadcastFloat32x16(15.)

	denom := x2.MulAdd(archsimd.BroadcastFloat32x16(6.), vConst)
	numer := x2.MulAdd(x, x.Mul(vConst)) // x^3 + 15x

	// clamping to [-1,1] is not needed here because the caller will
	// always convert to int16 with signed saturation
	return numer.Div(denom)
}

// padeTanhAVX2 is a simd version of `padeTanhScalar`
//
// CPU Feature: AVX2
func padeTanhAVX2(x archsimd.Float32x8) archsimd.Float32x8 {
	// clamp before divergence from tanh.
	x = x.Max(archsimd.BroadcastFloat32x8(padeMinInput))
	x = x.Min(archsimd.BroadcastFloat32x8(padeMaxInput))
	x2 := x.Mul(x)

	vConst := archsimd.BroadcastFloat32x8(15.)

	denom := x2.MulAdd(archsimd.BroadcastFloat32x8(6.), vConst)
	numer := x2.MulAdd(x, x.Mul(vConst)) // x^3 + 15x

	// clamping to [-1,1] is not needed here because the caller will
	// always convert to int16 with signed saturation
	return numer.Div(denom)
}
