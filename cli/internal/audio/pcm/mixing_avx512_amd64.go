//go:build amd64.v4

package pcm

import (
	"math"
	"simd/archsimd"
	"unsafe"
)

// mix mixes two or more pcm arrays with soft-saturation.
//
// CPU Feature: AVX512
func (s *Streams) mix(full [MaxStreams]unsafe.Pointer, numFull, numSamples int) {
	const int16Size = unsafe.Sizeof(int16(0))
	const w32 = 16 // AVX512 int32 width
	const w16 = w32 * 2
	vThreshold := archsimd.BroadcastFloat32x16(math.MaxInt16)

	// avoid bounds checks
	_ = full[numFull-1]       //nolint:gosec // G602: checked in streams.AddNew
	_ = s.mixed[numSamples-1] //nolint:gosec // G602: checked in streams.AddNew

	i := 0
	for ; i+w16 <= numSamples; i += w16 {
		accLo := archsimd.BroadcastInt32x16(0)
		accHi := archsimd.BroadcastInt32x16(0)
		offset := uintptr(i) * int16Size

		for j := range numFull {
			p := (*[w16]int16)(unsafe.Add(full[j], offset))
			v := archsimd.LoadInt16x32Array(p)
			accLo = accLo.Add(v.GetLo().ExtendToInt32())
			accHi = accHi.Add(v.GetHi().ExtendToInt32())
		}

		satLo := softSaturate(accLo, vThreshold)
		satHi := softSaturate(accHi, vThreshold)

		satLo.StoreArray((*[w32]int16)(s.mixed[i:]))
		satHi.StoreArray((*[w32]int16)(s.mixed[i+16:]))
	}

	// Scalar remainder. TODO: masked ops
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
// CPU Feature: AVX512
func softSaturate(acc archsimd.Int32x16, threshold archsimd.Float32x16) archsimd.Int16x16 {
	x := acc.ConvertToFloat32().Div(threshold)
	approx := padeTanh(x)

	// scale back to int16 range and saturate
	scaled := approx.Mul(threshold)
	return scaled.ConvertToInt32().SaturateToInt16()
}

// padeTanh applies a [3/2] Padé tanh approximant to float32s.
//
// CPU Feature: AVX512
func padeTanh(x archsimd.Float32x16) archsimd.Float32x16 {
	// clamp before divergence from tanh.
	x = x.Max(archsimd.BroadcastFloat32x16(padeMinInput))
	x = x.Min(archsimd.BroadcastFloat32x16(padeMaxInput))
	x2 := x.Mul(x)

	vConst := archsimd.BroadcastFloat32x16(padeConst)

	denom := x2.MulAdd(archsimd.BroadcastFloat32x16(padeCoeff), vConst)
	numer := x2.MulAdd(x, x.Mul(vConst)) // x^3 + 15x

	// clamping to [-1,1] is not needed here because the caller will
	// always convert to int16 with signed saturation
	return numer.Div(denom)
}
