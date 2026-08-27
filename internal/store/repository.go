package store

import "steam-sterilization-thermal-validation/internal/domain"

type RevisionRecord struct {
	Revision    domain.Revision
	DeviceID    string
	BusinessKey string
	Inputs      domain.DraftInputs
	Snapshot    domain.Snapshot
	Evidence    *domain.EvidencePackage
	Jobs        map[string]domain.AnalysisJob
	Replays     map[string]domain.ReplayComparison
}
