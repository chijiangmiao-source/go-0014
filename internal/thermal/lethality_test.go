package thermal_test

import (
	"math"
	"testing"

	"steam-sterilization-thermal-validation/internal/domain"
	"steam-sterilization-thermal-validation/internal/series"
	"steam-sterilization-thermal-validation/internal/thermal"
)

func TestLethalityMinutesUsesTrapezoid(t *testing.T) {
	req := domain.Requirements{TRefC: 121, ZC: 10}
	metric, err := thermal.LethalityMinutes("load-1", []series.Point{{AtNanos: 0, Value: 121}, {AtNanos: int64(timeMinute), Value: 121}}, req)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(metric.EquivalentMinutes-1) > 1e-12 {
		t.Fatalf("got %f want 1", metric.EquivalentMinutes)
	}
}

func TestSaturatedTemperatureInterpolatesAndRejectsOutOfRange(t *testing.T) {
	table := []domain.SteamPoint{{PressureKPa: 100, SaturatedC: 100}, {PressureKPa: 200, SaturatedC: 120}}
	got, err := thermal.SaturatedTemperature(table, 150)
	if err != nil {
		t.Fatal(err)
	}
	if got != 110 {
		t.Fatalf("got %f want 110", got)
	}
	if _, err := thermal.SaturatedTemperature(table, 250); err == nil {
		t.Fatal("expected out of range error")
	}
}

const timeMinute = 60 * 1_000_000_000
