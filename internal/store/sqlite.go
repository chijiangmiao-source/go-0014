package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"steam-sterilization-thermal-validation/internal/domain"

	_ "modernc.org/sqlite"
)

type SQLiteRepository struct {
	db *sql.DB
}

func OpenSQLite(path string) (*SQLiteRepository, error) {
	if path == "" {
		path = filepath.Join("data", "sterilization.db")
	}
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	repo := &SQLiteRepository{db: db}
	if err := repo.Migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return repo, nil
}

func (s *SQLiteRepository) Close() error {
	return s.db.Close()
}

func (s *SQLiteRepository) Migrate(ctx context.Context) error {
	for _, statement := range schemaStatements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteRepository) CreateRevision(deviceID, businessKey string) (domain.Revision, error) {
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Revision{}, domain.NewTemporaryError("SQLite transaction could not start", err.Error())
	}
	defer rollback(tx)
	var existing string
	err = tx.QueryRowContext(ctx, `select current_revision_id from sterilization_runs where business_key = ?`, businessKey).Scan(&existing)
	if err == nil {
		revision, err := scanRevision(ctx, tx, existing)
		if err != nil {
			return domain.Revision{}, err
		}
		return revision, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return domain.Revision{}, err
	}
	runResult, err := tx.ExecContext(ctx, `insert into sterilization_runs(device_id, business_key, created_at_nanos) values(?, ?, ?)`, deviceID, businessKey, time.Now().UTC().UnixNano())
	if err != nil {
		return domain.Revision{}, err
	}
	runID, _ := runResult.LastInsertId()
	revID := fmt.Sprintf("rev-%06d", runID)
	inputs := domain.DraftInputs{Run: domain.RunDescriptor{DeviceID: deviceID, BusinessKey: businessKey}}
	inputBytes, _ := json.Marshal(inputs)
	revision := domain.Revision{ID: revID, State: domain.StateDraft, Version: 1}
	if _, err := tx.ExecContext(ctx, `insert into revisions(id, run_id, state, lock_version, inputs_json) values(?, ?, ?, ?, ?)`, revID, runID, revision.State, revision.Version, inputBytes); err != nil {
		return domain.Revision{}, err
	}
	if _, err := tx.ExecContext(ctx, `update sterilization_runs set current_revision_id = ? where id = ?`, revID, runID); err != nil {
		return domain.Revision{}, err
	}
	if err := insertEvent(ctx, tx, revID, domain.StateEvent{From: "", To: domain.StateDraft, ReasonCode: "create", ResultVersion: revision.Version}); err != nil {
		return domain.Revision{}, err
	}
	return revision, tx.Commit()
}

func (s *SQLiteRepository) GetRevision(id string) (domain.Revision, error) {
	return scanRevision(context.Background(), s.db, id)
}

