package ringbuffer

// RingBuffer is a circular buffer of pcm data that allows for reading and writing without
// reslicing, preventing allocations and GC pressure. RingBuffer is meant to be shared
// by pointer between the writer and reader, but all methods assume they are using a mutex
// to prevent data races. RingBuffer should be initialized to be multiple times larger than the
// pcm frame size as to not overwrite samples during bad network conditions. The length and capacity
// of the underlying buffer are never used in any operation.
type RingBuffer struct {
	// pcm data
	buf []int16

	// next read pos
	head int

	// next write pos
	tail int

	// num of unread elems in buf
	count int

	// capacity
	size int
}

func New(size int) RingBuffer {
	return RingBuffer{
		buf:  make([]int16, size),
		size: size,
	}
}

// Write adds all samples in src, overwriting oldest data if full.
func (r *RingBuffer) Write(src []int16) {
	n := len(src)
	if n == 0 {
		return
	}

	// split into two copy calls if we wrap around
	space := r.size - r.tail
	if n <= space {
		copy(r.buf[r.tail:], src)
	} else {
		copy(r.buf[r.tail:], src[:space])
		copy(r.buf[0:], src[space:])
	}

	r.tail = (r.tail + n) % r.size

	if n >= r.size {
		// wrote more than capacity — head follows tail
		r.head = r.tail
		r.count = r.size
	} else if r.count+n > r.size {
		overflow := r.count + n - r.size
		r.head = (r.head + overflow) % r.size
		r.count = r.size
	} else {
		r.count += n
	}
}

// Read consumes up to len(dst) samples and returns how many were read.
func (r *RingBuffer) Read(dst []int16) int {
	n := min(r.count, len(dst))
	if n == 0 {
		return 0
	}

	space := r.size - r.head
	if n <= space {
		copy(dst, r.buf[r.head:r.head+n])
	} else {
		copy(dst, r.buf[r.head:])
		copy(dst[space:], r.buf[:n-space])
	}

	r.head = (r.head + n) % r.size
	r.count -= n
	return n
}

// Len returns the logical length: the number of unread samples in the ringbuffer
func (r *RingBuffer) Len() int    { return r.count }
func (r *RingBuffer) Full() bool  { return r.count == r.size }
func (r *RingBuffer) Empty() bool { return r.count == 0 }

// Peek reads len(dst) samples into dst without consuming.
// func (r *RingBuffer) Peek(dst []int16) int {
// 	n := min(r.count, len(dst))
// 	for i := range n {
// 		dst[i] = r.buf[(r.head+i)%r.size]
// 	}
// 	return n
// }

// func (r *RingBuffer) Cap() int    { return r.size }
// func (r *RingBuffer) Reset() {
// 	r.head = 0
// 	r.tail = 0
// 	r.count = 0
// }
