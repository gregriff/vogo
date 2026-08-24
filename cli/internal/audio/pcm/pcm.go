package pcm

import (
	"fmt"
	"slices"
	"sync"
	"unsafe"

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

// Read reads at most count pcm samples into dst,
// if the internal ringbuffer has count samples available.
func (s *Stream) ReadBytes(dst []byte, count int) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Need at least n samples
	if s.rb.Len() < count {
		return 0
	}

	// Extract count samples and remove it from the buffer
	return s.rb.ReadNBytes(dst, count) // overwrites whatever's in dst
}

// ReadFrame reads up to len(dst) pcm samples into dst. Does not read any if there
// are < FrameSize samples in the stream's ringbuffer, so the caller can wait for more.
func (s *Stream) ReadFrame(dst []int16) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Need at least one frame worth of data
	if s.rb.Len() < FrameSize {
		return 0
	}

	// Extract one frame and remove it from the buffer
	return s.rb.Read(dst) // overwrites whatever's in dst
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
	defer s.mu.Unlock()

	idx := slices.Index(s.ids[:], id)
	if idx == -1 {
		return fmt.Errorf("stream with id %s not found", id)
	}

	if s.data[idx] == nil {
		return fmt.Errorf("stream with id %s is nil", id)
	}

	s.data[idx].Write(src)
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
		return nil
	}

	if s.data[idx] == nil {
		return fmt.Errorf("desync between s.ids and s.data. s.ids[%d]==%s but s.data[%d] is nil", idx, id, idx)
	}

	s.ids[idx] = ""
	s.data[idx] = nil
	return nil
}

// MixAndWrite finds all pcm buffers that have numSamples samples,
// mixes them and copies the mixed array to dst. Fast paths exist for
// 0 or 1 full buffer.
//
// The mixing function uses SIMD and is determined at compile-time by
// build flags.
//
// It is also responsible for zeroing the mixing sink,
// since miniaudio has been configured to not do it itself.
//
// Note: bursty packet arrival could lead to a backup of samples in the ringbuffers.
// Consider implementing time-compressing frames if this is detected.
func (s *Streams) MixAndWrite(dst []byte, numSamples int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// this should never happen. if it does, s.fullStreams only will read
	// len(s.writeBufs[n]) samples. this would leave some samples in the rb?
	// if numSamples > len(s.mixed) {
	// 	log.Panicf("samplesToRead > cap(mixed)")
	// }

	// todo: try storing full as a streams field
	full, numFull := s.fullStreams(numSamples)
	defer clear(full[:])

	// slowdown prob related to numfull and full... scalar 1 stream no slowdown bc of early return
	switch numFull {
	case 0:
		return // nothing to write
	case 1:
		// if only one other person in the room, don't mix, just write their pcm
		ints := ringbuffer.Int16ToBytes(s.writeBufs[0][:numSamples])
		copy(dst, ints)
		return
	}

	// try clearing :numSamples, or no need to clear at all?
	clear(s.mixed[:])

	// write a full mixed sample to the speaker buffer
	s.mix(full, numFull, numSamples)
	mixed := ringbuffer.Int16ToBytes(s.mixed[:numSamples])
	copy(dst, mixed)
}

// BytesToInt16 reinterprets a byte slice of PCM audio into an int16 slice.
func BytesToInt16(b []byte) []int16 {
	if len(b) == 0 {
		return nil
	}
	return unsafe.Slice((*int16)(unsafe.Pointer(&b[0])), len(b)/2)
}
