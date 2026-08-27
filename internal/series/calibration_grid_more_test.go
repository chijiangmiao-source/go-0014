package series_test

import (
	"testing"

	"steam-sterilization-thermal-validation/internal/domain"
	"steam-sterilization-thermal-validation/internal/series"
)

func TestCalibrationCoversRunUsesInclusiveEndpoints(t *testing.T) {
	cal := domain.Calibration{ValidFromNanos: 10, ValidUntilNanos: 20}
	if !series.CalibrationCoversRun(cal, 10, 20) {
		t.Fatal("expected calibration to cover exact run endpoints")
	}
}

func TestInterpolateUsesExactEndpointWithoutGapCheck(t *testing.T) {
	points := []series.Point{{AtNanos: 0, Value: 10}, {AtNanos: 100, Value: 20}}
	got, err := series.Interpolate(points, 100, 10)
	if err != nil {
		t.Fatal(err)
	}
	if got != 20 {
		t.Fatalf("got %f want 20", got)
	}
}

func TestCoverageGapsFindsInteriorGap(t *testing.T) {
	points := []series.Point{{AtNanos: 0, Value: 1}, {AtNanos: 10, Value: 2}}
	gaps := series.CoverageGaps(points, 0, 10, 5)
	if len(gaps) != 1 || gaps[0].StartNanos != 0 || gaps[0].EndNanos != 10 {
		t.Fatalf("unexpected gaps: %+v", gaps)
	}
}

func TestUnionGridOrdersUniqueTimes(t *testing.T) {
	got := series.UnionGrid([]int64{3, 1}, []int64{2, 3})
	want := []int64{1, 2, 3}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("grid[%d]=%d want %d", i, got[i], want[i])
		}
	}
}
