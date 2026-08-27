package workflow_test

import (
	"testing"
	"time"

	"steam-sterilization-thermal-validation/internal/domain"
	"steam-sterilization-thermal-validation/internal/store"
	"steam-sterilization-thermal-validation/internal/workflow"
)

type fixedClock struct{}

func (fixedClock) Now() time.Time { return time.Unix(1, 0).UTC() }

func TestApplyTransitionRejectsOldVersion(t *testing.T) {
	rev := domain.Revision{ID: "rev-1", State: domain.StateDraft, Version: 2}
	if _, _, err := workflow.ApplyTransition(rev, 1, domain.StateFrozen, "freeze"); err == nil {
		t.Fatal("expected version conflict")
	}
}

func TestServiceCreateRunIsIdempotentByBusinessKey(t *testing.T) {
	svc := workflow.NewService(store.NewMemoryRepository(), fixedClock{})
	a, err := svc.CreateRun("device-1", "run-1")
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.CreateRun("device-1", "run-1")
	if err != nil {
		t.Fatal(err)
	}
	if a.ID != b.ID {
		t.Fatalf("expected same revision id, got %s and %s", a.ID, b.ID)
	}
}