func (s *SQLiteRepository) SaveRevision(revision domain.Revision, event domain.StateEvent) error {
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.NewTemporaryError("SQLite transaction could not start", err.Error())
	}
	defer rollback(tx)
	resultBytes, err := json.Marshal(revision.Result)
	if err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, `update revisions set state = ?, lock_version = ?, snapshot_hash = ?, algorithm_version = ?, conclusion = ?, result_json = ? where id = ? and lock_version = ?`,
		revision.State, revision.Version, revision.SnapshotHash, revision.AlgorithmVersion, revision.Conclusion, nullableJSON(resultBytes, revision.Result != nil), revision.ID, revision.Version-1)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows != 1 {
		return domain.NewStateError(domain.CodeVersionConflict, "stored revision changed")
	}
	if err := insertEvent(ctx, tx, revision.ID, event); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLiteRepository) Events(id string) ([]domain.StateEvent, error) {
	rows, err := s.db.Query(`select ordinal, from_state, to_state, reason_code, result_version from state_events where revision_id = ? order by ordinal`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []domain.StateEvent
	for rows.Next() {
		var event domain.StateEvent
		if err := rows.Scan(&event.Ordinal, &event.From, &event.To, &event.ReasonCode, &event.ResultVersion); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if len(events) == 0 {
		return nil, domain.NewInputError(domain.CodeInvalidInput, "revision not found", id)
	}
	return events, rows.Err()
}

func (s *SQLiteRepository) UpdateInputs(id string, expectedVersion int64, mutate func(*domain.DraftInputs) error) (domain.Revision, error) {
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Revision{}, domain.NewTemporaryError("SQLite transaction could not start", err.Error())
	}
	defer rollback(tx)
	revision, err := scanRevision(ctx, tx, id)
	if err != nil {
		return domain.Revision{}, err
	}
	if revision.State != domain.StateDraft {
		return domain.Revision{}, domain.NewStateError(domain.CodeIllegalState, "inputs can only change while draft")
	}
	if revision.Version != expectedVersion {
		return domain.Revision{}, domain.NewStateError(domain.CodeVersionConflict, "revision version does not match expected_version")
	}
	inputs, err := scanInputs(ctx, tx, id)
	if err != nil {
		return domain.Revision{}, err
	}
	if err := mutate(&inputs); err != nil {
		return domain.Revision{}, err
	}
	inputBytes, err := json.Marshal(inputs)
	if err != nil {
		return domain.Revision{}, err
	}
	revision.Version++
	res, err := tx.ExecContext(ctx, `update revisions set lock_version = ?, inputs_json = ? where id = ? and lock_version = ?`, revision.Version, inputBytes, id, expectedVersion)
	if err != nil {
		return domain.Revision{}, err
	}
	rows, _ := res.RowsAffected()
	if rows != 1 {
		return domain.Revision{}, domain.NewStateError(domain.CodeVersionConflict, "stored revision changed")
	}
	if err := insertEvent(ctx, tx, id, domain.StateEvent{From: domain.StateDraft, To: domain.StateDraft, ReasonCode: "input_update", ResultVersion: revision.Version}); err != nil {
		return domain.Revision{}, err
	}
	return revision, tx.Commit()
}

func (s *SQLiteRepository) Inputs(id string) (domain.DraftInputs, error) {
	return scanInputs(context.Background(), s.db, id)
}

func (s *SQLiteRepository) SaveSnapshot(id string, snapshot domain.Snapshot) error {
	bytes, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`update revisions set snapshot_json = ? where id = ?`, bytes, id)
	return err
}

func (s *SQLiteRepository) Snapshot(id string) (domain.Snapshot, error) {
	var data []byte
	err := s.db.QueryRow(`select snapshot_json from revisions where id = ?`, id).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Snapshot{}, domain.NewInputError(domain.CodeInvalidInput, "revision not found", id)
	}
	if err != nil {
		return domain.Snapshot{}, err
	}
	if len(data) == 0 {
		return domain.Snapshot{}, domain.NewInputError(domain.CodeInvalidInput, "revision is not frozen", id)
	}
	var snapshot domain.Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return domain.Snapshot{}, domain.NewInputError(domain.CodeDataUnrecoverable, "snapshot is damaged")
	}
	return snapshot, nil
}

func (s *SQLiteRepository) SaveJob(job domain.AnalysisJob) error {
	data, _ := json.Marshal(job)
	_, err := s.db.Exec(`insert into analysis_jobs(id, revision_id, type, status, job_json) values(?, ?, ?, ?, ?)
		on conflict(id) do update set status = excluded.status, job_json = excluded.job_json`,
		job.ID, job.RevisionID, job.Type, job.Status, data)
	return err
}

func (s *SQLiteRepository) Job(id string) (domain.AnalysisJob, error) {
	return s.lookupJob(id, `select job_json from analysis_jobs where id = ?`, id)
}

func (s *SQLiteRepository) JobForRevision(id string) (domain.AnalysisJob, error) {
	return s.lookupJob(id, `select job_json from analysis_jobs where revision_id = ? order by rowid desc limit 1`, id)
}

func (s *SQLiteRepository) lookupJob(notFoundDetail string, query string, args ...any) (domain.AnalysisJob, error) {
	var data []byte
	err := s.db.QueryRow(query, args...).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.AnalysisJob{}, domain.NewInputError(domain.CodeInvalidInput, "job not found", notFoundDetail)
	}
	if err != nil {
		return domain.AnalysisJob{}, err
	}
	var job domain.AnalysisJob
	return job, json.Unmarshal(data, &job)
}

func (s *SQLiteRepository) SaveEvidence(pkg domain.EvidencePackage) error {
	_, err := s.db.Exec(`insert into evidence_packages(revision_id, bytes, sha256, length, complete, version) values(?, ?, ?, ?, ?, ?)
		on conflict(revision_id) do update set bytes = excluded.bytes, sha256 = excluded.sha256, length = excluded.length, complete = excluded.complete, version = excluded.version`,
		pkg.RevisionID, pkg.Bytes, pkg.Hash, pkg.Length, boolInt(pkg.Complete), pkg.Version)
	return err
}

