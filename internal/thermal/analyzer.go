package thermal

import (
	"sort"
	"sync"

	"steam-sterilization-thermal-validation/internal/adjudicate"
	"steam-sterilization-thermal-validation/internal/domain"
	"steam-sterilization-thermal-validation/internal/series"
)

type PreparedSeries struct {
	Grid          []int64
	Readings      []GridReading
	LoadCurves    map[string][]series.Point
	ChamberCurves map[string][]series.Point
	PressureCurve []series.Point
	SteamReadings []SteamReading
}

func AnalyzeSnapshot(snapshot domain.Snapshot) (domain.AnalysisResult, error) {
	prepared, err := PrepareSnapshot(snapshot)
	if err != nil {
		return domain.AnalysisResult{}, err
	}
	segment, err := IdentifyExposure(prepared.Readings, snapshot.Requirements)
	if err != nil {
		return domain.AnalysisResult{}, err
	}
	metrics, err := integrateMetrics(segment, prepared.LoadCurves, snapshot.Requirements)
	if err != nil {
		return domain.AnalysisResult{}, err
	}
	var findings []domain.Finding
	findings = append(findings, spreadFindings(prepared.Readings, snapshot.Requirements)...)
	steamFindings, err := SteamCompatibility(prepared.SteamReadings, snapshot.Requirements)
	if err != nil {
		return domain.AnalysisResult{}, err
	}
	findings = append(findings, steamFindings...)
	return adjudicate.Decide(segment, metrics, snapshot.Requirements, findings), nil
}

