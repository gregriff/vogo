package pcm

import (
	"fmt"
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
const ringBufferSize = BufferSize * 6

// Stream stores PCM audio data. The microphone writes PCM to its Stream, where
// it is then encoded into Opus and written to a webrtc Track, and in a 1:1 voice call
// the speaker writes decoded Opus to its Stream, where it is then written to the malgo device.
type Stream struct {
	mu sync.Mutex
	rb ringbuffer.RingBuffer
}

func NewStream() Stream {
	return Stream{
		rb: ringbuffer.New(ringBufferSize),
	}
}

// Read reads at least count pcm samples into dst,
// if the internal ringbuffer has count samples available.
func (s *Stream) Read(dst []int16, count int) int {
	s.mu.Lock()

	// Need at least n samples
	if s.rb.Len() < count {
		s.mu.Unlock()
		return 0
	}

	// Extract count samples and remove it from the buffer
	n := s.rb.Read(dst) // overwrites whatever's in there
	s.mu.Unlock()
	return n
}

// ReadFrame reads up to len(dst) pcm samples into dst. Does not read any if there
// are < FrameSize samples in the stream's ringbuffer, so the caller can wait for more.
func (s *Stream) ReadFrame(dst []int16) int {
	s.mu.Lock()

	// Need at least one frame worth of data
	if s.rb.Len() < FrameSize {
		s.mu.Unlock()
		return 0
	}

	// Extract one frame and remove it from the buffer
	n := s.rb.Read(dst) // overwrites whatever's in there
	s.mu.Unlock()
	return n
}

// WriteFrame writes a frame of pcm data from src into the internal ringbuffer.
func (s *Stream) WriteFrame(src []int16) {
	s.mu.Lock()
	s.rb.Write(src)
	s.mu.Unlock()
}

// Streams is a shared set of PCM buffers that is written to from the network by each remoteTrack
// and read by malgo for playback. It is used for channel calls, storing the incoming
// audio data from the other users.
type Streams struct {
	mu sync.Mutex

	// ids[n] is the stream ID of data[n]. Used to access the ringbuffers.
	ids [MaxStreams]string

	// data stores references to the buffers that each TrackRemote
	// writes data to from the network.
	data [MaxStreams]*ringbuffer.RingBuffer

	// these are used during mixing to allow for iteration of pcm via pointer arithmetic.
	writeBufs [MaxStreams][BufferSize]int16

	// this is where mixed pcm is written.
	mixed [BufferSize]int16
}

func NewStreams() Streams {
	return Streams{
		data:      [MaxStreams]*ringbuffer.RingBuffer{},
		writeBufs: [MaxStreams][BufferSize]int16{},
		mixed:     [BufferSize]int16{},
	}
}

// WriteFrame writes a PCM frame from src into the ringbuffer with id.
// If this returns an error, the caller used s.AddNew or s.Remove incorrectly.
func (s *Streams) WriteFrame(id string, src []int16) error {
	s.mu.Lock()

	idx := slices.Index(s.ids[:], id)
	if idx == -1 {
		return fmt.Errorf("stream with id %s not found", id)
	}

	if s.data[idx] == nil {
		return fmt.Errorf("stream with id %s is nil", id)
	}

	s.data[idx].Write(src)
	s.mu.Unlock()
	return nil
}

// AddNew creates an empty pcm ringbuffer and adds it to the list of
// buffers being tracked. It returns its pointer so the caller can
// continue to access it without doing lookups. id must be unique.
func (s *Streams) AddNew(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// see if a stream with id is already being tracked.
	if idx := slices.Index(s.ids[:], id); idx >= 0 {
		// this should not happen and tests should ensure that.
		if s.data[idx] == nil {
			return fmt.Errorf("desync between s.ids and s.data. s.ids[%d]==%s but s.data[%d] is nil", idx, id, idx)
		}

		return fmt.Errorf("stream with id %s already exists at index %d", id, idx)
	}

	// find the first empty slot
	for i := range len(s.data) {
		if s.data[i] != nil {
			continue
		}

		// slot found
		if s.ids[i] != "" {
			return fmt.Errorf("desync between s.ids and s.data. s.ids[%d]==%s but s.data[%d] is nil", i, id, i)
		}

		s.ids[i] = id
		rb := ringbuffer.New(ringBufferSize)
		s.data[i] = &rb
		return nil
	}

	// vogo server and streams.remove should guard against this.
	return fmt.Errorf("cannot add stream %s. there are already %d streams (max)", id, MaxStreams)
}

// Remove removes a stream. This should be called when a PeerConnection fails.
// It takes the user's name as the id. No-op if the stream is not found.
func (s *Streams) Remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := slices.Index(s.ids[:], id)
	if idx == -1 {
		log.Printf("WARN: stream with id %s has already been removed", id)
		return nil
	}

	if s.data[idx] == nil {
		return fmt.Errorf("desync between s.ids and s.data. s.ids[%d]==%s but s.data[%d] is nil", idx, id, idx)
	}

	s.ids[idx] = ""
	s.data[idx] = nil
	log.Printf("INFO: removed stream %s at index %d", id, idx)
	return nil
}
