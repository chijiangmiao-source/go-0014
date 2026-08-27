package ingest

import (
	"sort"
	"strings"

	"steam-sterilization-thermal-validation/internal/domain"
)

type RawSample struct {
	ProbeID       string
	Time          string
	Kind          domain.SampleKind
	Value         float64
	Unit          string
	SourceOrdinal int
}

func NormalizeSample(raw RawSample) (domain.SensorSample, error) {
	if raw.ProbeID == "" {
		return domain.SensorSample{}, domain.NewInputError(domain.CodeInvalidInput, "probe id is required")
	}
	if !domain.IsFinite(raw.Value) {
		return domain.SensorSample{}, domain.NewInputError(domain.CodeInvalidInput, "sample value must be finite", raw.ProbeID)
	}
	at, err := domain.ParseRFC3339NanoUTC(raw.Time)
	if err != nil {
		return domain.SensorSample{}, err
	}
	value, unit, err := normalizeValue(raw.Kind, raw.Value, raw.Unit)
	if err != nil {
		return domain.SensorSample{}, err
	}
	return domain.SensorSample{ProbeID: raw.ProbeID, AtNanos: at, Kind: raw.Kind, Value: value, Unit: unit, SourceOrdinal: raw.SourceOrdinal}, nil
}

func FoldSamples(samples []domain.SensorSample) ([]domain.SensorSample, error) {
	cp := append([]domain.SensorSample(nil), samples...)
	sort.SliceStable(cp, func(i, j int) bool {
		if cp[i].ProbeID != cp[j].ProbeID {
			return cp[i].ProbeID < cp[j].ProbeID
		}
		if cp[i].AtNanos != cp[j].AtNanos {
			return cp[i].AtNanos < cp[j].AtNanos
		}
		return cp[i].SourceOrdinal < cp[j].SourceOrdinal
	})
	out := make([]domain.SensorSample, 0, len(cp))
	for _, sample := range cp {
		if len(out) == 0 {
			out = append(out, sample)
			continue
		}
		last := &out[len(out)-1]
		if last.ProbeID == sample.ProbeID && last.AtNanos == sample.AtNanos {
			if last.Kind != sample.Kind || last.Unit != sample.Unit || last.Value != sample.Value {
				return nil, domain.NewInputError(domain.CodeSampleConflict, "conflicting sample at same probe and time", sample.ProbeID)
			}
			continue
		}
		out = append(out, sample)
	}
	return out, nil
}

func normalizeValue(kind domain.SampleKind, value float64, unit string) (float64, string, error) {
	switch kind {
	case domain.SampleTemperature:
		switch strings.ToLower(strings.TrimSpace(unit)) {
		case "c", "degc", "celsius":
			return value, "C", nil
		case "f", "degf", "fahrenheit":
			return (value - 32) * 5 / 9, "C", nil
		default:
			return 0, "", domain.NewInputError(domain.CodeInvalidInput, "temperature unit must be explicit", unit)
		}
	case domain.SamplePressure:
		switch strings.ToLower(strings.TrimSpace(unit)) {
		case "kpa_abs", "kpa-a", "absolute_kpa":
			return value, "kPa_abs", nil
		default:
			return 0, "", domain.NewInputError(domain.CodeInvalidInput, "pressure unit must be absolute kPa", unit)
		}
	default:
		return 0, "", domain.NewInputError(domain.CodeInvalidInput, "sample kind is not supported")
	}
}
