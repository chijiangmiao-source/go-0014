package domain

import "math"

func IsFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

type Role string

const (
	RoleChamber  Role = "chamber"
	RoleLoad     Role = "load"
	RoleDrain    Role = "drain"
	RolePressure Role = "pressure"
)

func (r Role) Valid() bool {
	switch r {
	case RoleChamber, RoleLoad, RoleDrain, RolePressure:
		return true
	default:
		return false
	}
}

type RevisionState string

const (
	StateDraft       RevisionState = "DRAFT"
	StateFrozen      RevisionState = "FROZEN"
	StateAnalyzing   RevisionState = "ANALYZING"
	StateResultReady RevisionState = "RESULT_READY"
	StateSealedPass  RevisionState = "SEALED_PASS"
	StateSealedFail  RevisionState = "SEALED_FAIL"
	StateAborted     RevisionState = "ABORTED"
)

type Conclusion string

const (
	ConclusionPass Conclusion = "PASS"
	ConclusionFail Conclusion = "FAIL"
)

type Revision struct {
	ID               string          `json:"revision_id"`
	State            RevisionState   `json:"state"`
	Version          int64           `json:"version"`
	SnapshotHash     string          `json:"snapshot_hash,omitempty"`
	AlgorithmVersion string          `json:"algorithm_version,omitempty"`
	Conclusion       Conclusion      `json:"conclusion,omitempty"`
	Result           *AnalysisResult `json:"result,omitempty"`
}

type RunDescriptor struct {
	DeviceID    string `json:"device_id"`
	BusinessKey string `json:"business_key"`
}

type ProbeSpec struct {
	ProbeID  string `json:"probe_id"`
	Role     Role   `json:"role"`
	Required bool   `json:"required"`
	Unit     string `json:"unit"`
}

type Calibration struct {
	ProbeID         string  `json:"probe_id"`
	ValidFromNanos  int64   `json:"valid_from_nanos"`
	ValidUntilNanos int64   `json:"valid_until_nanos"`
	OffsetC         float64 `json:"offset_c"`
	UncertaintyC    float64 `json:"uncertainty_c"`
}

type SampleKind string

const (
	SampleTemperature SampleKind = "temperature"
	SamplePressure    SampleKind = "pressure"
)

type SensorSample struct {
	ProbeID       string     `json:"probe_id"`
	AtNanos       int64      `json:"at_nanos"`
	Kind          SampleKind `json:"kind"`
	Value         float64    `json:"value"`
	Unit          string     `json:"unit"`
	SourceOrdinal int        `json:"source_ordinal"`
}

type SteamPoint struct {
	PressureKPa float64 `json:"pressure_kpa"`
	SaturatedC  float64 `json:"saturated_c"`
}

type Requirements struct {
	RunStartNanos       int64        `json:"run_start_nanos"`
	RunEndNanos         int64        `json:"run_end_nanos"`
	SampleStepNanos     int64        `json:"sample_step_nanos"`
	MaxGapNanos         int64        `json:"max_gap_nanos"`
	ExposureMinC        float64      `json:"exposure_min_c"`
	ExposureMinNanos    int64        `json:"exposure_min_nanos"`
	ConfirmNanos        int64        `json:"confirm_nanos"`
	GraceNanos          int64        `json:"grace_nanos"`
	SpreadMaxC          float64      `json:"spread_max_c"`
	MinLethalityMinutes float64      `json:"min_lethality_minutes"`
	SteamToleranceC     float64      `json:"steam_tolerance_c"`
	SteamAllowedNanos   int64        `json:"steam_allowed_nanos"`
	TRefC               float64      `json:"t_ref_c"`
	ZC                  float64      `json:"z_c"`
	SteamTableVersion   string       `json:"steam_table_version"`
	RequiredProbeIDs    []string     `json:"required_probe_ids"`
	FrozenSteamTable    []SteamPoint `json:"frozen_steam_table"`
}

