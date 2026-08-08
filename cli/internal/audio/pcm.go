package audio

import (
	"log"
	"sync"

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

	// data stores references to the buffers that each TrackRemote
	// writes data to from the network.
	data map[string]*ringbuffer.RingBuffer

	// these are used during mixing to allow for iteration of pcm via pointer arithmetic.
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
