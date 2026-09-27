package ringbuffer

import (
	"testing"
)

func TestRingBuffer(t *testing.T) {
	t.Run("writing", func(t *testing.T) {
		rbCap, writeLen := 10, 4

		r := New(rbCap)
		r.Write([]int16{})
		if r.Len() != 0 {
			t.Errorf("nil write failed, Len() should return 0, got %d", r.Len())
		}

		r = New(rbCap)
		src := make([]int16, 0)
		r.Write(src)
		if r.Len() != 0 {
			t.Errorf("empty write failed, Len() should return 0, got %d", r.Len())
		}

		r = New(rbCap)
		src = make([]int16, writeLen)
		r.Write(src)
		if r.Len() != writeLen {
			t.Errorf("small write failed, expected len %d, got %d", writeLen, r.Len())
		}
		if r.tail != writeLen {
			t.Errorf("small write failed, expected tail=%d, got %d", writeLen, r.tail)
		}

		r = New(rbCap)
		src = make([]int16, rbCap)
		r.Write(src)
		if !r.Full() {
			t.Errorf("full write failed, expected len %d, got %d", rbCap, r.Len())
		}
		if r.head != 0 {
			t.Errorf("full write failed, expected head=0, got %d", r.head)
		}
		if r.tail != 0 {
			t.Errorf("full write failed, expected tail=0, got %d", r.tail)
		}

		wraparoundLen := 2
		r = New(rbCap)
		src = make([]int16, rbCap+wraparoundLen)
		r.Write(src[:4])
		r.Write(src[4:])
		if r.tail != wraparoundLen {
			t.Errorf("wraparound write failed, expected tail at %d, got %d", wraparoundLen, r.tail)
		}
		if !r.Full() {
			t.Errorf("wraparound write failed, expected len %d, got %d", rbCap, r.Len())
		}
	})

	t.Run("reading", func(t *testing.T) {
		rbCap, readLen := 10, 4
		src := make([]int16, rbCap) // will fill rb entirely

		r := New(rbCap)
		r.Write(src)
		n := r.Read([]int16{})
		if n != 0 {
			t.Errorf("nil read failed, expected n=0, got %d", n)
		}

		r = New(rbCap)
		r.Write(src)
		emptyDst := make([]int16, 0)
		n = r.Read(emptyDst)
		if n != 0 {
			t.Errorf("empty read failed, expected n=0, got %d", n)
		}

		r = New(rbCap)
		r.Write(src)
		dst := make([]int16, readLen)
		n = r.Read(dst)
		if len(dst) != readLen {
			t.Errorf("read failed, expected n=%d, got %d", readLen, n)
		}
		if r.head != readLen {
			t.Errorf("read failed, expected head=%d, got %d", readLen, r.head)
		}
		if r.tail != 0 { // 0 because we wrote a full buf to rb.
			t.Errorf("read failed, expected tail=0, got %d", r.tail)
		}

	})
}
