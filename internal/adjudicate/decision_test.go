package adjudicate_test

import (
	"testing"

	"steam-sterilization-thermal-validation/internal/adjudicate"
	"steam-sterilization-thermal-validation/internal/domain"
)

func TestDecidePassesWhenMarginsAreZero(t *testing.T) {
	segment := &domain.ExposureSegment{StartNanos: 0, EndNanos: 10, DurationNanos: 10}
	metrics := []domain.ProbeMetric{{ProbeID: "load-1", EquivalentMinutes: 5}}
	result := adjudicate.Decide(segment, metrics, domain.Requirements{ExposureMinNanos: 10, MinLethalityMinutes: 5}, nil)
	if result.Conclusion != domain.ConclusionPass || len(result.Findings) != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestStableSortFindingsAppliesRulePriority(t *testing.T) {
	findings := []domain.Finding{{RuleCode: "LETHALITY_LOW", ProbeOrArea: "b"}, {RuleCode: "NO_EXPOSURE", ProbeOrArea: "run"}}
	got := adjudicate.StableSortFindings(findings)
	if got[0].RuleCode != "NO_EXPOSURE" || got[0].Ordinal != 1 || got[1].Ordinal != 2 {
		t.Fatalf("unexpected order: %+v", got)
	}
}
