package pcm

import (
	"fmt"
	"testing"
)

// assertTracked checks that id is present at some index and that
// s.data is non-nil at that index. Returns the index found.
func assertTracked(t *testing.T, s *Streams, id string) int {
	t.Helper()
	idx := -1
	for i, gotID := range s.ids {
		if gotID == id && s.data[i] != nil {
			idx = i
			break
		}
	}
	if idx == -1 {
		t.Fatalf("id %q not found in s.ids/s.data", id)
	}
	return idx
}

func assertAbsent(t *testing.T, s *Streams, id string) {
	t.Helper()
	for i, gotID := range s.ids {
		if gotID == id {
			t.Fatalf("expected id %q to be absent, found at index %d", id, i)
		}
	}
}

func assertSlotEmpty(t *testing.T, s *Streams, idx int) {
	t.Helper()
	if s.ids[idx] != "" {
		t.Fatalf("expected s.ids[%d] to be empty, got %q", idx, s.ids[idx])
	}
	if s.data[idx] != nil {
		t.Fatalf("expected s.data[%d] to be nil, got %p", idx, s.data[idx])
	}
}

func countID(s *Streams, id string) int {
	count := 0
	for _, gotID := range s.ids {
		if gotID == id {
			count++
		}
	}
	return count
}

func TestAdd_Basic(t *testing.T) {
	s := NewStreams()

	if err := s.AddNew("alice"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertTracked(t, &s, "alice")
}

func TestAdd_MultipleDistinctIDs(t *testing.T) {
	s := NewStreams()

	if err := s.AddNew("alice"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := s.AddNew("bob"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertTracked(t, &s, "alice")
	assertTracked(t, &s, "bob")
}

func TestAdd_FillsToMax(t *testing.T) {
	s := NewStreams()

	for i := range MaxStreams {
		if err := s.AddNew(fmt.Sprintf("user-%d", i)); err != nil {
			t.Fatalf("unexpected error adding stream %d: %v", i, err)
		}
	}

	for i := range MaxStreams {
		if s.data[i] == nil {
			t.Fatalf("expected slot %d to be filled", i)
		}
	}
}

func TestAdd_PastMax_ReturnsError(t *testing.T) {
	s := NewStreams()

	for i := range MaxStreams {
		if err := s.AddNew(fmt.Sprintf("user-%d", i)); err != nil {
			t.Fatalf("unexpected error adding stream %d: %v", i, err)
		}
	}

	if err := s.AddNew("overflow"); err == nil {
		t.Fatal("expected error when adding beyond MaxStreams, got nil")
	}
}

func TestRemove_Basic(t *testing.T) {
	s := NewStreams()

	if err := s.AddNew("alice"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := s.Remove("alice"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertAbsent(t, &s, "alice")
}

func TestRemove_NonExistent_NoError(t *testing.T) {
	s := NewStreams()

	if err := s.Remove("nobody"); err != nil {
		t.Fatalf("expected no error removing nonexistent id, got: %v", err)
	}
}

func TestRemove_Twice_NoError(t *testing.T) {
	s := NewStreams()
	if err := s.AddNew("alice"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := s.Remove("alice"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := s.Remove("alice"); err != nil {
		t.Fatalf("expected no error on second remove, got: %v", err)
	}
}

func TestRemove_DoesNotAffectOtherIDs(t *testing.T) {
	s := NewStreams()

	if err := s.AddNew("alice"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := s.AddNew("bob"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := s.AddNew("carol"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := s.Remove("bob"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertAbsent(t, &s, "bob")
	assertTracked(t, &s, "alice")
	assertTracked(t, &s, "carol")
}

func TestAddAfterRemove_ReusesSlot(t *testing.T) {
	s := NewStreams()

	if err := s.AddNew("alice"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	idx := assertTracked(t, &s, "alice")

	if err := s.Remove("alice"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertSlotEmpty(t, &s, idx)

	if err := s.AddNew("bob"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	idx2 := assertTracked(t, &s, "bob")

	// add scans for the first nil slot, so the freed slot should be reused.
	if idx != idx2 {
		t.Fatalf("expected freed slot %d to be reused by new add, got %d want %d", idx, idx2, idx)
	}
}
