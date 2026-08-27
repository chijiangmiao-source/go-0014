package workflow

import (
	"fmt"
	"time"

	"steam-sterilization-thermal-validation/internal/domain"
	"steam-sterilization-thermal-validation/internal/thermal"
)

type RecoveryRepository interface {
	GetRevision(id string) (domain.Revision, error)
	SaveRevision(domain.Revision, domain.StateEvent) error
	Snapshot(id string) (domain.Snapshot, error)
	SaveJob(domain.AnalysisJob) error
	ExpiredJobs(nowNanos int64) ([]domain.AnalysisJob, error)
}

func (s *Service) RecoverExpiredLeases(holder string) (int, error) {
	repo, ok := s.repo.(RecoveryRepository)
	if !ok {
		return 0, nil
	}
	now := s.clock.Now()
	jobs, err := repo.ExpiredJobs(now.UnixNano())
	if err != nil {
		return 0, err
	}
	claimed := 0
	for _, job := range jobs {
		if job.CancelRequested {
			job.Status = domain.JobCanceled
			_ = repo.SaveJob(job)
			continue
		}
		job.Attempt++
		job.Status = domain.JobRunning
		job.LeaseHolder = holder
		job.LeaseUntilNanos = now.Add(30 * time.Second).UnixNano()
		if err := repo.SaveJob(job); err != nil {
			return claimed, err
		}
		if err := s.recomputeClaimedJob(repo, job); err != nil {
			job.Status = domain.JobFailed
			if boundary, ok := err.(*domain.BoundaryError); ok {
				job.ErrorCode = boundary.Code
			} else {
				job.ErrorCode = domain.CodeDataUnrecoverable
			}
			_ = repo.SaveJob(job)
			return claimed, err
		}
		claimed++
	}
	return claimed, nil
}

func (s *Service) recomputeClaimedJob(repo RecoveryRepository, job domain.AnalysisJob) error {
	revision, err := repo.GetRevision(job.RevisionID)
	if err != nil {
		return err
	}
	if revision.State != domain.StateAnalyzing {
		job.Status = domain.JobCanceled
		return repo.SaveJob(job)
	}
	snapshot, err := repo.Snapshot(job.RevisionID)
	if err != nil {
		return err
	}
	result, err := thermal.AnalyzeSnapshot(snapshot)
	if err != nil {
		return err
	}
	next, event, err := ApplyTransition(revision, revision.Version, domain.StateResultReady, "recover_complete")
	if err != nil {
		return err
	}
	next.Result = &result
	next.Conclusion = result.Conclusion
	if err := repo.SaveRevision(next, event); err != nil {
		return err
	}
	job.DoneSteps = job.TotalSteps
	job.Status = domain.JobSucceeded
	return repo.SaveJob(job)
}

func LeaseID(prefix string, now time.Time) string {
	if prefix == "" {
		prefix = "worker"
	}
	return fmt.Sprintf("%s-%d", prefix, now.UTC().UnixNano())
}
