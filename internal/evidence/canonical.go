package evidence

import (
	"math"

	"steam-sterilization-thermal-validation/internal/domain"
)

func CanonicalResult(result domain.AnalysisResult) (domain.AnalysisResult, error) {
	out := result
	if out.Segment != nil {
		value, err := canonicalFloat(out.Segment.ColdPointC)
		if err != nil {
			return domain.AnalysisResult{}, err
		}
		out.Segment.ColdPointC = value
	}
	for i := range out.Metrics {
		minimum, err := canonicalFloat(out.Metrics[i].MinimumC)
		if err != nil {
			return domain.AnalysisResult{}, err
		}
		maximum, err := canonicalFloat(out.Metrics[i].MaximumC)
		if err != nil {
			return domain.AnalysisResult{}, err
		}
		minutes, err := canonicalFloat(out.Metrics[i].EquivalentMinutes)
		if err != nil {
			return domain.AnalysisResult{}, err
		}
		out.Metrics[i].MinimumC = minimum
		out.Metrics[i].MaximumC = maximum
		out.Metrics[i].EquivalentMinutes = minutes
	}
	for i := range out.Findings {
		measured, err := canonicalFloat(out.Findings[i].Measured)
		if err != nil {
			return domain.AnalysisResult{}, err
		}
		threshold, err := canonicalFloat(out.Findings[i].Threshold)
		if err != nil {
			return domain.AnalysisResult{}, err
		}
		margin, err := canonicalFloat(out.Findings[i].Margin)
		if err != nil {
			return domain.AnalysisResult{}, err
		}
		out.Findings[i].Measured = measured
		out.Findings[i].Threshold = threshold
		out.Findings[i].Margin = margin
	}
	return out, nil
}

func canonicalFloat(value float64) (float64, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, domain.NewInputError(domain.CodeDataUnrecoverable, "evidence contains non-finite number")
	}
	if value == 0 {
		return 0, nil
	}
	return value, nil
}
