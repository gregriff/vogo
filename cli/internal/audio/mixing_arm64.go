package audio

import (
	"math"
	"simd/archsimd"
	"unsafe"
)

// stub for compilation on arm64
func (s *streams) mixAVX(int, bool) {
	panic("not implemented")
}

// mixNEON is a SIMD implementation of [s.mix].
//
// CPU Feature: NEON
func (s *streams) mixNEON(numSamples int) {
	full, numFull, done := s.preMix(numSamples)
	if done {
		return
	}

	// avoid bounds checks
	_ = full[numFull-1]       //nolint:gosec // G602: checked in streams.add
	_ = s.mixed[numSamples-1] //nolint:gosec // G602: checked in streams.add

	const w16 = 8             // NEON int16 width
	const increment = w16 * 4 // unroll factor of 4
	const int16Size = unsafe.Sizeof(int16(0))
	vThreshold := archsimd.BroadcastFloat32x4(math.MaxInt16)

	// Loop is unrolled to improve ILP.
	var i = 0
	for ; i+increment <= numSamples; i += increment {
		// Accumulators. NEON has 8 lanes of int16, but we are doing
		// operations in int32, so we need to accumulate with 4 lanes.
		// But since we can pull out 8 int16s at a time, by using two
		// accumulators and duplicating the math logic, we can double-pump
		// the operations and iterate over the source arrays half as often.
		accLo := archsimd.BroadcastInt32x4(0)
		accHi := archsimd.BroadcastInt32x4(0)
		accLo2 := archsimd.BroadcastInt32x4(0)
		accHi2 := archsimd.BroadcastInt32x4(0)
		accLo3 := archsimd.BroadcastInt32x4(0)
		accHi3 := archsimd.BroadcastInt32x4(0)
		accLo4 := archsimd.BroadcastInt32x4(0)
		accHi4 := archsimd.BroadcastInt32x4(0)
		offset := uintptr(i) * int16Size
		offset2 := uintptr(i+8) * int16Size
		offset3 := uintptr(i+16) * int16Size
		offset4 := uintptr(i+24) * int16Size

		// for each pcm array, grab 24 samples and add to the accumulators.
		for j := range numFull {
			p := (*[w16]int16)(unsafe.Add(full[j], offset))
			v := archsimd.LoadInt16x8Array(p)
			accLo = accLo.Add(v.ExtendLo4ToInt32())
			accHi = accHi.Add(v.HiToLo().ExtendLo4ToInt32())

			p2 := (*[w16]int16)(unsafe.Add(full[j], offset2))
			v2 := archsimd.LoadInt16x8Array(p2)
			accLo2 = accLo2.Add(v2.ExtendLo4ToInt32())
			accHi2 = accHi2.Add(v2.HiToLo().ExtendLo4ToInt32())

			p3 := (*[w16]int16)(unsafe.Add(full[j], offset3))
			v3 := archsimd.LoadInt16x8Array(p3)
			accLo3 = accLo3.Add(v3.ExtendLo4ToInt32())
			accHi3 = accHi3.Add(v3.HiToLo().ExtendLo4ToInt32())

			p4 := (*[w16]int16)(unsafe.Add(full[j], offset4))
			v4 := archsimd.LoadInt16x8Array(p4)
			accLo4 = accLo4.Add(v4.ExtendLo4ToInt32())
			accHi4 = accHi4.Add(v4.HiToLo().ExtendLo4ToInt32())
		}

		// After saturating, pack the two result vectors by
		// reinterpreting as int64 and zipping.
		// Ops are ordered as to improve ILP.
		satHi := softSaturateNEON(accHi, vThreshold)
		hi := satHi.ToBits().ReshapeToUint64s().BitsToInt64()
		satLo := softSaturateNEON(accLo, vThreshold)
		lo := satLo.ToBits().ReshapeToUint64s().BitsToInt64()
		packed := lo.InterleaveLo(hi) // [lo, hi]
		mixed := packed.ToBits().ReshapeToUint16s().BitsToInt16()
		mixed.StoreArray((*[w16]int16)(s.mixed[i:]))

		satHi2 := softSaturateNEON(accHi2, vThreshold)
		hi2 := satHi2.ToBits().ReshapeToUint64s().BitsToInt64()
		satLo2 := softSaturateNEON(accLo2, vThreshold)
		lo2 := satLo2.ToBits().ReshapeToUint64s().BitsToInt64()
		packed2 := lo2.InterleaveLo(hi2)
		mixed2 := packed2.ToBits().ReshapeToUint16s().BitsToInt16()
		mixed2.StoreArray((*[w16]int16)(s.mixed[i+8:]))

		satHi3 := softSaturateNEON(accHi3, vThreshold)
		hi3 := satHi3.ToBits().ReshapeToUint64s().BitsToInt64()
		satLo3 := softSaturateNEON(accLo3, vThreshold)
		lo3 := satLo3.ToBits().ReshapeToUint64s().BitsToInt64()
		packed3 := lo3.InterleaveLo(hi3)
		mixed3 := packed3.ToBits().ReshapeToUint16s().BitsToInt16()
		mixed3.StoreArray((*[w16]int16)(s.mixed[i+16:]))

		satHi4 := softSaturateNEON(accHi4, vThreshold)
		hi4 := satHi4.ToBits().ReshapeToUint64s().BitsToInt64()
		satLo4 := softSaturateNEON(accLo4, vThreshold)
		lo4 := satLo4.ToBits().ReshapeToUint64s().BitsToInt64()
		packed4 := lo4.InterleaveLo(hi4)
		mixed4 := packed4.ToBits().ReshapeToUint16s().BitsToInt16()
		mixed4.StoreArray((*[w16]int16)(s.mixed[i+24:]))
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

// softSaturateNEON is a SIMD version of softSaturate.
//
// CPU Feature: NEON
func softSaturateNEON(acc archsimd.Int32x4, threshold archsimd.Float32x4) archsimd.Int16x8 {
	x := acc.ConvertToFloat32().Div(threshold)
	approx := padeTanhNEON(x)

	// scale back to int16 range and saturate
	scaled := approx.Mul(threshold)
	ints := scaled.ConvertToInt32()
	return ints.SaturateToInt16()
}

// padeTanhNEON is a simd version of `padeTanhScalar`
//
// CPU Feature: NEON
func padeTanhNEON(x archsimd.Float32x4) archsimd.Float32x4 {
	// clamp before divergence from tanh.
	x = x.Max(archsimd.BroadcastFloat32x4(padeMinInput))
	x = x.Min(archsimd.BroadcastFloat32x4(padeMaxInput))
	x2 := x.Mul(x)

	vConst := archsimd.BroadcastFloat32x4(15.)

	numer := x2.MulAdd(x, x.Mul(vConst)) // x^3 + 15x
	denom := x2.MulAdd(archsimd.BroadcastFloat32x4(6.), vConst)

	// clamping to [-1,1] is not needed here because the caller will
	// always convert to int16 with signed saturation
	return numer.Div(denom)
}
