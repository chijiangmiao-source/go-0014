package thermal

import (
	"math"
	"sort"

	"steam-sterilization-thermal-validation/internal/domain"
)

type SteamReading struct {
	AtNanos      int64
	PressureKPa  float64
	ChamberC     []float64
	ExpectedC    float64
	DeviationAbs float64
}

func SteamCompatibility(readings []SteamReading, req domain.Requirements) ([]domain.Finding, error) {
	var findings []domain.Finding
	var open *domain.Finding
	for _, reading := range readings {
		if !domain.IsFinite(reading.PressureKPa) {
			return nil, domain.NewInputError(domain.CodeDataUnrecoverable, "pressure reading is non-finite")
		}
		expected, err := SaturatedTemperature(req.FrozenSteamTable, reading.PressureKPa)
		if err != nil {
			return nil, err
		}
		median := Median(reading.ChamberC)
		if !domain.IsFinite(median) {
			return nil, domain.NewInputError(domain.CodeDataUnrecoverable, "chamber median is non-finite")
		}
		deviation := math.Abs(median - expected)
		if deviation > req.SteamToleranceC {
			if open == nil {
				open = &domain.Finding{RuleCode: "STEAM_DEVIATION", ProbeOrArea: "chamber", StartNanos: reading.AtNanos, FirstFailedNanos: reading.AtNanos, Threshold: req.SteamToleranceC, Unit: "C", Severity: "fail"}
			}
			open.EndNanos = reading.AtNanos
			if deviation > open.Measured {
				open.Measured = deviation
				open.Margin = req.SteamToleranceC - deviation
			}
			continue
		}
		if open != nil {
			if open.EndNanos-open.StartNanos > req.SteamAllowedNanos {
				findings = append(findings, *open)
			}
			open = nil
		}
	}
	if open != nil && open.EndNanos-open.StartNanos > req.SteamAllowedNanos {
		findings = append(findings, *open)
	}
	return findings, nil
}

func Median(values []float64) float64 {
	if len(values) == 0 {
		return math.NaN()
	}
	cp := append([]float64(nil), values...)
	sort.Float64s(cp)
	mid := len(cp) / 2
	if len(cp)%2 == 1 {
		return cp[mid]
	}
	return (cp[mid-1] + cp[mid]) / 2
}

func ChamberSpreadFinding(reading GridReading, req domain.Requirements) *domain.Finding {
	if len(reading.ChamberTempsC) < 2 {
		return nil
	}
	minimum, maximum := reading.ChamberTempsC[0], reading.ChamberTempsC[0]
	for _, value := range reading.ChamberTempsC[1:] {
		if value < minimum {
			minimum = value
		}
		if value > maximum {
			maximum = value
		}
	}
	spread := maximum - minimum
	margin := req.SpreadMaxC - spread
	if margin >= 0 {
		return nil
	}
	return &domain.Finding{RuleCode: "SPREAD_HIGH", ProbeOrArea: "chamber", StartNanos: reading.AtNanos, EndNanos: reading.AtNanos, FirstFailedNanos: reading.AtNanos, Measured: spread, Threshold: req.SpreadMaxC, Margin: margin, Unit: "C", Severity: "fail"}
}
