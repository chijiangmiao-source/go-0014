package workflow

import (
	"fmt"
	"time"

	"steam-sterilization-thermal-validation/internal/domain"
	"steam-sterilization-thermal-validation/internal/evidence"
	"steam-sterilization-thermal-validation/internal/ingest"
	"steam-sterilization-thermal-validation/internal/thermal"
)

type Clock interface {
	Now() time.Time
}

type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now().UTC() }

type Repository interface {
	CreateRevision(deviceID, businessKey string) (domain.Revision, error)
	GetRevision(id string) (domain.Revision, error)
	SaveRevision(domain.Revision, domain.StateEvent) error
	Events(id string) ([]domain.StateEvent, error)
	UpdateInputs(id string, expectedVersion int64, mutate func(*domain.DraftInputs) error) (domain.Revision, error)
	Inputs(id string) (domain.DraftInputs, error)
	SaveSnapshot(id string, snapshot domain.Snapshot) error
	Snapshot(id string) (domain.Snapshot, error)
	SaveJob(job domain.AnalysisJob) error
	Job(id string) (domain.AnalysisJob, error)
	JobForRevision(id string) (domain.AnalysisJob, error)
	SaveEvidence(domain.EvidencePackage) error
	Evidence(id string) (domain.EvidencePackage, error)
	SaveReplay(domain.ReplayComparison) error
	Replay(id string) (domain.ReplayComparison, error)
}

type Service struct {
	repo  Repository
	clock Clock
}

func NewService(repo Repository, clock Clock) *Service {
	return &Service{repo: repo, clock: clock}
}

func (s *Service) CreateRun(deviceID, businessKey string) (domain.Revision, error) {
	if deviceID == "" || businessKey == "" {
		return domain.Revision{}, domain.NewInputError(domain.CodeInvalidInput, "device_id and business_key are required")
	}
	return s.repo.CreateRevision(deviceID, businessKey)
}

func (s *Service) Freeze(id string, expectedVersion int64, snapshot domain.Snapshot) (domain.Revision, error) {
	revision, err := s.repo.GetRevision(id)
	if err != nil {
		return domain.Revision{}, err
	}
	sorted, hash, err := ingest.CanonicalSnapshot(snapshot)
	if err != nil {
		return domain.Revision{}, err
	}
	if err := ingest.ValidateProbeSpecs(sorted.Probes); err != nil {
		return domain.Revision{}, err
	}
	if err := ingest.ValidateFreezeReadiness(sorted); err != nil {
		return domain.Revision{}, err
	}
	next, event, err := ApplyTransition(revision, expectedVersion, domain.StateFrozen, "freeze")
	if err != nil {
		return domain.Revision{}, err
	}
	next.SnapshotHash = hash
	next.AlgorithmVersion = sorted.AlgorithmVersion
	if err := s.repo.SaveSnapshot(id, sorted); err != nil {
		return domain.Revision{}, err
	}
	if err := s.repo.SaveRevision(next, event); err != nil {
		return domain.Revision{}, err
	}
	return next, nil
}

func (s *Service) PutProbes(id string, expectedVersion int64, probes []domain.ProbeSpec) (domain.Revision, error) {
	if err := ingest.ValidateProbeSpecs(probes); err != nil {
		return domain.Revision{}, err
	}
	return s.repo.UpdateInputs(id, expectedVersion, func(inputs *domain.DraftInputs) error {
		inputs.Probes = append([]domain.ProbeSpec(nil), probes...)
		return nil
	})
}

func (s *Service) PutCalibrations(id string, expectedVersion int64, calibrations []domain.Calibration) (domain.Revision, error) {
	for _, calibration := range calibrations {
		if calibration.ValidUntilNanos < calibration.ValidFromNanos || calibration.UncertaintyC < 0 {
			return domain.Revision{}, domain.NewInputError(domain.CodeInvalidInput, "calibration validity and uncertainty are invalid", calibration.ProbeID)
		}
		if !domain.IsFinite(calibration.OffsetC) || !domain.IsFinite(calibration.UncertaintyC) {
			return domain.Revision{}, domain.NewInputError(domain.CodeInvalidInput, "calibration values must be finite", calibration.ProbeID)
		}
	}
	return s.repo.UpdateInputs(id, expectedVersion, func(inputs *domain.DraftInputs) error {
		inputs.Calibrations = append([]domain.Calibration(nil), calibrations...)
		return nil
	})
}

