package audio

import (
	"log"
	"slices"
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
	data [MaxStreams]*ringbuffer.RingBuffer

	// these are used during mixing to allow for iteration of pcm via pointer arithmetic.
	writeBufs [MaxStreams][pcmBufferSize]int16

	// this is where mixed pcm is written.
	mixed [pcmBufferSize]int16

	// ids[n] is the stream ID of data[n]. Used when someone leaves a room,
	// in order to delete the correct stream.
	ids [MaxStreams]string
}

func newStreams() streams {
	return streams{
		data:      [MaxStreams]*ringbuffer.RingBuffer{},
		writeBufs: [MaxStreams][pcmBufferSize]int16{},
		mixed:     [pcmBufferSize]int16{},
	}
}

// add adds a newly-created empty pcm buffer to the list of buffers (bufs) being tracked. It takes its pointer,
// so that the caller can continue modifying the original, and using this struct will always point to the same memory.
func (s *streams) add(id string, rb *ringbuffer.RingBuffer) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// see if a stream with id is already being tracked.
	if idx := slices.Index(s.ids[:], id); idx >= 0 {
		// this should not happen and tests should ensure that.
		log.Printf("WARN: stream with id: %s has already been added", id)
		if s.data[idx] == nil {
			log.Panicf("stream %s is tracked at streams.ids[%d], but streams.data[%d] is nil", id, idx, idx)
		}

		// this code shound never run.
		log.Printf("WARN: overwriting stream with id %s", id)
		// overwrite old rb, since it was tracked by the same stream id.
		s.data[idx] = rb
	}

	// add &rb to first empty slot
	for i := range len(s.data) {
		if s.data[i] == nil {
			if s.ids[i] != "" {
				log.Panicf("desync between s.ids and s.data. s.ids[%d]==%s but s.data[%d] is nil", i, id, i)
			}
			s.ids[i] = id
			s.data[i] = rb
			break
		}

		// vogo server and streams.remove should guard against this.
		if i == len(s.data)-1 {
			log.Panicf("cannot add stream %s. there are already %d streams (max)", id, MaxStreams)
		}
	}
}

// remove removes a stream. This should be called when a PeerConnection fails.
// It takes the user's name as the id.
func (s *streams) remove(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := slices.Index(s.ids[:], id)
	if idx == -1 {
		log.Printf("WARN: stream with id %s has already been removed", id)
		return
	}

	if s.data[idx] == nil {
		log.Panicf("desync between s.ids and s.data. s.ids[%d]==%s but s.data[%d] is nil", idx, id, idx)
	}

	s.ids[idx] = ""
	s.data[idx] = nil
	log.Printf("INFO: removed stream %s at index %d", id, idx)
}
