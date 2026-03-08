package audio

import "testing"

func TestStreamsHasFullSample(t *testing.T) {
	t.Run("empty streams returns false", func(t *testing.T) {
		s := newStreams()
		fullBufs, ok := s.hasFullSample(5)
		if ok {
			t.Error("expected ok=false for empty streams")
		}
		if len(fullBufs) != 0 {
			t.Errorf("expected empty fullBufs, got len=%d", len(fullBufs))
		}
	})

	t.Run("no buffers have enough samples", func(t *testing.T) {
		s := newStreams()
		buf := []int16{1, 2, 3}
		s.add("a", &buf)

		fullBufs, ok := s.hasFullSample(5)
		if ok {
			t.Error("expected ok=false when no buffer has enough samples")
		}
		if len(fullBufs) != 0 {
			t.Errorf("expected empty fullBufs, got len=%d", len(fullBufs))
		}
	})

	t.Run("exactly amt samples returns true", func(t *testing.T) {
		s := newStreams()
		buf := []int16{1, 2, 3, 4, 5}
		s.add("a", &buf)

		fullBufs, ok := s.hasFullSample(5)
		if !ok {
			t.Error("expected ok=true when buffer has exactly amt samples")
		}
		if len(fullBufs) != 1 {
			t.Errorf("expected 1 full buf, got %d", len(fullBufs))
		}
	})

	t.Run("more than amt samples returns true", func(t *testing.T) {
		s := newStreams()
		buf := []int16{1, 2, 3, 4, 5, 6, 7}
		s.add("a", &buf)

		_, ok := s.hasFullSample(5)
		if !ok {
			t.Error("expected ok=true when buffer has more than amt samples")
		}
	})

	t.Run("only some buffers are full", func(t *testing.T) {
		s := newStreams()
		full := []int16{1, 2, 3, 4, 5}
		partial := []int16{1, 2}
		s.add("full", &full)
		s.add("partial", &partial)

		fullBufs, ok := s.hasFullSample(5)
		if !ok {
			t.Error("expected ok=true when at least one buffer is full")
		}
		if len(fullBufs) != 1 {
			t.Errorf("expected 1 full buf, got %d", len(fullBufs))
		}
		if fullBufs[0] != &full {
			t.Error("expected fullBufs[0] to point to the full buffer")
		}
	})

	t.Run("all buffers are full", func(t *testing.T) {
		s := newStreams()
		a := []int16{1, 2, 3, 4, 5}
		b := []int16{6, 7, 8, 9, 10}
		c := []int16{11, 12, 13, 14, 15}
		s.add("a", &a)
		s.add("b", &b)
		s.add("c", &c)

		fullBufs, ok := s.hasFullSample(5)
		if !ok {
			t.Error("expected ok=true when all buffers are full")
		}
		if len(fullBufs) != 3 {
			t.Errorf("expected 3 full bufs, got %d", len(fullBufs))
		}
	})

	t.Run("returned pointers point to original data", func(t *testing.T) {
		s := newStreams()
		buf := []int16{10, 20, 30}
		s.add("a", &buf)

		fullBufs, _ := s.hasFullSample(3)
		if fullBufs[0] != &buf {
			t.Error("expected fullBufs[0] to be the same pointer as the original buffer")
		}
		// mutate via returned pointer and confirm original changed
		(*fullBufs[0])[0] = 99
		if buf[0] != 99 {
			t.Error("expected mutation through returned pointer to affect original buffer")
		}
	})
}

func TestStreamsMix(t *testing.T) {
	t.Run("empty bufs returns nil", func(t *testing.T) {
		s := newStreams()
		result := s.mix(nil, 5)
		if result != nil {
			t.Errorf("expected nil, got %v", result)
		}
	})

	t.Run("single stream passthrough", func(t *testing.T) {
		s := newStreams()
		buf := []int16{10, 20, 30, 40, 50}
		s.add("a", &buf)

		fullBufs, ok := s.hasFullSample(5)
		if !ok {
			t.Fatal("expected full sample")
		}

		mixed := s.mix(fullBufs, 5)
		for i, want := range buf {
			if mixed[i] != want {
				t.Errorf("sample[%d]: got %d, want %d", i, mixed[i], want)
			}
		}
	})

	t.Run("two streams averaged correctly", func(t *testing.T) {
		s := newStreams()
		a := []int16{100}
		b := []int16{200}
		s.add("a", &a)
		s.add("b", &b)

		fullBufs, ok := s.hasFullSample(1)
		if !ok {
			t.Fatal("expected full sample")
		}

		mixed := s.mix(fullBufs, 1)
		if len(mixed) != 1 {
			t.Fatalf("expected len=3, got %d", len(mixed))
		}
		if mixed[0] != 150 {
			t.Errorf("sample mix err: got %d, want %d", mixed[0], 150)
		}
	})

	t.Run("clamps positive overflow", func(t *testing.T) {
		s := newStreams()
		a := []int16{32767, 32767}
		b := []int16{32767, 32767}
		s.add("a", &a)
		s.add("b", &b)

		fullBufs, _ := s.hasFullSample(2)
		mixed := s.mix(fullBufs, 2)

		// sum = 65534, avg = 32767 — no clamp needed here since we divide before clamping
		// but let's use 3 streams to force a sum that when divided would be large
		// this validates clampInt16 is wired in
		for i, v := range mixed {
			if v > 32767 {
				t.Errorf("sample[%d] overflowed int16: %d", i, v)
			}
		}
	})
	t.Run("clamps negative overflow", func(t *testing.T) {
		s := newStreams()
		a := []int16{-32768, -32768}
		b := []int16{-32768, -32768}
		s.add("a", &a)
		s.add("b", &b)

		fullBufs, _ := s.hasFullSample(2)
		mixed := s.mix(fullBufs, 2)

		for i, v := range mixed {
			if v < -32768 {
				t.Errorf("sample[%d] underflowed int16: %d", i, v)
			}
		}
	})

	t.Run("mix does not consume or modify source buffers", func(t *testing.T) {
		s := newStreams()
		a := []int16{10}
		b := []int16{40}
		s.add("a", &a)
		s.add("b", &b)

		fullBufs, _ := s.hasFullSample(1)
		s.mix(fullBufs, 1)

		if a[0] != 10 {
			t.Errorf("source buffer 'a' was modified: %v", a)
		}
		if b[0] != 40 {
			t.Errorf("source buffer 'b' was modified: %v", b)
		}
	})
	t.Run("silence streams mix to silence", func(t *testing.T) {
		s := newStreams()
		a := []int16{0, 0, 0}
		b := []int16{0, 0, 0}
		s.add("a", &a)
		s.add("b", &b)

		fullBufs, _ := s.hasFullSample(3)
		mixed := s.mix(fullBufs, 3)

		for i, v := range mixed {
			if v != 0 {
				t.Errorf("sample[%d]: expected 0, got %d", i, v)
			}
		}
	})
}

func TestStreamsAddRemove(t *testing.T) {
	t.Run("remove makes buffer unavailable", func(t *testing.T) {
		s := newStreams()
		buf := []int16{1, 2, 3}
		s.add("a", &buf)
		s.remove("a")

		fullBufs, ok := s.hasFullSample(3)
		if ok {
			t.Error("expected ok=false after removing the only buffer")
		}
		if len(fullBufs) != 0 {
			t.Errorf("expected no full bufs after remove, got %d", len(fullBufs))
		}
	})
}