func (s *Service) PutSamples(id string, expectedVersion int64, samples []domain.SensorSample) (domain.Revision, error) {
	folded, err := ingest.FoldSamples(samples)
	if err != nil {
		return domain.Revision{}, err
	}
	return s.repo.UpdateInputs(id, expectedVersion, func(inputs *domain.DraftInputs) error {
		merged, err := ingest.MergeSampleBatch(inputs.Samples, folded)
		if err != nil {
			return err
		}
		inputs.Samples = merged
		return nil
	})
}

func (s *Service) PutRequirements(id string, expectedVersion int64, req domain.Requirements) (domain.Revision, error) {
	return s.repo.UpdateInputs(id, expectedVersion, func(inputs *domain.DraftInputs) error {
		req = ingest.CanonicalizeRequirements(req, inputs.Probes)
		if err := ingest.ValidateRequirements(req, inputs.Probes); err != nil {
			return err
		}
		inputs.Requirements = req
		return nil
	})
}

func (s *Service) FreezeStored(id string, expectedVersion int64) (domain.Revision, error) {
	inputs, err := s.repo.Inputs(id)
	if err != nil {
		return domain.Revision{}, err
	}
	snapshot, _, err := ingest.BuildSnapshot(inputs)
	if err != nil {
		return domain.Revision{}, err
	}
	return s.Freeze(id, expectedVersion, snapshot)
}

func (s *Service) StartAnalysis(id string, expectedVersion int64) (domain.Revision, string, error) {
	revision, err := s.repo.GetRevision(id)
	if err != nil {
		return domain.Revision{}, "", err
	}
	job := domain.AnalysisJob{
		ID:              fmt.Sprintf("job-%s-%d", id, s.clock.Now().UnixNano()),
		RevisionID:      id,
		Type:            "analysis",
		Status:          domain.JobRunning,
		LeaseHolder:     "inline-worker",
		LeaseUntilNanos: s.clock.Now().Add(30 * time.Second).UnixNano(),
		Attempt:         1,
		TotalSteps:      5,
	}
	if err := s.repo.SaveJob(job); err != nil {
		return domain.Revision{}, "", err
	}
	next, event, err := ApplyTransition(revision, expectedVersion, domain.StateAnalyzing, "start")
	if err != nil {
		return domain.Revision{}, "", err
	}
	if err := s.repo.SaveRevision(next, event); err != nil {
		return domain.Revision{}, "", err
	}
	snapshot, err := s.repo.Snapshot(id)
	if err != nil {
		job.Status = domain.JobFailed
		job.ErrorCode = domain.CodeInvalidInput
		_ = s.repo.SaveJob(job)
		return next, job.ID, err
	}
	result, err := thermal.AnalyzeSnapshot(snapshot)
	if err != nil {
		job.Status = domain.JobFailed
		if boundary, ok := err.(*domain.BoundaryError); ok {
			job.ErrorCode = boundary.Code
		} else {
			job.ErrorCode = domain.CodeDataUnrecoverable
		}
		_ = s.repo.SaveJob(job)
		return next, job.ID, err
	}
	completed, err := s.Complete(id, next.Version, result)
	if err != nil {
		job.Status = domain.JobFailed
		job.ErrorCode = domain.CodeVersionConflict
		_ = s.repo.SaveJob(job)
		return next, job.ID, err
	}
	job.Status = domain.JobSucceeded
	job.DoneSteps = job.TotalSteps
	_ = s.repo.SaveJob(job)
	return completed, job.ID, nil
}

func (s *Service) Complete(id string, expectedVersion int64, result domain.AnalysisResult) (domain.Revision, error) {
	revision, err := s.repo.GetRevision(id)
	if err != nil {
		return domain.Revision{}, err
	}
	next, event, err := ApplyTransition(revision, expectedVersion, domain.StateResultReady, "complete")
	if err != nil {
		return domain.Revision{}, err
	}
	next.Result = &result
	next.Conclusion = result.Conclusion
	if err := s.repo.SaveRevision(next, event); err != nil {
		return domain.Revision{}, err
	}
	return next, nil
}

func (s *Service) Finalize(id string, expectedVersion int64) (domain.Revision, error) {
	revision, err := s.repo.GetRevision(id)
	if err != nil {
		return domain.Revision{}, err
	}
	if revision.State == domain.StateSealedPass || revision.State == domain.StateSealedFail {
		if revision.Version != expectedVersion {
			return domain.Revision{}, domain.NewStateError(domain.CodeVersionConflict, "revision version does not match expected_version")
		}
		return revision, nil
	}
	if revision.Result == nil {
		return domain.Revision{}, domain.NewStateError(domain.CodeIllegalState, "revision has no result to finalize")
	}
	to := domain.StateSealedFail
	if revision.Conclusion == domain.ConclusionPass {
		to = domain.StateSealedPass
	}
	next, event, err := ApplyTransition(revision, expectedVersion, to, "finalize")
	if err != nil {
		return domain.Revision{}, err
	}
	pkg, err := s.buildEvidence(id, next, event)
	if err != nil {
		return domain.Revision{}, err
	}
	if err := s.repo.SaveRevision(next, event); err != nil {
		return domain.Revision{}, err
	}
	pkg.RevisionID = id
	pkg.Version = next.Version
	if err := s.repo.SaveEvidence(pkg); err != nil {
		return domain.Revision{}, err
	}
	return next, nil
}

