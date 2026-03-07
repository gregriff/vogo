package audio

import (
	"log"
	"math"
	"sync"
)

// This file contains structs that store PCM data.

// Note: this would have to be doubled if you want to allow users to send text also.
const maxStreams = 5 // max remote tracks (max users - 1)

// stream stores PCM audio data. The microphone writes PCM to its stream, where
// it is then encoded into Opus and written to a webrtc Track, and in a 1:1 voice call
// the speaker writes decoded Opus to its stream, where it is then written to the malgo device.
type stream struct {
	mu  sync.Mutex
	buf []int16
}

// streams is a shared set of PCM buffers that is written to from the network by each remoteTrack
// and read by malgo for playback. It is used for channel calls, storing the incoming
// audio data from the other users.
// TODO: experiment with storing fixed arrays per stream (5), and a []*[]int of overrun slices for the streams,
// for the worst-case when bursty packet loss happens and we recv more samples than expected from the network.
// this may allow mixing to be faster since in most cases all streams could be prefetched and next to eachother
// in cache lines. this would require returning the pointer to the arrays in addStream and bounds checking
// each copy of decoded PCM.
type streams struct {
	mu   sync.Mutex
	bufs []*[]int16
}

func newStreams() *streams {
	return &streams{
		bufs: make([]*[]int16, 0, maxStreams),
	}
}

// add adds a newly-created empty pcm buffer to the list of buffers (bufs) being tracked. It takes its pointer,
// so that the caller can continue modifying the original, and using this struct will always point to the same memory.
func (s *streams) add(b *[]int16) {
	log.Println("ADDING NEW STREAM!")
	s.mu.Lock()
	s.bufs = append(s.bufs, b)
	s.mu.Unlock()
}

// hasFullSample iterates through all the audio buffers, and if any have at least `amt` samples (elements),
// returns true. This is used to short-circuit the speaker callback if no data has been received over the wire.
// This function also returns an array of pointers to each buffers containing a full sample to be mixed.
// NOTES:
// - if returns false, speaker will not play sound for this frame
// - may need to tweak this logic. hopefully this does not cause loss of audio data
// - this may be unnessicary at a certain point, but it may also prime the cache with all the PCM before
// the mixing happens, so profile to see.
// ex: if 3 full buffers, returns {[ptr, ptr, ptr], (cap=len(ab.bufs)), true}
func (s *streams) hasFullSample(amt int) (fullBufs []*[]int16, ok bool) {
	// fullBufs = make([]*[]int16, len(ab.bufs))
	fullBufs = make([]*[]int16, 0, len(s.bufs))
	for i := range len(s.bufs) {
		if len(*s.bufs[i]) >= amt {
			ok = true
			fullBufs = append(fullBufs, s.bufs[i])
			// fullBufs[i] = ab.bufs[i]
		}
	}
	return
}

// mix returns a pcm slice containing mixed samples from pcm slices all containing at least
// numSamples elements. This is enforced by the fullBufs param, which may only contain pointers
// to such slices. fullBufs needs to be created by s.hasFullSample, and both of these functions
// must execute within the same mutex lock.
func (s *streams) mix(fullBufs []*[]int16, numSamples int) []int16 {
	if len(s.bufs) == 0 {
		return nil
	}

	// note: if you impl the overrun slices, you'd check for that here
	// and exec a slow path if any stream has one

	const zero = int32(0)
	var (
		mixed   = make([]int16, numSamples)
		numFull = int32(len(fullBufs))
		sum     int32
	)
	for i := range len(mixed) {
		sum = zero
		for _, p := range fullBufs { // TODO: use SIMD in Go 1.26
			sum += int32((*p)[i])
		}
		mixed[i] = clampInt16(sum / numFull)
	}
	return mixed
}

// note: hopefully these untyped consts cast to both types they're compared to...
func clampInt16(val int32) int16 {
	const (
		min = math.MinInt16
		max = math.MaxInt16
	)
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return int16(val)
}
