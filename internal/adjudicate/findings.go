package adjudicate

import (
	"sort"

	"steam-sterilization-thermal-validation/internal/domain"
)

var priority = map[string]int{
	"NO_EXPOSURE":     10,
	"EXPOSURE_SHORT":  20,
	"LETHALITY_LOW":   30,
	"SPREAD_HIGH":     40,
	"STEAM_DEVIATION": 50,
}

func StableSortFindings(findings []domain.Finding) []domain.Finding {
	out := append([]domain.Finding(nil), findings...)
	sort.SliceStable(out, func(i, j int) bool {
		pi, pj := priority[out[i].RuleCode], priority[out[j].RuleCode]
		if pi != pj {
			return pi < pj
		}
		if out[i].ProbeOrArea != out[j].ProbeOrArea {
			return out[i].ProbeOrArea < out[j].ProbeOrArea
		}
		if out[i].StartNanos != out[j].StartNanos {
			return out[i].StartNanos < out[j].StartNanos
		}
		return out[i].EndNanos < out[j].EndNanos
	})
	for i := range out {
		out[i].Ordinal = i + 1
	}
	return out
}

func minimumMargin(measured, threshold float64) float64 {
	return measured - threshold
}
