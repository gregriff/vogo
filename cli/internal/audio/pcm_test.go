package audio

import (
	"fmt"
	"testing"

	"github.com/gregriff/vogo/cli/internal/audio/ringbuffer"
)

func newRB() *ringbuffer.RingBuffer {
	rb := ringbuffer.New(ringBufferSize)
	return &rb
}

// assertTracked checks that id/rb are present at some index and that
// s.ids and s.data agree with each other at that index. Returns the index found.
func assertTracked(t *testing.T, s *streams, id string, rb *ringbuffer.RingBuffer) int {
	t.Helper()
	idx := -1
	for i, gotID := range s.ids {
		if gotID == id && s.data[i] == rb {
			idx = i
			break
		}
	}
	if idx == -1 {
		t.Fatalf("id %q with rb %p not found in s.ids/s.data", id, rb)
	}
	return idx
}

func assertAbsent(t *testing.T, s *streams, id string) {
	t.Helper()
	for i, gotID := range s.ids {
		if gotID == id {
			t.Fatalf("expected id %q to be absent, found at index %d", id, i)
		}
	}
}

func assertSlotEmpty(t *testing.T, s *streams, idx int) {
	t.Helper()
	if s.ids[idx] != "" {
		t.Fatalf("expected s.ids[%d] to be empty, got %q", idx, s.ids[idx])
	}
	if s.data[idx] != nil {
		t.Fatalf("expected s.data[%d] to be nil, got %p", idx, s.data[idx])
	}
}

func countID(s *streams, id string) int {
	count := 0
	for _, gotID := range s.ids {
		if gotID == id {
			count++
		}
	}
	return count
}

func TestAdd_Basic(t *testing.T) {
	s := newStreams()
	rb := newRB()

	if err := s.add("alice", rb); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertTracked(t, &s, "alice", rb)
}

func TestAdd_MultipleDistinctIDs(t *testing.T) {
	s := newStreams()
	rbA, rbB := newRB(), newRB()

	if err := s.add("alice", rbA); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := s.add("bob", rbB); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertTracked(t, &s, "alice", rbA)
	assertTracked(t, &s, "bob", rbB)
}

func TestAdd_FillsToMax(t *testing.T) {
	s := newStreams()

	for i := 0; i < MaxStreams; i++ {
		if err := s.add(fmt.Sprintf("user-%d", i), newRB()); err != nil {
			t.Fatalf("unexpected error adding stream %d: %v", i, err)
		}
	}

	for i := 0; i < MaxStreams; i++ {
		if s.data[i] == nil {
			t.Fatalf("expected slot %d to be filled", i)
		}
	}
}

func TestAdd_PastMax_ReturnsError(t *testing.T) {
	s := newStreams()
	for i := 0; i < MaxStreams; i++ {
		if err := s.add(fmt.Sprintf("user-%d", i), newRB()); err != nil {
			t.Fatalf("unexpected error adding stream %d: %v", i, err)
		}
	}

	err := s.add("overflow", newRB())
	if err == nil {
		t.Fatal("expected error when adding beyond MaxStreams, got nil")
	}
}

func TestRemove_Basic(t *testing.T) {
	s := newStreams()
	rb := newRB()
	if err := s.add("alice", rb); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := s.remove("alice"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertAbsent(t, &s, "alice")
}

func TestRemove_NonExistent_NoError(t *testing.T) {
	s := newStreams()

	if err := s.remove("nobody"); err != nil {
		t.Fatalf("expected no error removing nonexistent id, got: %v", err)
	}
}

func TestRemove_Twice_NoError(t *testing.T) {
	s := newStreams()
	if err := s.add("alice", newRB()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := s.remove("alice"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := s.remove("alice"); err != nil {
		t.Fatalf("expected no error on second remove, got: %v", err)
	}
}

func TestRemove_DoesNotAffectOtherIDs(t *testing.T) {
	s := newStreams()
	rbA, rbB, rbC := newRB(), newRB(), newRB()

	if err := s.add("alice", rbA); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := s.add("bob", rbB); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := s.add("carol", rbC); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := s.remove("bob"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertAbsent(t, &s, "bob")
	assertTracked(t, &s, "alice", rbA)
	assertTracked(t, &s, "carol", rbC)
}

func TestAddAfterRemove_ReusesSlot(t *testing.T) {
	s := newStreams()
	rbA := newRB()
	if err := s.add("alice", rbA); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	idx := assertTracked(t, &s, "alice", rbA)

	if err := s.remove("alice"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertSlotEmpty(t, &s, idx)

	rbB := newRB()
	if err := s.add("bob", rbB); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// add scans for the first nil slot, so the freed slot should be reused.
	if s.data[idx] != rbB {
		t.Fatalf("expected freed slot %d to be reused by new add, got %p want %p", idx, s.data[idx], rbB)
	}
}
