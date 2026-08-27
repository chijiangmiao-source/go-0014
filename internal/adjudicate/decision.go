package adjudicate

import "steam-sterilization-thermal-validation/internal/domain"

func Decide(segment *domain.ExposureSegment, metrics []domain.ProbeMetric, req domain.Requirements, extra []domain.Finding) domain.AnalysisResult {
	findings := append([]domain.Finding(nil), extra...)
	if segment == nil {
		findings = append(findings, domain.Finding{RuleCode: "NO_EXPOSURE", ProbeOrArea: "run", Margin: -1, Unit: "segment", Severity: "fail"})
	} else if margin := minimumMargin(float64(segment.DurationNanos), float64(req.ExposureMinNanos)); margin < 0 {
		findings = append(findings, domain.Finding{RuleCode: "EXPOSURE_SHORT", ProbeOrArea: "run", StartNanos: segment.StartNanos, EndNanos: segment.EndNanos, Measured: float64(segment.DurationNanos), Threshold: float64(req.ExposureMinNanos), Margin: margin, Unit: "ns", Severity: "fail"})
	}
	for _, metric := range metrics {
		if margin := minimumMargin(metric.EquivalentMinutes, req.MinLethalityMinutes); margin < 0 {
			findings = append(findings, domain.Finding{RuleCode: "LETHALITY_LOW", ProbeOrArea: metric.ProbeID, Measured: metric.EquivalentMinutes, Threshold: req.MinLethalityMinutes, Margin: margin, Unit: "equiv_min", Severity: "fail"})
		}
	}
	findings = StableSortFindings(findings)
	conclusion := domain.ConclusionPass
	if len(findings) > 0 {
		conclusion = domain.ConclusionFail
	}
	return domain.AnalysisResult{Segment: segment, Metrics: metrics, Findings: findings, Conclusion: conclusion}
}
