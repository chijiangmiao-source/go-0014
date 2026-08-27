package thermal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"

	"steam-sterilization-thermal-validation/internal/domain"
	"steam-sterilization-thermal-validation/internal/series"
)

func LethalityMinutes(probeID string, curve []series.Point, req domain.Requirements) (domain.ProbeMetric, error) {
	if len(curve) < 2 || req.ZC == 0 {
		return domain.ProbeMetric{}, domain.NewInputError(domain.CodeInvalidInput, "lethality curve requires two points and nonzero z")
	}
	minC, maxC := curve[0].Value, curve[0].Value
	var minutes float64
	for i := 0; i < len(curve)-1; i++ {
		a, b := curve[i], curve[i+1]
		if b.AtNanos < a.AtNanos || !domain.IsFinite(a.Value) || !domain.IsFinite(b.Value) {
			return domain.ProbeMetric{}, domain.NewInputError(domain.CodeDataUnrecoverable, "invalid lethality curve")
		}
		if a.Value < minC {
			minC = a.Value
		}
		if b.Value < minC {
			minC = b.Value
		}
		if a.Value > maxC {
			maxC = a.Value
		}
		if b.Value > maxC {
			maxC = b.Value
		}
		fa := math.Pow(10, (a.Value-req.TRefC)/req.ZC)
		fb := math.Pow(10, (b.Value-req.TRefC)/req.ZC)
		seconds := float64(b.AtNanos-a.AtNanos) / 1e9
		minutes += ((fa + fb) / 2) * seconds / 60
	}
	metric := domain.ProbeMetric{ProbeID: probeID, MinimumC: minC, MaximumC: maxC, EquivalentMinutes: minutes}
	metric.SummaryHash = metricHash(metric)
	return metric, nil
}

func metricHash(metric domain.ProbeMetric) string {
	stable := metric
	stable.SummaryHash = ""
	bytes, _ := json.Marshal(stable)
	sum := sha256.Sum256(bytes)
	return hex.EncodeToString(sum[:])
}
