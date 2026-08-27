package evidence_test

import (
	"testing"

	"steam-sterilization-thermal-validation/internal/domain"
	"steam-sterilization-thermal-validation/internal/evidence"
)

func TestBuildEvidenceIsDeterministic(t *testing.T) {
	result := domain.AnalysisResult{Conclusion: domain.ConclusionPass}
	a, err := evidence.Build("abc", "foundation-1", result, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := evidence.Build("abc", "foundation-1", result, nil)
	if err != nil {
		t.Fatal(err)
	}
	if a.Hash != b.Hash || string(a.Bytes) != string(b.Bytes) {
		t.Fatalf("evidence is not deterministic")
	}
	if err := evidence.Verify(a); err != nil {
		t.Fatal(err)
	}
}
