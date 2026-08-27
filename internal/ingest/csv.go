package ingest

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"strings"

	"steam-sterilization-thermal-validation/internal/domain"
)

func ParseSamplesCSV(data []byte, sourceOffset int) ([]domain.SensorSample, string, error) {
	reader := csv.NewReader(bytes.NewReader(data))
	reader.TrimLeadingSpace = true
	header, err := reader.Read()
	if err != nil {
		return nil, "", domain.NewInputError(domain.CodeInvalidInput, "CSV header is required")
	}
	index := map[string]int{}
	for i, column := range header {
		index[strings.ToLower(strings.TrimSpace(column))] = i
	}
	required := []string{"probe_id", "time", "kind", "value", "unit"}
	for _, name := range required {
		if _, ok := index[name]; !ok {
			return nil, "", domain.NewInputError(domain.CodeInvalidInput, "CSV column is required", name)
		}
	}
	var out []domain.SensorSample
	ordinal := sourceOffset
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, "", domain.NewInputError(domain.CodeInvalidInput, "CSV row cannot be parsed", err.Error())
		}
		value, err := strconv.ParseFloat(record[index["value"]], 64)
		if err != nil {
			return nil, "", domain.NewInputError(domain.CodeInvalidInput, "CSV value must be numeric", record[index["value"]])
		}
		kind := domain.SampleKind(strings.ToLower(strings.TrimSpace(record[index["kind"]])))
		sample, err := NormalizeSample(RawSample{
			ProbeID:       strings.TrimSpace(record[index["probe_id"]]),
			Time:          strings.TrimSpace(record[index["time"]]),
			Kind:          kind,
			Value:         value,
			Unit:          strings.TrimSpace(record[index["unit"]]),
			SourceOrdinal: ordinal,
		})
		if err != nil {
			return nil, "", err
		}
		out = append(out, sample)
		ordinal++
	}
	hash := sha256.Sum256(data)
	return out, hex.EncodeToString(hash[:]), nil
}

func MergeSampleBatch(existing, incoming []domain.SensorSample) ([]domain.SensorSample, error) {
	merged := make([]domain.SensorSample, 0, len(existing)+len(incoming))
	merged = append(merged, existing...)
	merged = append(merged, incoming...)
	return FoldSamples(merged)
}

func BuildSnapshot(inputs domain.DraftInputs) (domain.Snapshot, string, error) {
	snapshot := domain.Snapshot{
		AlgorithmVersion: AlgorithmVersion,
		Requirements:     inputs.Requirements,
		Probes:           inputs.Probes,
		Calibrations:     inputs.Calibrations,
		Samples:          inputs.Samples,
	}
	if len(snapshot.Samples) == 0 {
		return domain.Snapshot{}, "", domain.NewInputError(domain.CodeInvalidInput, "at least one sample is required")
	}
	return CanonicalSnapshot(snapshot)
}

func ValidateFreezeReadiness(snapshot domain.Snapshot) error {
	if len(snapshot.Probes) == 0 {
		return domain.NewInputError(domain.CodeInvalidInput, "at least one probe is required")
	}
	if len(snapshot.Calibrations) == 0 {
		return domain.NewInputError(domain.CodeInvalidInput, "at least one calibration is required")
	}
	if len(snapshot.Samples) == 0 {
		return domain.NewInputError(domain.CodeInvalidInput, "at least one sample is required")
	}
	probes := map[string]domain.ProbeSpec{}
	for _, probe := range snapshot.Probes {
		probes[probe.ProbeID] = probe
	}
	calibrations := map[string]domain.Calibration{}
	for _, calibration := range snapshot.Calibrations {
		calibrations[calibration.ProbeID] = calibration
	}
	for _, id := range snapshot.Requirements.RequiredProbeIDs {
		if _, ok := probes[id]; !ok {
			return domain.NewInputError(domain.CodeInvalidInput, "required probe has no specification", id)
		}
		calibration, ok := calibrations[id]
		if !ok {
			return domain.NewInputError(domain.CodeInvalidInput, "required probe has no calibration", id)
		}
		if calibration.ValidFromNanos > snapshot.Requirements.RunStartNanos || calibration.ValidUntilNanos < snapshot.Requirements.RunEndNanos {
			return domain.NewInputError(domain.CodeInvalidInput, "calibration does not cover run", id)
		}
	}
	for _, sample := range snapshot.Samples {
		if _, ok := probes[sample.ProbeID]; !ok {
			return domain.NewInputError(domain.CodeInvalidInput, "sample references unknown probe", sample.ProbeID)
		}
		if sample.AtNanos < snapshot.Requirements.RunStartNanos || sample.AtNanos > snapshot.Requirements.RunEndNanos {
			return domain.NewInputError(domain.CodeInvalidInput, "sample is outside run bounds", fmt.Sprintf("%s/%d", sample.ProbeID, sample.AtNanos))
		}
	}
	return nil
}
