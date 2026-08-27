package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"steam-sterilization-thermal-validation/internal/api"
	"steam-sterilization-thermal-validation/internal/domain"
	"steam-sterilization-thermal-validation/internal/ingest"
	"steam-sterilization-thermal-validation/internal/store"
	"steam-sterilization-thermal-validation/internal/workflow"
)

func TestEvidenceAPIRunsFreezeAnalyzeFinalizeAndDownload(t *testing.T) {
	handler := api.NewServer(workflow.NewService(store.NewMemoryRepository(), workflow.RealClock{}))
	revision := postJSON[domain.Revision](t, handler, "/api/v1/sterilization-runs", `{"device_id":"device","business_key":"e2e"}`, http.StatusCreated)
	revision = postJSON[domain.Revision](t, handler, "/api/v1/revisions/"+revision.ID+"/probes", mustJSON(t, map[string]any{"expected_version": revision.Version, "probes": e2eProbes()}), http.StatusOK)
	revision = postJSON[domain.Revision](t, handler, "/api/v1/revisions/"+revision.ID+"/calibrations", mustJSON(t, map[string]any{"expected_version": revision.Version, "calibrations": e2eCalibrations()}), http.StatusOK)
	revision = postJSON[domain.Revision](t, handler, "/api/v1/revisions/"+revision.ID+"/requirements", mustJSON(t, map[string]any{"expected_version": revision.Version, "requirements": e2eRequirements()}), http.StatusOK)
	revision = postJSON[domain.Revision](t, handler, "/api/v1/revisions/"+revision.ID+"/samples:batch", mustJSON(t, map[string]any{"expected_version": revision.Version, "samples": e2eSamples(122)}), http.StatusOK)
	revision = postJSON[domain.Revision](t, handler, "/api/v1/revisions/"+revision.ID+":freeze", mustJSON(t, map[string]any{"expected_version": revision.Version}), http.StatusOK)
	started := postJSON[struct {
		Revision domain.Revision `json:"revision"`
		JobID    string          `json:"job_id"`
	}](t, handler, "/api/v1/revisions/"+revision.ID+":start", mustJSON(t, map[string]any{"expected_version": revision.Version}), http.StatusAccepted)
	if started.JobID == "" || started.Revision.Result == nil || started.Revision.Conclusion != domain.ConclusionPass {
		t.Fatalf("unexpected started payload: %+v", started)
	}
	sealed := postJSON[domain.Revision](t, handler, "/api/v1/revisions/"+revision.ID+":finalize", mustJSON(t, map[string]any{"expected_version": started.Revision.Version}), http.StatusOK)
	if sealed.State != domain.StateSealedPass {
		t.Fatalf("sealed state = %s", sealed.State)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/revisions/"+revision.ID+"/evidence", nil))
	if rec.Code != http.StatusOK || rec.Header().Get("X-SHA-256") == "" || rec.Body.Len() == 0 {
		t.Fatalf("evidence status=%d sha=%q len=%d", rec.Code, rec.Header().Get("X-SHA-256"), rec.Body.Len())
	}
}

func TestEvidenceAPIProducesFailEvidenceForLowLoad(t *testing.T) {
	handler := api.NewServer(workflow.NewService(store.NewMemoryRepository(), workflow.RealClock{}))
	revision := postJSON[domain.Revision](t, handler, "/api/v1/sterilization-runs", `{"device_id":"device","business_key":"fail"}`, http.StatusCreated)
	revision = postJSON[domain.Revision](t, handler, "/api/v1/revisions/"+revision.ID+"/probes", mustJSON(t, map[string]any{"expected_version": revision.Version, "probes": e2eProbes()}), http.StatusOK)
	revision = postJSON[domain.Revision](t, handler, "/api/v1/revisions/"+revision.ID+"/calibrations", mustJSON(t, map[string]any{"expected_version": revision.Version, "calibrations": e2eCalibrations()}), http.StatusOK)
	revision = postJSON[domain.Revision](t, handler, "/api/v1/revisions/"+revision.ID+"/requirements", mustJSON(t, map[string]any{"expected_version": revision.Version, "requirements": e2eRequirements()}), http.StatusOK)
	revision = postJSON[domain.Revision](t, handler, "/api/v1/revisions/"+revision.ID+"/samples:batch", mustJSON(t, map[string]any{"expected_version": revision.Version, "samples": e2eSamples(119)}), http.StatusOK)
	revision = postJSON[domain.Revision](t, handler, "/api/v1/revisions/"+revision.ID+":freeze", mustJSON(t, map[string]any{"expected_version": revision.Version}), http.StatusOK)
	started := postJSON[struct {
		Revision domain.Revision `json:"revision"`
	}](t, handler, "/api/v1/revisions/"+revision.ID+":start", mustJSON(t, map[string]any{"expected_version": revision.Version}), http.StatusAccepted)
	sealed := postJSON[domain.Revision](t, handler, "/api/v1/revisions/"+revision.ID+":finalize", mustJSON(t, map[string]any{"expected_version": started.Revision.Version}), http.StatusOK)
	if sealed.State != domain.StateSealedFail {
		t.Fatalf("sealed state = %s", sealed.State)
	}
}

func postJSON[T any](t *testing.T, handler http.Handler, path, body string, status int) T {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rec, req)
	if rec.Code != status {
		t.Fatalf("%s status=%d body=%s", path, rec.Code, rec.Body.String())
	}
	var out T
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func e2eProbes() []domain.ProbeSpec {
	return []domain.ProbeSpec{{ProbeID: "load-1", Role: domain.RoleLoad, Required: true, Unit: "C"}, {ProbeID: "chamber-1", Role: domain.RoleChamber, Required: true, Unit: "C"}, {ProbeID: "pressure-1", Role: domain.RolePressure, Required: true, Unit: "kPa_abs"}}
}

func e2eCalibrations() []domain.Calibration {
	return []domain.Calibration{{ProbeID: "load-1", ValidFromNanos: 0, ValidUntilNanos: 120_000_000_000}, {ProbeID: "chamber-1", ValidFromNanos: 0, ValidUntilNanos: 120_000_000_000}}
}

func e2eRequirements() domain.Requirements {
	req := ingest.DefaultRequirements(0, 120_000_000_000)
	req.SampleStepNanos = 30_000_000_000
	req.MaxGapNanos = 60_000_000_000
	req.ExposureMinNanos = 60_000_000_000
	req.ConfirmNanos = 30_000_000_000
	req.MinLethalityMinutes = 1
	req.RequiredProbeIDs = []string{"load-1"}
	return req
}

func e2eSamples(load float64) []domain.SensorSample {
	var samples []domain.SensorSample
	for _, at := range []int64{0, 60_000_000_000, 120_000_000_000} {
		samples = append(samples,
			domain.SensorSample{ProbeID: "load-1", AtNanos: at, Kind: domain.SampleTemperature, Value: load, Unit: "C"},
			domain.SensorSample{ProbeID: "chamber-1", AtNanos: at, Kind: domain.SampleTemperature, Value: 121, Unit: "C"},
			domain.SensorSample{ProbeID: "pressure-1", AtNanos: at, Kind: domain.SamplePressure, Value: 205, Unit: "kPa_abs"},
		)
	}
	return samples
}
