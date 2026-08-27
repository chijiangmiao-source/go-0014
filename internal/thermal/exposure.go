package thermal

import (
	"math"

	"steam-sterilization-thermal-validation/internal/domain"
)

type GridReading struct {
	AtNanos       int64
	LoadLowerC    map[string]float64
	ChamberTempsC []float64
}

func IdentifyExposure(readings []GridReading, req domain.Requirements) (*domain.ExposureSegment, error) {
	var start int64
	var trueSince int64
	var haveCandidate bool
	var falseSince int64
	var lastGood int64
	cold := math.Inf(1)

	for _, reading := range readings {
		ok, pointCold := exposurePredicate(reading, req)
		if ok {
			if !haveCandidate {
				if trueSince == 0 {
					trueSince = reading.AtNanos
				}
				if reading.AtNanos-trueSince >= req.ConfirmNanos {
					start = trueSince
					haveCandidate = true
				}
			}
			if pointCold < cold {
				cold = pointCold
			}
			lastGood = reading.AtNanos
			falseSince = 0
			continue
		}
		trueSince = 0
		if !haveCandidate {
			continue
		}
		if falseSince == 0 {
			falseSince = reading.AtNanos
		}
		if reading.AtNanos-falseSince > req.GraceNanos {
			return &domain.ExposureSegment{StartNanos: start, EndNanos: lastGood, DurationNanos: lastGood - start, ColdPointC: cold}, nil
		}
	}
	if haveCandidate {
		return &domain.ExposureSegment{StartNanos: start, EndNanos: lastGood, DurationNanos: lastGood - start, ColdPointC: cold}, nil
	}
	return nil, nil
}

func exposurePredicate(reading GridReading, req domain.Requirements) (bool, float64) {
	cold := math.Inf(1)
	for _, id := range req.RequiredProbeIDs {
		v, ok := reading.LoadLowerC[id]
		if !ok || v < req.ExposureMinC {
			return false, cold
		}
		if v < cold {
			cold = v
		}
	}
	if len(reading.ChamberTempsC) > 1 {
		min, max := reading.ChamberTempsC[0], reading.ChamberTempsC[0]
		for _, v := range reading.ChamberTempsC[1:] {
			if v < min {
				min = v
			}
			if v > max {
				max = v
			}
		}
		if max-min > req.SpreadMaxC {
			return false, cold
		}
	}
	return true, cold
}
