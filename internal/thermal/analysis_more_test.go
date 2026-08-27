package thermal_test

import (
	"testing"

	"steam-sterilization-thermal-validation/internal/domain"
	"steam-sterilization-thermal-validation/internal/ingest"
	"steam-sterilization-thermal-validation/internal/thermal"
)

func TestMedianAveragesEvenChamberSet(t *testing.T) {
	if got := thermal.Median([]float64{124, 120, 122, 126}); got != 123 {
		t.Fatalf("median = %f", got)
	}
}

func TestSteamCompatibilityIgnoresShortDeviation(t *testing.T) {
	req := ingest.DefaultRequirements(0, 20)
	req.SteamAllowedNanos = 30
	readings := []thermal.SteamReading{{AtNanos: 0, PressureKPa: 205, ChamberC: []float64{130}}, {AtNanos: 20, PressureKPa: 205, ChamberC: []float64{121}}}
	findings, err := thermal.SteamCompatibility(readings, req)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("unexpected findings: %+v", findings)
	}
}

func TestAnalyzeSnapshotProducesPassResult(t *testing.T) {
	result, err := thermal.AnalyzeSnapshot(passSnapshot())
	if err != nil {
		t.Fatal(err)
	}
	if result.Conclusion != domain.ConclusionPass || result.Segment == nil || len(result.Metrics) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestAnalyzeSnapshotProducesFailForLowLoad(t *testing.T) {
	snapshot := passSnapshot()
	for i := range snapshot.Samples {
		if snapshot.Samples[i].ProbeID == "load-1" {
			snapshot.Samples[i].Value = 119
		}
	}
	result, err := thermal.AnalyzeSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if result.Conclusion != domain.ConclusionFail || len(result.Findings) == 0 {
		t.Fatalf("expected fail finding, got %+v", result)
	}
}

func passSnapshot() domain.Snapshot {
	req := ingest.DefaultRequirements(0, 120_000_000_000)
	req.SampleStepNanos = 30_000_000_000
	req.MaxGapNanos = 60_000_000_000
	req.ExposureMinNanos = 60_000_000_000
	req.ConfirmNanos = 30_000_000_000
	req.MinLethalityMinutes = 1
	req.RequiredProbeIDs = []string{"load-1"}
	return domain.Snapshot{
		AlgorithmVersion: ingest.AlgorithmVersion,
		Requirements:     req,
		Probes: []domain.ProbeSpec{
			{ProbeID: "load-1", Role: domain.RoleLoad, Required: true},
			{ProbeID: "chamber-1", Role: domain.RoleChamber, Required: true},
			{ProbeID: "pressure-1", Role: domain.RolePressure, Required: true},
		},
		Calibrations: []domain.Calibration{
			{ProbeID: "load-1", ValidFromNanos: 0, ValidUntilNanos: 120_000_000_000},
			{ProbeID: "chamber-1", ValidFromNanos: 0, ValidUntilNanos: 120_000_000_000},
		},
		Samples: []domain.SensorSample{
			{ProbeID: "load-1", AtNanos: 0, Kind: domain.SampleTemperature, Value: 122, Unit: "C"},
			{ProbeID: "load-1", AtNanos: 60_000_000_000, Kind: domain.SampleTemperature, Value: 122, Unit: "C"},
			{ProbeID: "load-1", AtNanos: 120_000_000_000, Kind: domain.SampleTemperature, Value: 122, Unit: "C"},
			{ProbeID: "chamber-1", AtNanos: 0, Kind: domain.SampleTemperature, Value: 121, Unit: "C"},
			{ProbeID: "chamber-1", AtNanos: 60_000_000_000, Kind: domain.SampleTemperature, Value: 121, Unit: "C"},
			{ProbeID: "chamber-1", AtNanos: 120_000_000_000, Kind: domain.SampleTemperature, Value: 121, Unit: "C"},
			{ProbeID: "pressure-1", AtNanos: 0, Kind: domain.SamplePressure, Value: 205, Unit: "kPa_abs"},
			{ProbeID: "pressure-1", AtNanos: 60_000_000_000, Kind: domain.SamplePressure, Value: 205, Unit: "kPa_abs"},
			{ProbeID: "pressure-1", AtNanos: 120_000_000_000, Kind: domain.SamplePressure, Value: 205, Unit: "kPa_abs"},
		},
	}
}
