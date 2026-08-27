package evidence

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"time"

	"steam-sterilization-thermal-validation/internal/domain"
)

type Manifest struct {
	Version      int                   `json:"version"`
	SnapshotHash string                `json:"snapshot_hash"`
	Algorithm    string                `json:"algorithm"`
	Result       domain.AnalysisResult `json:"result"`
	Events       []domain.StateEvent   `json:"events"`
}

type Package struct {
	Bytes  []byte
	Hash   string
	Length int
}

func Build(snapshotHash, algorithm string, result domain.AnalysisResult, events []domain.StateEvent) (Package, error) {
	canonical, err := CanonicalResult(result)
	if err != nil {
		return Package{}, err
	}
	if err := rejectNonFinite(canonical); err != nil {
		return Package{}, err
	}
	manifest := Manifest{Version: 1, SnapshotHash: snapshotHash, Algorithm: algorithm, Result: canonical, Events: events}
	payload, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return Package{}, err
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	header := &zip.FileHeader{Name: "manifest.json", Method: zip.Deflate}
	header.SetModTime(time.Unix(0, 0).UTC())
	w, err := zw.CreateHeader(header)
	if err != nil {
		return Package{}, err
	}
	if _, err := w.Write(payload); err != nil {
		return Package{}, err
	}
	if err := zw.Close(); err != nil {
		return Package{}, err
	}
	bytes := buf.Bytes()
	return Package{Bytes: bytes, Hash: SHA256Hex(bytes), Length: len(bytes)}, nil
}

func Verify(pkg Package) error {
	if pkg.Length != len(pkg.Bytes) || pkg.Hash != SHA256Hex(pkg.Bytes) {
		return domain.NewInputError(domain.CodeEvidenceCorrupt, "evidence length or hash does not match")
	}
	return nil
}

func rejectNonFinite(result domain.AnalysisResult) error {
	if result.Segment != nil && !domain.IsFinite(result.Segment.ColdPointC) {
		return domain.NewInputError(domain.CodeDataUnrecoverable, "segment contains non-finite cold point")
	}
	for _, metric := range result.Metrics {
		if !domain.IsFinite(metric.MinimumC) || !domain.IsFinite(metric.MaximumC) || !domain.IsFinite(metric.EquivalentMinutes) {
			return domain.NewInputError(domain.CodeDataUnrecoverable, "metric contains non-finite value", metric.ProbeID)
		}
	}
	for _, finding := range result.Findings {
		if !domain.IsFinite(finding.Measured) || !domain.IsFinite(finding.Threshold) || !domain.IsFinite(finding.Margin) {
			return domain.NewInputError(domain.CodeDataUnrecoverable, "finding contains non-finite value", finding.RuleCode)
		}
	}
	return nil
}
