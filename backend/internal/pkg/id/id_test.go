package id_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/takayoshi-u/polaris/backend/internal/pkg/id"
)

func TestNewV7(t *testing.T) {
	u1 := id.NewV7()
	time.Sleep(2 * time.Millisecond)
	u2 := id.NewV7()

	if u1.Version() != uuid.Version(7) {
		t.Fatalf("expected UUID version 7, got %v", u1.Version())
	}
	if u2.Version() != uuid.Version(7) {
		t.Fatalf("expected UUID version 7, got %v", u2.Version())
	}
	if u1.String() >= u2.String() {
		t.Fatalf("expected UUID v7 to be lexicographically ordered by time: %s vs %s", u1.String(), u2.String())
	}
	if !id.IsValid(u1.String()) {
		t.Fatalf("expected IsValid to return true for %s", u1.String())
	}
	if id.IsValid("not-a-uuid") {
		t.Fatalf("expected IsValid to return false for invalid string")
	}
}

