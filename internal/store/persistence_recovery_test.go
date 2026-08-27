package store_test

import (
	"path/filepath"
	"testing"

	"steam-sterilization-thermal-validation/internal/domain"
	"steam-sterilization-thermal-validation/internal/store"
)

func TestSQLiteRepositoryPersistsRevisionAcrossRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runs.db")
	first, err := store.OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	created, err := first.CreateRevision("autoclave-1", "run-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := store.OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	got, err := second.GetRevision(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != created.ID || got.State != domain.StateDraft {
		t.Fatalf("unexpected persisted revision: %+v", got)
	}
}

func TestMemoryRepositoryReturnsExpiredRunningJobsOnce(t *testing.T) {
	repo := store.NewMemoryRepository()
	revision, err := repo.CreateRevision("dev", "run")
	if err != nil {
		t.Fatal(err)
	}
	job := domain.AnalysisJob{ID: "job-1", RevisionID: revision.ID, Type: "analysis", Status: domain.JobRunning, LeaseUntilNanos: 10}
	if err := repo.SaveJob(job); err != nil {
		t.Fatal(err)
	}
	jobs, err := repo.ExpiredJobs(20)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 || jobs[0].ID != "job-1" {
		t.Fatalf("unexpected expired jobs: %+v", jobs)
	}
}
