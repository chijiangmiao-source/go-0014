package ingest_test

import (
	"strings"
	"testing"

	"steam-sterilization-thermal-validation/internal/domain"
	"steam-sterilization-thermal-validation/internal/ingest"
)

func TestNormalizeSampleTreatsOffsetTimesAsSameUTCInstant(t *testing.T) {
	a, err := ingest.NormalizeSample(ingest.RawSample{ProbeID: "load-1", Time: "2026-08-27T12:00:00Z", Kind: domain.SampleTemperature, Value: 121, Unit: "C"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := ingest.NormalizeSample(ingest.RawSample{ProbeID: "load-1", Time: "2026-08-27T20:00:00+08:00", Kind: domain.SampleTemperature, Value: 121, Unit: "C"})
	if err != nil {
		t.Fatal(err)
	}
	if a.AtNanos != b.AtNanos {
		t.Fatalf("UTC nanos differ: %d %d", a.AtNanos, b.AtNanos)
	}
}

func TestFoldSamplesCollapsesDuplicateSameValue(t *testing.T) {
	samples := []domain.SensorSample{
		{ProbeID: "load-1", AtNanos: 1, Kind: domain.SampleTemperature, Value: 121, Unit: "C"},
		{ProbeID: "load-1", AtNanos: 1, Kind: domain.SampleTemperature, Value: 121, Unit: "C"},
	}
	got, err := ingest.FoldSamples(samples)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d folded samples", len(got))
	}
}

func TestParseSamplesCSVNormalizesPublicFormat(t *testing.T) {
	csv := "probe_id,time,kind,value,unit\np,2026-08-27T12:00:00Z,temperature,250,F\n"
	got, hash, err := ingest.ParseSamplesCSV([]byte(csv), 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].SourceOrdinal != 4 || got[0].Unit != "C" || hash == "" {
		t.Fatalf("unexpected parsed CSV: %+v hash %q", got, hash)
	}
}

func TestValidateRequirementsRejectsNonMonotonicSteamTable(t *testing.T) {
	req := ingest.DefaultRequirements(0, 10)
	req.FrozenSteamTable = []domain.SteamPoint{{PressureKPa: 200, SaturatedC: 120}, {PressureKPa: 100, SaturatedC: 121}}
	probes := []domain.ProbeSpec{{ProbeID: "load-1", Role: domain.RoleLoad, Required: true}}
	if err := ingest.ValidateRequirements(req, probes); err == nil {
		t.Fatal("expected invalid steam table")
	}
}

func TestValidateFreezeReadinessReportsUnknownSampleProbe(t *testing.T) {
	req := ingest.DefaultRequirements(0, 10)
	req.RequiredProbeIDs = []string{"load-1"}
	err := ingest.ValidateFreezeReadiness(domain.Snapshot{
		AlgorithmVersion: ingest.AlgorithmVersion,
		Requirements:     req,
		Probes:           []domain.ProbeSpec{{ProbeID: "load-1", Role: domain.RoleLoad, Required: true}},
		Calibrations:     []domain.Calibration{{ProbeID: "load-1", ValidFromNanos: 0, ValidUntilNanos: 10}},
		Samples:          []domain.SensorSample{{ProbeID: "missing", AtNanos: 0, Kind: domain.SampleTemperature, Value: 121, Unit: "C"}},
	})
	if err == nil || !strings.Contains(err.Error(), "unknown probe") {
		t.Fatalf("expected unknown probe error, got %v", err)
	}
}
