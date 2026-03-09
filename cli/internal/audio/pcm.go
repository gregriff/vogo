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
type streams struct {
	mu    sync.Mutex
	bufs  map[string]*[]int16
	mixed []int16
}

func newStreams() streams {
	return streams{
		bufs:  make(map[string]*[]int16, maxStreams),
		mixed: make([]int16, pcmBufferSize),
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
	log.Printf("INFO: removed %s's stream", id)
}

// fullBufs iterates through all the audio buffers, and if any have at least [amt] samples (elements),
// returns a slice of pointers to those buffers.
// ex: if 3 full buffers, returns []*int16{ptr, ptr, ptr} (cap=len(s.bufs))
func (s *streams) fullBufs(amt int) []*[]int16 {
	full := make([]*[]int16, 0, len(s.bufs))
	for key := range s.bufs {
		if len(*s.bufs[key]) >= amt {
			full = append(full, s.bufs[key])
		}
	}
	return full
}

// mix takes pointers to a slice of pcm buffers, each with at least [numSamples] samples
// and mixes their audio data, writing the result to s.mixed. [bufs] is created using s.fullBufs,
// and both s.fullBufs and s.mix must execute within the same mutex lock. If [full] is empty due to
// network conditions, or [s.bufs] is empty due to none being added, the caller can still write [s.mixed]
// to the speaker because it is zeroed, and the speaker will play silence.
// NOTE: this assumes that numSamples <= cap(s.mixed).
func (s *streams) mix(full []*[]int16, numSamples int) {
	clear(s.mixed)

	var numFull = int32(len(full))
	if len(s.bufs) == 0 || numFull == 0 {
		return
	}

	// TODO: write slow path func that runs if numSamples > cap(s.mixed), using append()

	// note: if you impl the overrun slices, you'd check for that here
	// and exec a slow path if any stream has one

	const zero = int32(0)
	var sum int32
	for i := range numSamples {
		sum = zero
		for _, p := range full { // TODO: use SIMD in Go 1.26
			sum += int32((*p)[i])
		}
		s.mixed[i] = clampInt16(sum / numFull)
	}
}

func clampInt16(val int32) int16 {
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