func (s *Service) Cancel(id string, expectedVersion int64) (domain.Revision, error) {
	revision, err := s.repo.GetRevision(id)
	if err != nil {
		return domain.Revision{}, err
	}
	next, event, err := ApplyTransition(revision, expectedVersion, domain.StateAborted, "cancel")
	if err != nil {
		return domain.Revision{}, err
	}
	job, _ := s.repo.JobForRevision(id)
	if job.ID != "" {
		job.CancelRequested = true
		job.Status = domain.JobCanceled
		_ = s.repo.SaveJob(job)
	}
	if err := s.repo.SaveRevision(next, event); err != nil {
		return domain.Revision{}, err
	}
	return next, nil
}

func (s *Service) Revision(id string) (domain.Revision, error) {
	return s.repo.GetRevision(id)
}

func (s *Service) Events(id string) ([]domain.StateEvent, error) {
	return s.repo.Events(id)
}

func (s *Service) Job(id string) (domain.AnalysisJob, error) {
	return s.repo.JobForRevision(id)
}

func (s *Service) Evidence(id string) (domain.EvidencePackage, error) {
	pkg, err := s.repo.Evidence(id)
	if err != nil {
		return domain.EvidencePackage{}, err
	}
	if pkg.Length != len(pkg.Bytes) || pkg.Hash != evidence.SHA256Hex(pkg.Bytes) || !pkg.Complete {
		return domain.EvidencePackage{}, domain.NewInputError(domain.CodeEvidenceCorrupt, "evidence package failed integrity check")
	}
	return pkg, nil
}

func (s *Service) Replay(id string) (domain.ReplayComparison, error) {
	revision, err := s.repo.GetRevision(id)
	if err != nil {
		return domain.ReplayComparison{}, err
	}
	snapshot, err := s.repo.Snapshot(id)
	if err != nil {
		return domain.ReplayComparison{}, err
	}
	result, err := thermal.AnalyzeSnapshot(snapshot)
	if err != nil {
		return domain.ReplayComparison{}, err
	}
	events, err := s.repo.Events(id)
	if err != nil {
		return domain.ReplayComparison{}, err
	}
	pkg, err := evidence.Build(revision.SnapshotHash, revision.AlgorithmVersion, result, events)
	if err != nil {
		return domain.ReplayComparison{}, err
	}
	original := ""
	if sealed, err := s.repo.Evidence(id); err == nil {
		original = sealed.Hash
	}
	replay := domain.ReplayComparison{
		TaskID:            fmt.Sprintf("replay-%s-%d", id, s.clock.Now().UnixNano()),
		RevisionID:        id,
		Result:            result,
		OriginalHash:      original,
		ReplayedHash:      pkg.Hash,
		ConclusionMatches: result.Conclusion == revision.Conclusion,
		HashMatches:       original != "" && original == pkg.Hash,
		MetricCount:       len(result.Metrics),
		FindingCount:      len(result.Findings),
	}
	if err := s.repo.SaveReplay(replay); err != nil {
		return domain.ReplayComparison{}, err
	}
	return replay, nil
}

func (s *Service) ReplayResult(taskID string) (domain.ReplayComparison, error) {
	return s.repo.Replay(taskID)
}

func (s *Service) buildEvidence(id string, revision domain.Revision, finalEvent domain.StateEvent) (domain.EvidencePackage, error) {
	events, err := s.repo.Events(id)
	if err != nil {
		return domain.EvidencePackage{}, err
	}
	finalEvent.Ordinal = len(events) + 1
	events = append(events, finalEvent)
	result := domain.AnalysisResult{}
	if revision.Result != nil {
		result = *revision.Result
	}
	pkg, err := evidence.Build(revision.SnapshotHash, revision.AlgorithmVersion, result, events)
	if err != nil {
		return domain.EvidencePackage{}, err
	}
	return domain.EvidencePackage{RevisionID: id, Bytes: pkg.Bytes, Hash: pkg.Hash, Length: pkg.Length, Complete: true, Version: revision.Version}, nil
}
