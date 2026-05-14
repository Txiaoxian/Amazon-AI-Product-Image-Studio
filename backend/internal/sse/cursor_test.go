package sse

import (
	"errors"
	"testing"
)

func TestCursorFromEventIDsPrefersHeaderOverQueryFallback(t *testing.T) {
	sequence, err := CursorFromEventIDs("evt_00000000000000000007", "evt_00000000000000000003")
	if err != nil {
		t.Fatalf("parse cursor: %v", err)
	}
	if sequence != 7 {
		t.Fatalf("sequence = %d, want 7", sequence)
	}
}

func TestCursorFromEventIDsUsesQueryFallback(t *testing.T) {
	sequence, err := CursorFromEventIDs("", "evt_00000000000000000003")
	if err != nil {
		t.Fatalf("parse cursor: %v", err)
	}
	if sequence != 3 {
		t.Fatalf("sequence = %d, want 3", sequence)
	}
}

func TestParseEventIDRejectsMalformedValues(t *testing.T) {
	for _, value := range []string{"", "1", "evt_", "evt_abc", "evt_0001_suffix", "bad_0001"} {
		if _, err := ParseEventID(value); !errors.Is(err, ErrInvalidEventID) {
			t.Fatalf("ParseEventID(%q) error = %v, want ErrInvalidEventID", value, err)
		}
	}
}