type Snapshot struct {
	AlgorithmVersion string         `json:"algorithm_version"`
	Requirements     Requirements   `json:"requirements"`
	Probes           []ProbeSpec    `json:"probes"`
	Calibrations     []Calibration  `json:"calibrations"`
	Samples          []SensorSample `json:"samples"`
}

type DraftInputs struct {
	Probes       []ProbeSpec    `json:"probes"`
	Calibrations []Calibration  `json:"calibrations"`
	Samples      []SensorSample `json:"samples"`
	Requirements Requirements   `json:"requirements"`
	Run          RunDescriptor  `json:"run"`
}

type ExposureSegment struct {
	StartNanos    int64   `json:"start_nanos"`
	EndNanos      int64   `json:"end_nanos"`
	DurationNanos int64   `json:"duration_nanos"`
	ColdPointC    float64 `json:"cold_point_c"`
}

type ProbeMetric struct {
	ProbeID           string  `json:"probe_id"`
	MinimumC          float64 `json:"minimum_c"`
	MaximumC          float64 `json:"maximum_c"`
	EquivalentMinutes float64 `json:"equivalent_minutes"`
	SummaryHash       string  `json:"summary_hash"`
}

type Finding struct {
	Ordinal          int     `json:"ordinal"`
	RuleCode         string  `json:"rule_code"`
	ProbeOrArea      string  `json:"probe_or_area,omitempty"`
	StartNanos       int64   `json:"start_nanos,omitempty"`
	EndNanos         int64   `json:"end_nanos,omitempty"`
	FirstFailedNanos int64   `json:"first_failed_nanos,omitempty"`
	Measured         float64 `json:"measured"`
	Threshold        float64 `json:"threshold"`
	Margin           float64 `json:"margin"`
	Unit             string  `json:"unit"`
	Severity         string  `json:"severity"`
}

type AnalysisResult struct {
	Segment    *ExposureSegment `json:"segment,omitempty"`
	Metrics    []ProbeMetric    `json:"metrics"`
	Findings   []Finding        `json:"findings"`
	Conclusion Conclusion       `json:"conclusion"`
}

type JobStatus string

const (
	JobQueued    JobStatus = "QUEUED"
	JobRunning   JobStatus = "RUNNING"
	JobSucceeded JobStatus = "SUCCEEDED"
	JobCanceled  JobStatus = "CANCELED"
	JobFailed    JobStatus = "FAILED"
)

type AnalysisJob struct {
	ID              string    `json:"job_id"`
	RevisionID      string    `json:"revision_id"`
	Type            string    `json:"type"`
	Status          JobStatus `json:"status"`
	LeaseHolder     string    `json:"lease_holder,omitempty"`
	LeaseUntilNanos int64     `json:"lease_until_nanos,omitempty"`
	Attempt         int       `json:"attempt"`
	DoneSteps       int       `json:"done_steps"`
	TotalSteps      int       `json:"total_steps"`
	CancelRequested bool      `json:"cancel_requested"`
	ErrorCode       ErrorCode `json:"error_code,omitempty"`
}

type EvidencePackage struct {
	RevisionID string `json:"revision_id"`
	Bytes      []byte `json:"-"`
	Hash       string `json:"sha256"`
	Length     int    `json:"length"`
	Complete   bool   `json:"complete"`
	Version    int64  `json:"version"`
}

type ReplayComparison struct {
	TaskID            string         `json:"task_id"`
	RevisionID        string         `json:"revision_id"`
	Result            AnalysisResult `json:"result"`
	OriginalHash      string         `json:"original_hash"`
	ReplayedHash      string         `json:"replayed_hash"`
	ConclusionMatches bool           `json:"conclusion_matches"`
	HashMatches       bool           `json:"hash_matches"`
	MetricCount       int            `json:"metric_count"`
	FindingCount      int            `json:"finding_count"`
}

type StateEvent struct {
	Ordinal       int           `json:"ordinal"`
	From          RevisionState `json:"from"`
	To            RevisionState `json:"to"`
	ReasonCode    string        `json:"reason_code"`
	ResultVersion int64         `json:"result_version"`
}
