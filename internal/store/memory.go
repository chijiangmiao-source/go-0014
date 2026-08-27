package store

import (
	"fmt"
	"sync"

	"steam-sterilization-thermal-validation/internal/domain"
)

type MemoryRepository struct {
	mu      sync.Mutex
	nextID  int
	records map[string]RevisionRecord
	byKey   map[string]string
	events  map[string][]domain.StateEvent
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{records: map[string]RevisionRecord{}, byKey: map[string]string{}, events: map[string][]domain.StateEvent{}}
}

func (m *MemoryRepository) CreateRevision(deviceID, businessKey string) (domain.Revision, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.byKey[businessKey]; ok {
		return m.records[existing].Revision, nil
	}
	m.nextID++
	id := fmt.Sprintf("rev-%06d", m.nextID)
	revision := domain.Revision{ID: id, State: domain.StateDraft, Version: 1}
	m.records[id] = RevisionRecord{
		Revision:    revision,
		DeviceID:    deviceID,
		BusinessKey: businessKey,
		Inputs:      domain.DraftInputs{Run: domain.RunDescriptor{DeviceID: deviceID, BusinessKey: businessKey}},
		Jobs:        map[string]domain.AnalysisJob{},
		Replays:     map[string]domain.ReplayComparison{},
	}
	m.byKey[businessKey] = id
	m.events[id] = []domain.StateEvent{{Ordinal: 1, From: "", To: domain.StateDraft, ReasonCode: "create", ResultVersion: revision.Version}}
	return revision, nil
}

func (m *MemoryRepository) UpdateInputs(id string, expectedVersion int64, mutate func(*domain.DraftInputs) error) (domain.Revision, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.records[id]
	if !ok {
		return domain.Revision{}, domain.NewInputError(domain.CodeInvalidInput, "revision not found", id)
	}
	if record.Revision.State != domain.StateDraft {
		return domain.Revision{}, domain.NewStateError(domain.CodeIllegalState, "inputs can only change while draft")
	}
	if record.Revision.Version != expectedVersion {
		return domain.Revision{}, domain.NewStateError(domain.CodeVersionConflict, "revision version does not match expected_version")
	}
	if err := mutate(&record.Inputs); err != nil {
		return domain.Revision{}, err
	}
	record.Revision.Version++
	m.records[id] = record
	event := domain.StateEvent{Ordinal: len(m.events[id]) + 1, From: domain.StateDraft, To: domain.StateDraft, ReasonCode: "input_update", ResultVersion: record.Revision.Version}
	m.events[id] = append(m.events[id], event)
	return record.Revision, nil
}

func (m *MemoryRepository) Inputs(id string) (domain.DraftInputs, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.records[id]
	if !ok {
		return domain.DraftInputs{}, domain.NewInputError(domain.CodeInvalidInput, "revision not found", id)
	}
	cp := cloneInputs(record.Inputs)
	return cp, nil
}

func (m *MemoryRepository) SaveSnapshot(id string, snapshot domain.Snapshot) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.records[id]
	if !ok {
		return domain.NewInputError(domain.CodeInvalidInput, "revision not found", id)
	}
	record.Snapshot = cloneSnapshot(snapshot)
	m.records[id] = record
	return nil
}

func (m *MemoryRepository) Snapshot(id string) (domain.Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.records[id]
	if !ok {
		return domain.Snapshot{}, domain.NewInputError(domain.CodeInvalidInput, "revision not found", id)
	}
	if record.Snapshot.AlgorithmVersion == "" {
		return domain.Snapshot{}, domain.NewInputError(domain.CodeInvalidInput, "revision is not frozen", id)
	}
	return cloneSnapshot(record.Snapshot), nil
}

func (m *MemoryRepository) SaveJob(job domain.AnalysisJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.records[job.RevisionID]
	if !ok {
		return domain.NewInputError(domain.CodeInvalidInput, "revision not found", job.RevisionID)
	}
	if record.Jobs == nil {
		record.Jobs = map[string]domain.AnalysisJob{}
	}
	for _, existing := range record.Jobs {
		active := existing.Status == domain.JobQueued || existing.Status == domain.JobRunning
		same := existing.ID == job.ID
		if active && !same && existing.Type == job.Type {
			return domain.NewStateError(domain.CodeIllegalState, "revision already has active job")
		}
	}
	record.Jobs[job.ID] = job
	m.records[job.RevisionID] = record
	return nil
}

func (m *MemoryRepository) Job(id string) (domain.AnalysisJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, record := range m.records {
		if job, ok := record.Jobs[id]; ok {
			return job, nil
		}
	}
	return domain.AnalysisJob{}, domain.NewInputError(domain.CodeInvalidInput, "job not found", id)
}

