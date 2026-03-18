package audio

import (
	"log"
	"math"
	"sync"
	"unsafe"

	"github.com/gregriff/vogo/cli/internal/audio/ringbuffer"
	"github.com/gregriff/vogo/shared"
)

// This file contains structs that store PCM data.

// Note: this would have to be doubled if you want to allow users to send text also.
const MaxStreams = shared.ChannelCapacity - 1

// allocating more size to rb prevents overwriting samples after bursty packet arrival.
const ringBufferSize = pcmBufferSize * 6

// stream stores PCM audio data. The microphone writes PCM to its stream, where
// it is then encoded into Opus and written to a webrtc Track, and in a 1:1 voice call
// the speaker writes decoded Opus to its stream, where it is then written to the malgo device.
type stream struct {
	mu sync.Mutex
	rb ringbuffer.RingBuffer
}

func newStream() stream {
	return stream{
		rb: ringbuffer.New(ringBufferSize),
	}
}

// streams is a shared set of PCM buffers that is written to from the network by each remoteTrack
// and read by malgo for playback. It is used for channel calls, storing the incoming
// audio data from the other users.
type streams struct {
	mu sync.Mutex

	// data stores references to the ringbuffers that each TrackRemote writes data to from the network.
	data map[string]*ringbuffer.RingBuffer

	// these are used during mixing to allow for easier vectorized iteration of pcm.
	writeBufs [MaxStreams][pcmBufferSize]int16

	// this is where mixed pcm is written.
	mixed [pcmBufferSize]int16
}

func newStreams() streams {
	return streams{
		data:      make(map[string]*ringbuffer.RingBuffer, MaxStreams),
		writeBufs: [MaxStreams][pcmBufferSize]int16{},
		mixed:     [pcmBufferSize]int16{},
	}
}

// add adds a newly-created empty pcm buffer to the list of buffers (bufs) being tracked. It takes its pointer,
// so that the caller can continue modifying the original, and using this struct will always point to the same memory.
func (s *streams) add(id string, rb *ringbuffer.RingBuffer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data[id]; ok {
		log.Printf("WARN stream with id: %s has already been added", id)
	}
	if len(s.data) == MaxStreams {
		log.Printf("WARN there are %d streams", MaxStreams)
	}
	s.data[id] = rb
	if len(s.data) > MaxStreams {
		log.Panicf("len(streams.bufs): %d, > maxStreams", len(s.data))
	}
}

// remove removes a stream. This should be called when a PeerConnection fails.
// It takes the user's name as the id.
func (s *streams) remove(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, id)
	log.Printf("INFO: removed %s's stream", id)
}

// mix takes all [s.bufs] that have at least [numSamples] samples and mixes their pcm data, writing the result to
// [s.mixed]. It must be run within a mutex lock. If [full] is empty due to network conditions,
// or [s.bufs] is empty due to none being added, the caller can still write [s.mixed]
// to the speaker because it is zeroed, and the speaker will play silence.
// Assumes numSamples <= cap(s.mixed) and len(s.bufs) <= maxStreams.
func (s *streams) mix(numSamples int) {
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

	// sum samples for each buffer
	// TODO: SIMD
	const zero = int32(0)
	const int16Size = unsafe.Sizeof(int16(0))
	var sum int32
	var offset uintptr
	for i := range numSamples {
		sum = zero
		offset = uintptr(i) * int16Size
		for j := range numFull {
			// use ptr arithmetic for no bounds checks for branchless SIMD.
			sum += int32(*((*int16)(unsafe.Add(full[j], offset))))
		}
		summed[i] = sum
	}

	// actual mixing
	_ = s.mixed[numSamples-1]
	_ = summed[numSamples-1] //nolint:gosec // G602: checked in caller
	for i := range numSamples {
		// s.mixed[i] = clampInt16(sum / numFull)
		s.mixed[i] = softSaturate(summed[i], math.MaxInt16)
	}

	// ensure these are cleaned up
	for i := range numFull {
		full[i] = nil
	}
}

// softSaturate takes a summed int32 value and a threshold,
// returns a soft-saturated int16 using tanh.
func softSaturate(sum int32, threshold float64) int16 {
	// note: could prob reimpl math.tanh with simd.
	saturated := math.Tanh(float64(sum)/threshold) * threshold
	return clampInt16(saturated)
}

type summedPCMSample interface {
	int32 | float64
}

func clampInt16[S summedPCMSample](val S) int16 {
	return int16(min(max(val, math.MinInt16), math.MaxInt16))
}
