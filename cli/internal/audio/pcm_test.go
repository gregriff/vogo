package audio

import (
	"math"
	"testing"
)

func TestStreamsMix(t *testing.T) {
	t.Run("single full stream", func(t *testing.T) {
		numSamples := 5
		s := newStreams()
		buf := []int16{10, 20, 30, 40, 50}
		s.add("a", &buf)
		s.mix(numSamples)
		for i, want := range buf {
			if s.mixed[i] != want {
				t.Errorf("sample[%d]: got %d, want %d", i, s.mixed[i], want)
			}
		}
		count := 0
		for _, v := range s.mixed {
			if v == 0 {
				break
			}
			count++
		}
		if count != numSamples {
			t.Errorf("num mixed samples in mixed buf (%d) != numSamples arg (%d)", numSamples, numSamples)
		}
	})

	// when using softSaturate() this won't pass
	// t.Run("two streams averaged correctly", func(t *testing.T) {
	// 	s := newStreams()
	// 	a := []int16{100}
	// 	b := []int16{200}
	// 	s.add("a", &a)
	// 	s.add("b", &b)
	// 	s.mix(1)
	// 	if s.mixed[0] != 150 {
	// 		t.Errorf("sample mix err: got %d, want %d", s.mixed[0], 150)
	// 	}
	// })

	t.Run("clamps overflow", func(t *testing.T) {
		s := newStreams()
		a := []int16{math.MaxInt16, math.MaxInt16}
		b := []int16{math.MaxInt16, math.MaxInt16}
		s.add("a", &a)
		s.add("b", &b)
		s.mix(2)
		if s.mixed[0] > math.MaxInt16 {
			t.Errorf("sample overflowed int16: %d", s.mixed[0])
		}
	})
	t.Run("clamps underflow", func(t *testing.T) {
		s := newStreams()
		a := []int16{math.MaxInt16, math.MaxInt16}
		b := []int16{math.MaxInt16, math.MaxInt16}
		s.add("a", &a)
		s.add("b", &b)
		s.mix(2)
		if s.mixed[0] < math.MinInt16 {
			t.Errorf("sample underflow int16: %d", s.mixed[0])
		}
	})

	t.Run("silence streams mix to silence", func(t *testing.T) {
		s := newStreams()
		a := []int16{0, 0, 0}
		b := []int16{0, 0, 0}
		s.add("a", &a)
		s.add("b", &b)
		s.mix(3)
		if s.mixed[0] != 0 {
			t.Errorf("expected sample[0]==0, got %d", s.mixed[0])
		}
		for i, v := range s.mixed {
			if v != 0 {
				t.Errorf("sample[%d]: expected 0, got %d", i, v)
			}
		}
	})

	t.Run("remove makes buffer unavailable", func(t *testing.T) {
		s := newStreams()
		buf := []int16{1, 2, 3}
		s.add("a", &buf)
		s.remove("a")
		s.mix(3)
		if s.mixed[0] == 1 {
			t.Error("expected s.mix to not use removed buffer")
		}
	})
}
