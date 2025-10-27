package internal

import "sync/atomic"

type RingBuffer struct {
	buffer []byte
	size   int64
	write  int64 // write pos
	read   int64 // read pos
}

func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{
		buffer: make([]byte, size),
		size:   int64(size),
	}
}

// Write writes to the RingBuffer data from src. Safe for single producer
func (rb *RingBuffer) Write(src []byte) int {
	written := 0
	for _, b := range src {
		curWrite := atomic.LoadInt64(&rb.write)
		curRead := atomic.LoadInt64(&rb.read)

		nextWrite := (curWrite + 1) % rb.size
		if nextWrite == curRead {
			break // buffer full
		}

		rb.buffer[curWrite] = b
		atomic.StoreInt64(&rb.write, nextWrite) // publish write
		written++
	}
	return written
}

// Read writes to dst data read from the RingBuffer. Safe for single consumer
func (rb *RingBuffer) Read(dst []byte) int {
	read := 0
	for i := range dst {
		curRead := atomic.LoadInt64(&rb.read)
		curWrite := atomic.LoadInt64(&rb.write)

		if curRead == curWrite {
			break // buffer empty
		}

		dst[i] = rb.buffer[curRead]
		atomic.StoreInt64(&rb.read, (curRead+1)%rb.size) // publish read
		read++
	}
	return read
}
