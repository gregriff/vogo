package audio

import (
	"log"
	"math"
	"sync"
	"unsafe"

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
type streams struct {
	mu    sync.Mutex
	bufs  map[string]*[]int16
	mixed [pcmBufferSize]int16
}

func newStreams() streams {
	return streams{
		bufs:  make(map[string]*[]int16, maxStreams),
		mixed: [pcmBufferSize]int16{},
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
	if len(s.bufs) > maxStreams {
		log.Panicf("len(streams.bufs): %d, > maxStreams", len(s.bufs))
	}
	s.mu.Unlock()
}

func (s *streams) remove(id string) {
	s.mu.Lock()
	delete(s.bufs, id)
	s.mu.Unlock()
	log.Printf("INFO: removed %s's stream", id)
}

// mix takes all [s.bufs] that have at least [numSamples] samples and mixes their pcm data, writing the result to
// [s.mixed]. It must be run within a mutex lock. If [full] is empty due to network conditions,
// or [s.bufs] is empty due to none being added, the caller can still write [s.mixed]
// to the speaker because it is zeroed, and the speaker will play silence.
// Assumes numSamples <= cap(s.mixed) and len(s.bufs) <= maxStreams
func (s *streams) mix(numSamples int) {
	// get pointers to bufs with at least [numSamples] samples
	full, numFull := [maxStreams]unsafe.Pointer{}, int32(0)
	for _, buf := range s.bufs {
		if len(*buf) >= numSamples {
			full[numFull] = unsafe.Pointer(&(*buf))
			numFull++
		}
	}

	// ensure previous mixed pcm is erased
	for i := range s.mixed {
		s.mixed[i] = 0
	}
	if numFull == 0 || len(s.bufs) == 0 {
		return
	}

	// if only one other person in the room, don't mix, just write their pcm
	if numFull == 1 {
		src := *(*[]int16)(full[0])
		// avoid bounds checks
		_ = s.mixed[numSamples-1]
		_ = src[numSamples-1]

		for i := range numSamples {
			s.mixed[i] = src[i]
		}
		// remove samples from stream that was written.
		*(*[]int16)(full[0]) = src[numSamples:]
		return
	}

	// avoid bounds checks
	_ = s.mixed[numSamples-1]
	_ = full[numFull-1]

	// mix full pcm bufs and write to s.mixed
	// NOTE: for SIMD, it's prob better to create an array of all the sums,
	// then do the mixing on those sums after arr is full.
	const zero = int32(0)
	var sum int32
	for i := range numSamples {
		sum = zero
		for j := range numFull { // TODO: use SIMD
			// use unsafe ptr arithmetic for no bounds checks, preparing for SIMD.
			sum += int32(*((*int16)(unsafe.Add(full[j], uintptr(i)*unsafe.Sizeof(int16(0))))))
		}
		// s.mixed[i] = clampInt16(sum / numFull)
		s.mixed[i] = softSaturate(sum, math.MaxInt16)
	}

	// remove samples from streams that were just mixed.
	for i := range numFull {
		*(*[]int16)(full[i]) = (*(*[]int16)(full[i]))[numSamples:]
	}
}

// softSaturate takes a summed int32 value and a threshold,
// returns a soft-saturated int16 using tanh.
func softSaturate(sum int32, threshold float64) int16 {
	saturated := math.Tanh(float64(sum)/threshold) * threshold
	return clampInt16(saturated)
}

type MixedPCMSample interface {
	int32 | float64
}

func clampInt16[S MixedPCMSample](val S) int16 {
	const (
		Min = math.MinInt16
		Max = math.MaxInt16
	)
	if val < Min {
		return Min
	}
	if val > Max {
		return Max
	}
	return int16(val)
}
