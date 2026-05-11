package audio

import (
	"math"
	"simd/archsimd"
	"unsafe"
)

func (s *streams) mixAVX512(numSamples int) {
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
	if !archsimd.X86.AVX512() {
		return
	}
	const simdW = 16

	var i = 0
	const int16Size = unsafe.Sizeof(int16(0))
	for ; i+simdW <= numSamples; i += simdW {
		acc := archsimd.BroadcastInt32x16(0)
		for j := range numFull {
			// TODO: profile not using unsafe, and using dedicated slice/slicepart funcs.
			ptr := (*[simdW]int16)(unsafe.Add(full[j], uintptr(i)*int16Size))
			v32 := archsimd.LoadInt16x16(ptr).ExtendToInt32() // extending avoids overflow
			acc = acc.Add(v32)
		}
		acc.Store((*[simdW]int32)(summed[i:]))
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
			v32 := archsimd.LoadInt16x8(ptr).ExtendToInt32()
			acc = acc.Add(v32)
		}
		acc.Store((*[simdW]int32)(summed[i:]))
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
