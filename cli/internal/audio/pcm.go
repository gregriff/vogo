package audio

import (
	"log"
	"math"
	"sync"

	"github.com/gregriff/vogo/shared"
)

// This file contains structs that store PCM data.

// Note: this would have to be doubled if you want to allow users to send text also.
const maxStreams = shared.ChannelCapacity - 1

// stream stores PCM audio data. The microphone writes PCM to its stream, where
// it is then encoded into Opus and written to a webrtc Track, and in a 1:1 voice call
// the speaker writes decoded Opus to its stream, where it is then written to the malgo device.
type stream struct {
	mu  sync.Mutex
	buf []int16
}

func newStream() stream {
	return stream{
		buf: make([]int16, 0, pcmBufferSize),
	}
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
	bufs map[string]*[]int16
}

func newStreams() streams {
	return streams{
		bufs: make(map[string]*[]int16, maxStreams),
	}
}

// add adds a newly-created empty pcm buffer to the list of buffers (bufs) being tracked. It takes its pointer,
// so that the caller can continue modifying the original, and using this struct will always point to the same memory.
func (s *streams) add(id string, b *[]int16) {
	s.mu.Lock()
	if _, ok := s.bufs[id]; ok {
		log.Printf("WARN stream with id: %s has already been added", id)
	}
	if len(s.bufs) == maxStreams {
		log.Printf("WARN there are %d streams", maxStreams)
	}
	s.bufs[id] = b
	s.mu.Unlock()
}

func (s *streams) remove(id string) {
	s.mu.Lock()
	delete(s.bufs, id)
	s.mu.Unlock()
}

// fullBufs iterates through all the audio buffers, and if any have at least [amt] samples (elements),
// returns a slice of pointers to those buffers, and a true boolean, which is used to short-circuit
// the speaker callback if no data has been received over the wire.
// NOTES:
// - if returns false, speaker will not play sound for this frame
// - may need to tweak this logic. hopefully this does not cause loss of audio data
// - this may be unnessicary at a certain point, but it may also prime the cache with all the PCM before
// the mixing happens, so profile to see.
// ex: if 3 full buffers, returns {[ptr, ptr, ptr] (cap=len(s.bufs)), true}
func (s *streams) fullBufs(amt int) (full []*[]int16, ok bool) {
	full = make([]*[]int16, 0, len(s.bufs))
	for key := range s.bufs {
		if len(*s.bufs[key]) >= amt {
			ok = true
			full = append(full, s.bufs[key])
		}
	}
	return
}

// mix returns a slice of pcm data containing mixed samples from [bufs]. All slices pointed to
// by [bufs] must all contain at least [numSamples] elements. This is guaranteed if [bufs] is created
// by s.fullBufs. Both s.fullBufs and s.mix must execute within the same mutex lock.
func (s *streams) mix(bufs []*[]int16, numSamples int) []int16 {
	if len(s.bufs) == 0 {
		return nil
	}

	// note: if you impl the overrun slices, you'd check for that here
	// and exec a slow path if any stream has one

	const zero = int32(0)
	var (
		mixed   = make([]int16, numSamples)
		numFull = int32(len(bufs))
		sum     int32
	)
	for i := range len(mixed) {
		sum = zero
		for _, p := range bufs { // TODO: use SIMD in Go 1.26
			sum += int32((*p)[i])
		}
		mixed[i] = clampInt16(sum / numFull)
	}
	return mixed
}

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
