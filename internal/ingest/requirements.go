package ingest

import (
	"slices"

	"steam-sterilization-thermal-validation/internal/domain"
)

const AlgorithmVersion = "steam-validation-v1"

func DefaultRequirements(start, end int64) domain.Requirements {
	return domain.Requirements{
		RunStartNanos:       start,
		RunEndNanos:         end,
		SampleStepNanos:     int64(30 * 1e9),
		MaxGapNanos:         int64(90 * 1e9),
		ExposureMinC:        121,
		ExposureMinNanos:    int64(8 * 60 * 1e9),
		ConfirmNanos:        int64(60 * 1e9),
		GraceNanos:          int64(30 * 1e9),
		SpreadMaxC:          2,
		MinLethalityMinutes: 8,
		SteamToleranceC:     3,
		SteamAllowedNanos:   int64(60 * 1e9),
		TRefC:               121,
		ZC:                  10,
		SteamTableVersion:   "frozen-demo-2026a",
		FrozenSteamTable: []domain.SteamPoint{
			{PressureKPa: 101.325, SaturatedC: 100},
			{PressureKPa: 143.27, SaturatedC: 110},
			{PressureKPa: 198.67, SaturatedC: 120},
			{PressureKPa: 205.0, SaturatedC: 121},
			{PressureKPa: 211.0, SaturatedC: 122},
			{PressureKPa: 232.1, SaturatedC: 125},
		},
	}
}

func ValidateRequirements(req domain.Requirements, probes []domain.ProbeSpec) error {
	if req.RunEndNanos <= req.RunStartNanos {
		return domain.NewInputError(domain.CodeInvalidInput, "run_end must be after run_start")
	}
	if req.SampleStepNanos <= 0 || req.MaxGapNanos <= 0 {
		return domain.NewInputError(domain.CodeInvalidInput, "sample step and max gap must be positive")
	}
	if req.ExposureMinNanos < 0 || req.ConfirmNanos < 0 || req.GraceNanos < 0 || req.SteamAllowedNanos < 0 {
		return domain.NewInputError(domain.CodeInvalidInput, "duration requirements must be non-negative")
	}
	numbers := []float64{req.ExposureMinC, req.SpreadMaxC, req.MinLethalityMinutes, req.SteamToleranceC, req.TRefC, req.ZC}
	for _, value := range numbers {
		if !domain.IsFinite(value) {
			return domain.NewInputError(domain.CodeInvalidInput, "requirements contain non-finite value")
		}
	}
	if req.ZC <= 0 {
		return domain.NewInputError(domain.CodeInvalidInput, "z value must be positive")
	}
	required := map[string]domain.Role{}
	for _, probe := range probes {
		if probe.Required {
			required[probe.ProbeID] = probe.Role
		}
	}
	if len(req.RequiredProbeIDs) == 0 {
		for id, role := range required {
			if role == domain.RoleLoad {
				req.RequiredProbeIDs = append(req.RequiredProbeIDs, id)
			}
		}
	}
	for _, id := range req.RequiredProbeIDs {
		role, ok := required[id]
		if !ok {
			return domain.NewInputError(domain.CodeInvalidInput, "required probe is not declared as required", id)
		}
		if role != domain.RoleLoad {
			return domain.NewInputError(domain.CodeInvalidInput, "required exposure probe must be load role", id)
		}
	}
	if err := validateSteamTable(req.FrozenSteamTable); err != nil {
		return err
	}
	return nil
}

func CanonicalizeRequirements(req domain.Requirements, probes []domain.ProbeSpec) domain.Requirements {
	if req.SteamTableVersion == "" {
		req.SteamTableVersion = "frozen-inline"
	}
	if req.RequiredProbeIDs == nil {
		for _, probe := range probes {
			if probe.Required && probe.Role == domain.RoleLoad {
				req.RequiredProbeIDs = append(req.RequiredProbeIDs, probe.ProbeID)
			}
		}
	}
	slices.Sort(req.RequiredProbeIDs)
	return req
}

func validateSteamTable(table []domain.SteamPoint) error {
	if len(table) < 2 {
		return domain.NewInputError(domain.CodeInvalidInput, "steam table requires at least two points")
	}
	for i, point := range table {
		if !domain.IsFinite(point.PressureKPa) || !domain.IsFinite(point.SaturatedC) {
			return domain.NewInputError(domain.CodeInvalidInput, "steam table values must be finite")
		}
		if i > 0 {
			prev := table[i-1]
			if point.PressureKPa <= prev.PressureKPa {
				return domain.NewInputError(domain.CodeInvalidInput, "steam table pressure must strictly increase")
			}
			if point.SaturatedC < prev.SaturatedC {
				return domain.NewInputError(domain.CodeInvalidInput, "steam table temperature must be monotonic")
			}
		}
	}
	return nil
}
