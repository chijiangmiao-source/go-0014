package series_test

import (
	"testing"

	"steam-sterilization-thermal-validation/internal/domain"
	"steam-sterilization-thermal-validation/internal/series"
)

func TestFixedGridIncludesRunEnd(t *testing.T) {
	got, err := series.FixedGrid(0, 10, 4)
	if err != nil {
		t.Fatal(err)
	}
	want := []int64{0, 4, 8, 10}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("grid[%d]=%d want %d", i, got[i], want[i])
		}
	}
}

func TestInterpolateRejectsExtrapolation(t *testing.T) {
	_, err := series.Interpolate([]series.Point{{AtNanos: 10, Value: 1}, {AtNanos: 20, Value: 2}}, 5, 20)
	if err == nil {
		t.Fatal("expected extrapolation error")
	}
}

func TestApplyCalibrationUsesOffsetAndUncertainty(t *testing.T) {
	sample := domain.SensorSample{ProbeID: "load-1", AtNanos: 10, Kind: domain.SampleTemperature, Value: 121}
	cal := domain.Calibration{ProbeID: "load-1", ValidFromNanos: 0, ValidUntilNanos: 20, OffsetC: 0.5, UncertaintyC: 0.2}
	got, err := series.ApplyCalibration(sample, cal)
	if err != nil {
		t.Fatal(err)
	}
	if got.CorrectedC != 121.5 || got.Conservative != 121.3 {
		t.Fatalf("unexpected corrected sample: %+v", got)
	}
}
