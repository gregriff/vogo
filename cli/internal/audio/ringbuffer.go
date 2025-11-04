package audio

import (
	"encoding/binary"
	"log"
	"sync/atomic"
)

// reference: https://en.wikipedia.org/wiki/Circular_buffer
type RingBuffer struct {
	buffer []int16

	// TODO: ensure this should not be split into a len and a cap...
	size int64
	writeIdx,
	readIdx atomic.Int64
}

func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{
		buffer: make([]int16, size),
		size:   int64(size),
	}
}

// Write writes to the RingBuffer data from src. Safe for single producer
func (rb *RingBuffer) Write(src []int16) int {
	written := 0
	for _, b := range src {
		writeIdx := rb.writeIdx.Load()
		readIdx := rb.readIdx.Load()

		nextWriteIdx := (writeIdx + 1) % rb.size
		if nextWriteIdx == readIdx {
			log.Println("BUFFER FULL DURING WRITE!")
			break // buffer full
		}

		rb.buffer[writeIdx] = b
		rb.writeIdx.Store(nextWriteIdx) // publish write
		written++
	}
	return written
}

// Read writes to dst data read from the RingBuffer. It exhausts the contents of the RingBuffer
// since the last write. Since the dst buffer is a byte slice, conversion from int16 is
// handled. Returns the number of bytes written. Safe for single consumer
func (rb *RingBuffer) Read(dst []byte) int {
	read := 0

	writeIdx := rb.writeIdx.Load()
	readIdx := rb.readIdx.Load()
	var numToRead int64
	if writeIdx < readIdx {
		numToRead = (rb.size - readIdx) + writeIdx // go around the ring
	} else {
		numToRead = writeIdx - readIdx
	}

	// TODO: use slice assignment and a mutex to instead of lockfree
	for i := range numToRead {
		writeIdx := rb.writeIdx.Load()
		readIdx := rb.readIdx.Load()

		if readIdx == writeIdx {
			break // buffer empty
		}

		nextReadIdx := (readIdx + 1) % rb.size
		rb.readIdx.Store(nextReadIdx)
		binary.LittleEndian.PutUint16(dst[i*2:], uint16(rb.buffer[readIdx]))
		// dst[i] = byte(rb.buffer[readIdx])
		read++
	}
	return read
}