func PrepareSnapshot(snapshot domain.Snapshot) (PreparedSeries, error) {
	req := snapshot.Requirements
	baseGrid, err := series.FixedGrid(req.RunStartNanos, req.RunEndNanos, req.SampleStepNanos)
	if err != nil {
		return PreparedSeries{}, err
	}
	probes := map[string]domain.ProbeSpec{}
	for _, probe := range snapshot.Probes {
		probes[probe.ProbeID] = probe
	}
	calibrations := map[string]domain.Calibration{}
	for _, calibration := range snapshot.Calibrations {
		calibrations[calibration.ProbeID] = calibration
	}
	corrected := map[string][]series.Point{}
	pressureProbe := ""
	for _, probe := range snapshot.Probes {
		if probe.Role == domain.RolePressure && pressureProbe == "" {
			pressureProbe = probe.ProbeID
		}
	}
	var pressureRaw []series.Point
	for _, sample := range snapshot.Samples {
		spec, ok := probes[sample.ProbeID]
		if !ok {
			return PreparedSeries{}, domain.NewInputError(domain.CodeInvalidInput, "sample references unknown probe", sample.ProbeID)
		}
		switch sample.Kind {
		case domain.SampleTemperature:
			calibration, ok := calibrations[sample.ProbeID]
			if !ok {
				return PreparedSeries{}, domain.NewInputError(domain.CodeInvalidInput, "temperature sample has no calibration", sample.ProbeID)
			}
			correctedSample, err := series.ApplyCalibration(sample, calibration)
			if err != nil {
				return PreparedSeries{}, err
			}
			corrected[sample.ProbeID] = append(corrected[sample.ProbeID], series.Point{AtNanos: correctedSample.AtNanos, Value: correctedSample.Conservative})
		case domain.SamplePressure:
			if spec.Role != domain.RolePressure {
				return PreparedSeries{}, domain.NewInputError(domain.CodeInvalidInput, "pressure sample must use pressure probe", sample.ProbeID)
			}
			pressureRaw = append(pressureRaw, series.Point{AtNanos: sample.AtNanos, Value: sample.Value})
		default:
			return PreparedSeries{}, domain.NewInputError(domain.CodeInvalidInput, "unsupported sample kind")
		}
	}
	var allTimes [][]int64
	for probeID, points := range corrected {
		sort.Slice(points, func(i, j int) bool { return points[i].AtNanos < points[j].AtNanos })
		if gaps := series.CoverageGaps(points, req.RunStartNanos, req.RunEndNanos, req.MaxGapNanos); len(gaps) > 0 {
			gaps[0].ProbeOrArea = probeID
			return PreparedSeries{}, domain.NewInputError(domain.CodeInvalidInput, "temperature coverage gap", probeID)
		}
		corrected[probeID] = points
		allTimes = append(allTimes, series.Times(points))
	}
	sort.Slice(pressureRaw, func(i, j int) bool { return pressureRaw[i].AtNanos < pressureRaw[j].AtNanos })
	if pressureProbe != "" {
		if gaps := series.CoverageGaps(pressureRaw, req.RunStartNanos, req.RunEndNanos, req.MaxGapNanos); len(gaps) > 0 {
			return PreparedSeries{}, domain.NewInputError(domain.CodeInvalidInput, "pressure coverage gap", pressureProbe)
		}
		allTimes = append(allTimes, series.Times(pressureRaw))
	}
	grid := series.UnionGrid(baseGrid, allTimes...)
	loadCurves := map[string][]series.Point{}
	chamberCurves := map[string][]series.Point{}
	var wg sync.WaitGroup
	errs := make(chan error, len(snapshot.Probes))
	for _, probe := range snapshot.Probes {
		if probe.Role != domain.RoleLoad && probe.Role != domain.RoleChamber {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			points := corrected[probe.ProbeID]
			curve, err := series.InterpolatedCurve(points, grid, req.MaxGapNanos)
			if err != nil {
				errs <- err
				return
			}
			if probe.Role == domain.RoleLoad {
				loadCurves[probe.ProbeID] = curve
			}
			if probe.Role == domain.RoleChamber {
				chamberCurves[probe.ProbeID] = curve
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		return PreparedSeries{}, err
	}
	pressureCurve, err := series.InterpolatedCurve(pressureRaw, grid, req.MaxGapNanos)
	if err != nil {
		return PreparedSeries{}, err
	}
	readings := make([]GridReading, 0, len(grid))
	steamReadings := make([]SteamReading, 0, len(grid))
	for i, at := range grid {
		loadLower := map[string]float64{}
		for _, id := range req.RequiredProbeIDs {
			if curve, ok := loadCurves[id]; ok {
				loadLower[id] = curve[i].Value
			}
		}
		var chamberTemps []float64
		for _, probe := range snapshot.Probes {
			if probe.Role == domain.RoleChamber {
				chamberTemps = append(chamberTemps, chamberCurves[probe.ProbeID][i].Value)
			}
		}
		readings = append(readings, GridReading{AtNanos: at, LoadLowerC: loadLower, ChamberTempsC: chamberTemps})
		steamReadings = append(steamReadings, SteamReading{AtNanos: at, PressureKPa: pressureCurve[i].Value, ChamberC: chamberTemps})
	}
	return PreparedSeries{Grid: grid, Readings: readings, LoadCurves: loadCurves, ChamberCurves: chamberCurves, PressureCurve: pressureCurve, SteamReadings: steamReadings}, nil
}

func integrateMetrics(segment *domain.ExposureSegment, curves map[string][]series.Point, req domain.Requirements) ([]domain.ProbeMetric, error) {
	ids := make([]string, 0, len(curves))
	for id := range curves {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	metrics := make([]domain.ProbeMetric, 0, len(ids))
	for _, id := range ids {
		curve := curves[id]
		window := curve
		if segment != nil {
			window = clipCurve(curve, segment.StartNanos, segment.EndNanos)
		}
		if len(window) < 2 {
			continue
		}
		metric, err := LethalityMinutes(id, window, req)
		if err != nil {
			return nil, err
		}
		metrics = append(metrics, metric)
	}
	return metrics, nil
}

func clipCurve(curve []series.Point, start, end int64) []series.Point {
	out := make([]series.Point, 0, len(curve))
	for _, point := range curve {
		if point.AtNanos >= start && point.AtNanos <= end {
			out = append(out, point)
		}
	}
	return out
}

func spreadFindings(readings []GridReading, req domain.Requirements) []domain.Finding {
	var findings []domain.Finding
	var open *domain.Finding
	for _, reading := range readings {
		point := ChamberSpreadFinding(reading, req)
		if point == nil {
			if open != nil {
				findings = append(findings, *open)
				open = nil
			}
			continue
		}
		if open == nil {
			copy := *point
			open = &copy
			continue
		}
		open.EndNanos = point.EndNanos
		if point.Measured > open.Measured {
			open.Measured = point.Measured
			open.Margin = point.Margin
			open.FirstFailedNanos = point.FirstFailedNanos
		}
	}
	if open != nil {
		findings = append(findings, *open)
	}
	return findings
}