func (m *MemoryRepository) JobForRevision(id string) (domain.AnalysisJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.records[id]
	if !ok {
		return domain.AnalysisJob{}, domain.NewInputError(domain.CodeInvalidInput, "revision not found", id)
	}
	var latest domain.AnalysisJob
	for _, job := range record.Jobs {
		if latest.ID == "" || job.Attempt >= latest.Attempt {
			latest = job
		}
	}
	if latest.ID == "" {
		return domain.AnalysisJob{}, domain.NewInputError(domain.CodeInvalidInput, "job not found", id)
	}
	return latest, nil
}

func (m *MemoryRepository) SaveEvidence(pkg domain.EvidencePackage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.records[pkg.RevisionID]
	if !ok {
		return domain.NewInputError(domain.CodeInvalidInput, "revision not found", pkg.RevisionID)
	}
	cp := pkg
	cp.Bytes = append([]byte(nil), pkg.Bytes...)
	record.Evidence = &cp
	m.records[pkg.RevisionID] = record
	return nil
}

func (m *MemoryRepository) Evidence(id string) (domain.EvidencePackage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.records[id]
	if !ok {
		return domain.EvidencePackage{}, domain.NewInputError(domain.CodeInvalidInput, "revision not found", id)
	}
	if record.Evidence == nil || !record.Evidence.Complete {
		return domain.EvidencePackage{}, domain.NewInputError(domain.CodeInvalidInput, "evidence is not sealed", id)
	}
	cp := *record.Evidence
	cp.Bytes = append([]byte(nil), record.Evidence.Bytes...)
	return cp, nil
}

func (m *MemoryRepository) SaveReplay(replay domain.ReplayComparison) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.records[replay.RevisionID]
	if !ok {
		return domain.NewInputError(domain.CodeInvalidInput, "revision not found", replay.RevisionID)
	}
	if record.Replays == nil {
		record.Replays = map[string]domain.ReplayComparison{}
	}
	record.Replays[replay.TaskID] = replay
	m.records[replay.RevisionID] = record
	return nil
}

func (m *MemoryRepository) Replay(id string) (domain.ReplayComparison, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, record := range m.records {
		if replay, ok := record.Replays[id]; ok {
			return replay, nil
		}
	}
	return domain.ReplayComparison{}, domain.NewInputError(domain.CodeInvalidInput, "replay not found", id)
}

func (m *MemoryRepository) ExpiredJobs(nowNanos int64) ([]domain.AnalysisJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var jobs []domain.AnalysisJob
	for _, record := range m.records {
		for _, job := range record.Jobs {
			if job.Status == domain.JobRunning && job.LeaseUntilNanos > 0 && job.LeaseUntilNanos < nowNanos {
				jobs = append(jobs, job)
			}
		}
	}
	return jobs, nil
}

func cloneInputs(in domain.DraftInputs) domain.DraftInputs {
	return domain.DraftInputs{
		Probes:       append([]domain.ProbeSpec(nil), in.Probes...),
		Calibrations: append([]domain.Calibration(nil), in.Calibrations...),
		Samples:      append([]domain.SensorSample(nil), in.Samples...),
		Requirements: in.Requirements,
		Run:          in.Run,
	}
}

func cloneSnapshot(in domain.Snapshot) domain.Snapshot {
	out := in
	out.Probes = append([]domain.ProbeSpec(nil), in.Probes...)
	out.Calibrations = append([]domain.Calibration(nil), in.Calibrations...)
	out.Samples = append([]domain.SensorSample(nil), in.Samples...)
	out.Requirements.RequiredProbeIDs = append([]string(nil), in.Requirements.RequiredProbeIDs...)
	out.Requirements.FrozenSteamTable = append([]domain.SteamPoint(nil), in.Requirements.FrozenSteamTable...)
	return out
}

func (m *MemoryRepository) GetRevision(id string) (domain.Revision, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.records[id]
	if !ok {
		return domain.Revision{}, domain.NewInputError(domain.CodeInvalidInput, "revision not found", id)
	}
	return record.Revision, nil
}

func (m *MemoryRepository) SaveRevision(revision domain.Revision, event domain.StateEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.records[revision.ID]
	if !ok {
		return domain.NewInputError(domain.CodeInvalidInput, "revision not found", revision.ID)
	}
	if record.Revision.Version+1 != revision.Version {
		return domain.NewStateError(domain.CodeVersionConflict, "stored revision changed")
	}
	record.Revision = revision
	m.records[revision.ID] = record
	event.Ordinal = len(m.events[revision.ID]) + 1
	m.events[revision.ID] = append(m.events[revision.ID], event)
	return nil
}

func (m *MemoryRepository) Events(id string) ([]domain.StateEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.records[id]; !ok {
		return nil, domain.NewInputError(domain.CodeInvalidInput, "revision not found", id)
	}
	return append([]domain.StateEvent(nil), m.events[id]...), nil
}
