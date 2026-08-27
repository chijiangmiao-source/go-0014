package ingest_test

import (
	"testing"

	"steam-sterilization-thermal-validation/internal/domain"
	"steam-sterilization-thermal-validation/internal/ingest"
)

func TestNormalizeSampleConvertsTemperatureAndTime(t *testing.T) {
	got, err := ingest.NormalizeSample(ingest.RawSample{
		ProbeID: "load-1", Time: "2026-08-27T12:00:00Z", Kind: domain.SampleTemperature, Value: 250, Unit: "F",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Unit != "C" || got.Value != 121.11111111111111 {
		t.Fatalf("unexpected normalized sample: %+v", got)
	}
}

func TestFoldSamplesRejectsConflict(t *testing.T) {
	samples := []domain.SensorSample{
		{ProbeID: "p1", AtNanos: 1, Kind: domain.SampleTemperature, Value: 121, Unit: "C"},
		{ProbeID: "p1", AtNanos: 1, Kind: domain.SampleTemperature, Value: 122, Unit: "C"},
	}
	if _, err := ingest.FoldSamples(samples); err == nil {
		t.Fatal("expected sample conflict")
	}
}

func TestCanonicalSnapshotHashIgnoresSampleOrder(t *testing.T) {
	a := domain.Snapshot{AlgorithmVersion: "foundation-1", Samples: []domain.SensorSample{{ProbeID: "b", AtNanos: 2}, {ProbeID: "a", AtNanos: 1}}}
	b := domain.Snapshot{AlgorithmVersion: "foundation-1", Samples: []domain.SensorSample{{ProbeID: "a", AtNanos: 1}, {ProbeID: "b", AtNanos: 2}}}
	_, ha, err := ingest.CanonicalSnapshot(a)
	if err != nil {
		t.Fatal(err)
	}
	_, hb, err := ingest.CanonicalSnapshot(b)
	if err != nil {
		t.Fatal(err)
	}
	if ha != hb {
		t.Fatalf("hashes differ: %s %s", ha, hb)
	}
}
