package series

import (
	"sort"

	"steam-sterilization-thermal-validation/internal/domain"
)

func PointsForProbe(samples []domain.SensorSample, probeID string, kind domain.SampleKind) []Point {
	var points []Point
	for _, sample := range samples {
		if sample.ProbeID == probeID && sample.Kind == kind {
			points = append(points, Point{AtNanos: sample.AtNanos, Value: sample.Value})
		}
	}
	sort.Slice(points, func(i, j int) bool { return points[i].AtNanos < points[j].AtNanos })
	return points
}

func CoverageGaps(points []Point, startNanos, endNanos, maxGapNanos int64) []domain.Finding {
	var findings []domain.Finding
	if len(points) == 0 {
		return []domain.Finding{{RuleCode: "COVERAGE_GAP", ProbeOrArea: "unknown", StartNanos: startNanos, EndNanos: endNanos, Margin: -1, Unit: "ns", Severity: "input"}}
	}
	if points[0].AtNanos > startNanos {
		findings = append(findings, domain.Finding{RuleCode: "COVERAGE_GAP", StartNanos: startNanos, EndNanos: points[0].AtNanos, Measured: float64(points[0].AtNanos - startNanos), Threshold: 0, Margin: float64(startNanos - points[0].AtNanos), Unit: "ns", Severity: "input"})
	}
	for i := 0; i+1 < len(points); i++ {
		gap := points[i+1].AtNanos - points[i].AtNanos
		if gap > maxGapNanos {
			findings = append(findings, domain.Finding{RuleCode: "COVERAGE_GAP", StartNanos: points[i].AtNanos, EndNanos: points[i+1].AtNanos, Measured: float64(gap), Threshold: float64(maxGapNanos), Margin: float64(maxGapNanos - gap), Unit: "ns", Severity: "input"})
		}
	}
	if points[len(points)-1].AtNanos < endNanos {
		findings = append(findings, domain.Finding{RuleCode: "COVERAGE_GAP", StartNanos: points[len(points)-1].AtNanos, EndNanos: endNanos, Measured: float64(endNanos - points[len(points)-1].AtNanos), Threshold: 0, Margin: float64(points[len(points)-1].AtNanos - endNanos), Unit: "ns", Severity: "input"})
	}
	return findings
}

func UnionGrid(base []int64, additions ...[]int64) []int64 {
	set := map[int64]struct{}{}
	for _, t := range base {
		set[t] = struct{}{}
	}
	for _, list := range additions {
		for _, t := range list {
			set[t] = struct{}{}
		}
	}
	out := make([]int64, 0, len(set))
	for t := range set {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func Times(points []Point) []int64 {
	out := make([]int64, len(points))
	for i, point := range points {
		out[i] = point.AtNanos
	}
	return out
}

func InterpolatedCurve(points []Point, grid []int64, maxGapNanos int64) ([]Point, error) {
	out := make([]Point, 0, len(grid))
	for _, at := range grid {
		value, err := Interpolate(points, at, maxGapNanos)
		if err != nil {
			return nil, err
		}
		out = append(out, Point{AtNanos: at, Value: value})
	}
	return out, nil
}
