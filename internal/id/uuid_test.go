package id

import (
	"regexp"
	"testing"
)

func TestNewUUID(t *testing.T) {
	first, err := NewUUID()
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewUUID()
	if err != nil {
		t.Fatal(err)
	}

	pattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !pattern.MatchString(first) {
		t.Fatalf("invalid UUID v4: %q", first)
	}
	if first == second {
		t.Fatal("expected independently generated UUIDs")
	}
}
