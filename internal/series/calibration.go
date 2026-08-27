package series

import "steam-sterilization-thermal-validation/internal/domain"

type CorrectedSample struct {
	ProbeID      string
	AtNanos      int64
	CorrectedC   float64
	Conservative float64
}

func ApplyCalibration(sample domain.SensorSample, calibration domain.Calibration) (CorrectedSample, error) {
	if sample.Kind != domain.SampleTemperature {
		return CorrectedSample{}, domain.NewInputError(domain.CodeInvalidInput, "calibration applies only to temperature samples", sample.ProbeID)
	}
	if sample.AtNanos < calibration.ValidFromNanos || sample.AtNanos > calibration.ValidUntilNanos {
		return CorrectedSample{}, domain.NewInputError(domain.CodeInvalidInput, "calibration does not cover sample", sample.ProbeID)
	}
	if calibration.UncertaintyC < 0 || !domain.IsFinite(calibration.OffsetC) || !domain.IsFinite(calibration.UncertaintyC) {
		return CorrectedSample{}, domain.NewInputError(domain.CodeInvalidInput, "calibration offset and uncertainty must be finite", sample.ProbeID)
	}
	corrected := sample.Value + calibration.OffsetC
	return CorrectedSample{ProbeID: sample.ProbeID, AtNanos: sample.AtNanos, CorrectedC: corrected, Conservative: corrected - calibration.UncertaintyC}, nil
}

func CalibrationCoversRun(calibration domain.Calibration, startNanos, endNanos int64) bool {
	return calibration.ValidFromNanos <= startNanos && calibration.ValidUntilNanos >= endNanos
}
