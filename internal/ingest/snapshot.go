package ingest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	"steam-sterilization-thermal-validation/internal/domain"
)

func CanonicalSnapshot(snapshot domain.Snapshot) (domain.Snapshot, string, error) {
	if snapshot.AlgorithmVersion == "" {
		snapshot.AlgorithmVersion = AlgorithmVersion
	}
	sorted := snapshot
	sort.Slice(sorted.Probes, func(i, j int) bool { return sorted.Probes[i].ProbeID < sorted.Probes[j].ProbeID })
	sort.Slice(sorted.Calibrations, func(i, j int) bool {
		if sorted.Calibrations[i].ProbeID != sorted.Calibrations[j].ProbeID {
			return sorted.Calibrations[i].ProbeID < sorted.Calibrations[j].ProbeID
		}
		return sorted.Calibrations[i].ValidFromNanos < sorted.Calibrations[j].ValidFromNanos
	})
	folded, err := FoldSamples(sorted.Samples)
	if err != nil {
		return domain.Snapshot{}, "", err
	}
	sorted.Samples = folded
	sorted.Requirements = CanonicalizeRequirements(sorted.Requirements, sorted.Probes)
	if err := ValidateProbeSpecs(sorted.Probes); err != nil {
		return domain.Snapshot{}, "", err
	}
	if sorted.Requirements.RunEndNanos != 0 || sorted.Requirements.SampleStepNanos != 0 || len(sorted.Requirements.RequiredProbeIDs) > 0 {
		if err := ValidateRequirements(sorted.Requirements, sorted.Probes); err != nil {
			return domain.Snapshot{}, "", err
		}
	}
	bytes, err := json.Marshal(sorted)
	if err != nil {
		return domain.Snapshot{}, "", err
	}
	sum := sha256.Sum256(bytes)
	return sorted, hex.EncodeToString(sum[:]), nil
}

func ValidateProbeSpecs(probes []domain.ProbeSpec) error {
	seen := map[string]struct{}{}
	for _, probe := range probes {
		if probe.ProbeID == "" {
			return domain.NewInputError(domain.CodeInvalidInput, "probe id is required")
		}
		if !probe.Role.Valid() {
			return domain.NewInputError(domain.CodeInvalidInput, "probe role is invalid", probe.ProbeID)
		}
		if _, ok := seen[probe.ProbeID]; ok {
			return domain.NewInputError(domain.CodeInvalidInput, "probe id must be unique", probe.ProbeID)
		}
		seen[probe.ProbeID] = struct{}{}
	}
	return nil
}
