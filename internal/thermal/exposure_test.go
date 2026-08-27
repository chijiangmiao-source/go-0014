package thermal_test

import (
	"testing"

	"steam-sterilization-thermal-validation/internal/domain"
	"steam-sterilization-thermal-validation/internal/thermal"
)

func TestIdentifyExposureHonorsConfirmWindow(t *testing.T) {
	req := domain.Requirements{ExposureMinC: 121, ConfirmNanos: 2, GraceNanos: 1, SpreadMaxC: 2, RequiredProbeIDs: []string{"load-1"}}
	readings := []thermal.GridReading{
		{AtNanos: 0, LoadLowerC: map[string]float64{"load-1": 120}, ChamberTempsC: []float64{121}},
		{AtNanos: 1, LoadLowerC: map[string]float64{"load-1": 121}, ChamberTempsC: []float64{121}},
		{AtNanos: 2, LoadLowerC: map[string]float64{"load-1": 122}, ChamberTempsC: []float64{121}},
		{AtNanos: 3, LoadLowerC: map[string]float64{"load-1": 123}, ChamberTempsC: []float64{121}},
	}
	segment, err := thermal.IdentifyExposure(readings, req)
	if err != nil {
		t.Fatal(err)
	}
	if segment == nil || segment.StartNanos != 1 || segment.EndNanos != 3 {
		t.Fatalf("unexpected segment: %+v", segment)
	}
}

func TestIdentifyExposureReturnsNilWhenNoSegment(t *testing.T) {
	req := domain.Requirements{ExposureMinC: 121, ConfirmNanos: 1, RequiredProbeIDs: []string{"load-1"}}
	segment, err := thermal.IdentifyExposure([]thermal.GridReading{{AtNanos: 0, LoadLowerC: map[string]float64{"load-1": 120}}}, req)
	if err != nil {
		t.Fatal(err)
	}
	if segment != nil {
		t.Fatalf("expected no segment, got %+v", segment)
	}
}
