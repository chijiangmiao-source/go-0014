package workflow

import "steam-sterilization-thermal-validation/internal/domain"

func CanTransition(from, to domain.RevisionState) bool {
	switch from {
	case domain.StateDraft:
		return to == domain.StateFrozen
	case domain.StateFrozen:
		return to == domain.StateAnalyzing
	case domain.StateAnalyzing:
		return to == domain.StateResultReady || to == domain.StateAborted
	case domain.StateResultReady:
		return to == domain.StateSealedPass || to == domain.StateSealedFail
	default:
		return false
	}
}

func ApplyTransition(revision domain.Revision, expectedVersion int64, to domain.RevisionState, reason string) (domain.Revision, domain.StateEvent, error) {
	if revision.Version != expectedVersion {
		return domain.Revision{}, domain.StateEvent{}, domain.NewStateError(domain.CodeVersionConflict, "revision version does not match expected_version")
	}
	if !CanTransition(revision.State, to) {
		return domain.Revision{}, domain.StateEvent{}, domain.NewStateError(domain.CodeIllegalState, "revision state transition is not allowed")
	}
	from := revision.State
	revision.State = to
	revision.Version++
	event := domain.StateEvent{From: from, To: to, ReasonCode: reason, ResultVersion: revision.Version}
	return revision, event, nil
}