func (s *SQLiteRepository) Evidence(id string) (domain.EvidencePackage, error) {
	var pkg domain.EvidencePackage
	var complete int
	err := s.db.QueryRow(`select revision_id, bytes, sha256, length, complete, version from evidence_packages where revision_id = ?`, id).Scan(&pkg.RevisionID, &pkg.Bytes, &pkg.Hash, &pkg.Length, &complete, &pkg.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.EvidencePackage{}, domain.NewInputError(domain.CodeInvalidInput, "evidence is not sealed", id)
	}
	if err != nil {
		return domain.EvidencePackage{}, err
	}
	pkg.Complete = complete == 1
	return pkg, nil
}

func (s *SQLiteRepository) SaveReplay(replay domain.ReplayComparison) error {
	data, _ := json.Marshal(replay)
	_, err := s.db.Exec(`insert into replay_results(id, revision_id, replay_json) values(?, ?, ?)
		on conflict(id) do update set replay_json = excluded.replay_json`, replay.TaskID, replay.RevisionID, data)
	return err
}

func (s *SQLiteRepository) Replay(id string) (domain.ReplayComparison, error) {
	var data []byte
	err := s.db.QueryRow(`select replay_json from replay_results where id = ?`, id).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ReplayComparison{}, domain.NewInputError(domain.CodeInvalidInput, "replay not found", id)
	}
	if err != nil {
		return domain.ReplayComparison{}, err
	}
	var replay domain.ReplayComparison
	return replay, json.Unmarshal(data, &replay)
}

func (s *SQLiteRepository) ExpiredJobs(nowNanos int64) ([]domain.AnalysisJob, error) {
	rows, err := s.db.Query(`select job_json from analysis_jobs where status = 'RUNNING' order by id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var jobs []domain.AnalysisJob
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var job domain.AnalysisJob
		if err := json.Unmarshal(data, &job); err != nil {
			return nil, domain.NewInputError(domain.CodeDataUnrecoverable, "job JSON is damaged")
		}
		if job.LeaseUntilNanos > 0 && job.LeaseUntilNanos < nowNanos {
			jobs = append(jobs, job)
		}
	}
	return jobs, rows.Err()
}

func scanRevision(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, id string) (domain.Revision, error) {
	var revision domain.Revision
	var resultData []byte
	err := q.QueryRowContext(ctx, `select id, state, lock_version, coalesce(snapshot_hash, ''), coalesce(algorithm_version, ''), coalesce(conclusion, ''), result_json from revisions where id = ?`, id).
		Scan(&revision.ID, &revision.State, &revision.Version, &revision.SnapshotHash, &revision.AlgorithmVersion, &revision.Conclusion, &resultData)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Revision{}, domain.NewInputError(domain.CodeInvalidInput, "revision not found", id)
	}
	if err != nil {
		return domain.Revision{}, err
	}
	if len(resultData) > 0 {
		var result domain.AnalysisResult
		if err := json.Unmarshal(resultData, &result); err != nil {
			return domain.Revision{}, domain.NewInputError(domain.CodeDataUnrecoverable, "result JSON is damaged")
		}
		revision.Result = &result
	}
	return revision, nil
}

func scanInputs(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, id string) (domain.DraftInputs, error) {
	var data []byte
	err := q.QueryRowContext(ctx, `select inputs_json from revisions where id = ?`, id).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.DraftInputs{}, domain.NewInputError(domain.CodeInvalidInput, "revision not found", id)
	}
	if err != nil {
		return domain.DraftInputs{}, err
	}
	var inputs domain.DraftInputs
	if len(data) > 0 {
		err = json.Unmarshal(data, &inputs)
	}
	return inputs, err
}

func insertEvent(ctx context.Context, tx *sql.Tx, revisionID string, event domain.StateEvent) error {
	var next int
	if err := tx.QueryRowContext(ctx, `select coalesce(max(ordinal), 0) + 1 from state_events where revision_id = ?`, revisionID).Scan(&next); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `insert into state_events(revision_id, ordinal, from_state, to_state, reason_code, result_version, created_at_nanos) values(?, ?, ?, ?, ?, ?, ?)`,
		revisionID, next, event.From, event.To, event.ReasonCode, event.ResultVersion, time.Now().UTC().UnixNano())
	return err
}

func rollback(tx *sql.Tx) {
	_ = tx.Rollback()
}

func nullableJSON(data []byte, ok bool) any {
	if !ok {
		return nil
	}
	return data
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
