package domain_test

import (
	"testing"
	"time"

	"steam-sterilization-thermal-validation/internal/domain"
)

func TestParseRFC3339NanoUTCRequiresZone(t *testing.T) {
	if _, err := domain.ParseRFC3339NanoUTC("2026-08-27T12:00:00"); err == nil {
		t.Fatal("expected missing zone to fail")
	}
}

func TestParseRFC3339NanoUTCNormalizesOffset(t *testing.T) {
	got, err := domain.ParseRFC3339NanoUTC("2026-08-27T20:00:00+08:00")
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC).UnixNano()
	if got != want {
		t.Fatalf("got %d want %d", got, want)
	}
}
