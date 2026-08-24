//go:build !amd64.v4

package pcm

import (
	"math"
	"simd/archsimd"
	"unsafe"
)

// mix mixes two or more pcm arrays with soft-saturation.
//
// CPU Feature: AVX2
func (s *Streams) mix(full [MaxStreams]unsafe.Pointer, numFull, numSamples int) {
	const int16Size = unsafe.Sizeof(int16(0))
	const w32 = 8 // AVX2 int32 width
	const w16 = w32 * 2
	const increment = w16 * 2 // unroll factor of 2
	vThreshold := archsimd.BroadcastFloat32x8(math.MaxInt16)

	// avoid bounds checks
	_ = full[numFull-1]       //nolint:gosec // G602: checked in streams.AddNew
	_ = s.mixed[numSamples-1] //nolint:gosec // G602: checked in streams.AddNew

	i := 0
	for ; i+increment <= numSamples; i += increment {
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

		satLo := softSaturate(accLo, vThreshold)
		satHi := softSaturate(accHi, vThreshold)
		satLo2 := softSaturate(accLo2, vThreshold)
		satHi2 := softSaturate(accHi2, vThreshold)

		satLo.StoreArray((*[w32]int16)(s.mixed[i:]))
		satHi.StoreArray((*[w32]int16)(s.mixed[i+8:]))
		satLo2.StoreArray((*[w32]int16)(s.mixed[i+16:]))
		satHi2.StoreArray((*[w32]int16)(s.mixed[i+24:]))
	}

	// Scalar remainder.
	for ; i < numSamples; i++ {
		var sum int32
		offset := uintptr(i) * int16Size
		for j := range numFull {
			sample := (*int16)(unsafe.Add(full[j], offset))
			sum += int32(*(sample))
		}
		s.mixed[i] = softSaturateScalar(sum, math.MaxInt16)
	}
}

// softSaturate saturates sums to a threshold.
//
// CPU Feature: AVX2
func softSaturate(acc archsimd.Int32x8, threshold archsimd.Float32x8) archsimd.Int16x8 {
	x := acc.ConvertToFloat32().Div(threshold)
	approx := padeTanh(x)

	// scale back to int16 range and saturate
	scaled := approx.Mul(threshold)
	return scaled.ConvertToInt32().SaturateToInt16()
}

// padeTanh applies a [3/2] Padé tanh approximant to float32s.
//
// CPU Feature: AVX2
func padeTanh(x archsimd.Float32x8) archsimd.Float32x8 {
	// clamp before divergence from tanh.
	x = x.Max(archsimd.BroadcastFloat32x8(padeMinInput))
	x = x.Min(archsimd.BroadcastFloat32x8(padeMaxInput))
	x2 := x.Mul(x)

	vConst := archsimd.BroadcastFloat32x8(padeConst)

	denom := x2.MulAdd(archsimd.BroadcastFloat32x8(padeCoeff), vConst)
	numer := x2.MulAdd(x, x.Mul(vConst)) // x^3 + 15x

	// clamping to [-1,1] is not needed here because the caller will
	// always convert to int16 with signed saturation
	return numer.Div(denom)
}
