package api

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"strconv"
	"strings"

	"steam-sterilization-thermal-validation/internal/domain"
	"steam-sterilization-thermal-validation/internal/ingest"
	"steam-sterilization-thermal-validation/internal/workflow"
)

type Server struct {
	mux *http.ServeMux
	svc *workflow.Service
}

func NewServer(svc *workflow.Service) http.Handler {
	server := &Server{mux: http.NewServeMux(), svc: svc}
	server.routes()
	return server
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.health)
	s.mux.HandleFunc("GET /readyz", s.ready)
	s.mux.HandleFunc("POST /api/v1/sterilization-runs", s.createRun)
	s.mux.HandleFunc("GET /api/v1/revisions/{id}", s.getRevision)
	s.mux.HandleFunc("GET /api/v1/revisions/{id}/events", s.getEvents)
	s.mux.HandleFunc("GET /api/v1/revisions/{id}/job", s.getJob)
	s.mux.HandleFunc("GET /api/v1/revisions/{id}/results", s.getResults)
	s.mux.HandleFunc("GET /api/v1/revisions/{id}/evidence", s.getEvidence)
	s.mux.HandleFunc("POST /api/v1/revisions/{id}/probes", s.putProbes)
	s.mux.HandleFunc("POST /api/v1/revisions/{id}/calibrations", s.putCalibrations)
	s.mux.HandleFunc("POST /api/v1/revisions/{id}/samples:batch", s.putSamples)
	s.mux.HandleFunc("POST /api/v1/revisions/{id}/requirements", s.putRequirements)
	s.mux.HandleFunc("POST /api/v1/revisions/{action...}", s.revisionAction)
	s.mux.HandleFunc("GET /api/v1/revisions/{id}/replays/{task_id}", s.getReplay)
	sub, _ := fs.Sub(staticFiles, "web/dist")
	s.mux.Handle("/", http.FileServerFS(sub))
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

type createRunRequest struct {
	DeviceID    string `json:"device_id"`
	BusinessKey string `json:"business_key"`
}

func (s *Server) createRun(w http.ResponseWriter, r *http.Request) {
	var req createRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, domain.NewInputError(domain.CodeInvalidInput, "invalid JSON request"))
		return
	}
	revision, err := s.svc.CreateRun(req.DeviceID, req.BusinessKey)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, revision)
}

func (s *Server) getRevision(w http.ResponseWriter, r *http.Request) {
	revision, err := s.svc.Revision(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, revision)
}

func (s *Server) getEvents(w http.ResponseWriter, r *http.Request) {
	events, err := s.svc.Events(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, events)
}

type versionedRequest struct {
	ExpectedVersion int64 `json:"expected_version"`
}

type probesRequest struct {
	ExpectedVersion int64              `json:"expected_version"`
	Probes          []domain.ProbeSpec `json:"probes"`
}

type calibrationsRequest struct {
	ExpectedVersion int64                `json:"expected_version"`
	Calibrations    []domain.Calibration `json:"calibrations"`
}

type samplesRequest struct {
	ExpectedVersion int64                 `json:"expected_version"`
	Samples         []domain.SensorSample `json:"samples"`
	CSV             string                `json:"csv,omitempty"`
}

type requirementsRequest struct {
	ExpectedVersion int64               `json:"expected_version"`
	Requirements    domain.Requirements `json:"requirements"`
}

func (s *Server) putProbes(w http.ResponseWriter, r *http.Request) {
	var req probesRequest
	if !decode(w, r, &req) {
		return
	}
	revision, err := s.svc.PutProbes(r.PathValue("id"), req.ExpectedVersion, req.Probes)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, revision)
}

func (s *Server) putCalibrations(w http.ResponseWriter, r *http.Request) {
	var req calibrationsRequest
	if !decode(w, r, &req) {
		return
	}
	revision, err := s.svc.PutCalibrations(r.PathValue("id"), req.ExpectedVersion, req.Calibrations)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, revision)
}

func (s *Server) putSamples(w http.ResponseWriter, r *http.Request) {
	var req samplesRequest
	if !decode(w, r, &req) {
		return
	}
	samples := req.Samples
	if req.CSV != "" {
		parsed, _, err := ingest.ParseSamplesCSV([]byte(req.CSV), len(samples))
		if err != nil {
			writeError(w, err)
			return
		}
		samples = append(samples, parsed...)
	}
	revision, err := s.svc.PutSamples(r.PathValue("id"), req.ExpectedVersion, samples)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, revision)
}

func (s *Server) putRequirements(w http.ResponseWriter, r *http.Request) {
	var req requirementsRequest
	if !decode(w, r, &req) {
		return
	}
	revision, err := s.svc.PutRequirements(r.PathValue("id"), req.ExpectedVersion, req.Requirements)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, revision)
}

func (s *Server) revisionAction(w http.ResponseWriter, r *http.Request) {
	action := r.PathValue("action")
	switch {
	case strings.HasSuffix(action, ":freeze"):
		s.freeze(w, withRevisionID(r, strings.TrimSuffix(action, ":freeze")))
	case strings.HasSuffix(action, ":start"):
		s.start(w, withRevisionID(r, strings.TrimSuffix(action, ":start")))
	case strings.HasSuffix(action, ":cancel"):
		s.cancel(w, withRevisionID(r, strings.TrimSuffix(action, ":cancel")))
	case strings.HasSuffix(action, ":finalize"):
		s.finalize(w, withRevisionID(r, strings.TrimSuffix(action, ":finalize")))
	case strings.HasSuffix(action, ":replay"):
		s.replay(w, withRevisionID(r, strings.TrimSuffix(action, ":replay")))
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) freeze(w http.ResponseWriter, r *http.Request) {
	var req versionedRequest
	if !decode(w, r, &req) {
		return
	}
	revision, err := s.svc.FreezeStored(r.PathValue("id"), req.ExpectedVersion)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, revision)
}

func (s *Server) start(w http.ResponseWriter, r *http.Request) {
	var req versionedRequest
	if !decode(w, r, &req) {
		return
	}
	revision, jobID, err := s.svc.StartAnalysis(r.PathValue("id"), req.ExpectedVersion)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"revision": revision, "job_id": jobID})
}

func (s *Server) cancel(w http.ResponseWriter, r *http.Request) {
	var req versionedRequest
	if !decode(w, r, &req) {
		return
	}
	revision, err := s.svc.Cancel(r.PathValue("id"), req.ExpectedVersion)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, revision)
}

func (s *Server) finalize(w http.ResponseWriter, r *http.Request) {
	var req versionedRequest
	if !decode(w, r, &req) {
		return
	}
	revision, err := s.svc.Finalize(r.PathValue("id"), req.ExpectedVersion)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, revision)
}

func (s *Server) getJob(w http.ResponseWriter, r *http.Request) {
	job, err := s.svc.Job(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) getResults(w http.ResponseWriter, r *http.Request) {
	revision, err := s.svc.Revision(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if revision.Result == nil {
		writeError(w, domain.NewInputError(domain.CodeInvalidInput, "results are not available"))
		return
	}
	writeJSON(w, http.StatusOK, revision.Result)
}

func (s *Server) getEvidence(w http.ResponseWriter, r *http.Request) {
	pkg, err := s.svc.Evidence(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("ETag", `"`+pkg.Hash+`"`)
	w.Header().Set("X-SHA-256", pkg.Hash)
	w.Header().Set("Content-Length", strconv.Itoa(pkg.Length))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pkg.Bytes)
}

func (s *Server) replay(w http.ResponseWriter, r *http.Request) {
	replay, err := s.svc.Replay(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, replay)
}

func (s *Server) getReplay(w http.ResponseWriter, r *http.Request) {
	replay, err := s.svc.ReplayResult(r.PathValue("task_id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, replay)
}

func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		writeError(w, domain.NewInputError(domain.CodeInvalidInput, "invalid JSON request"))
		return false
	}
	return true
}

func withRevisionID(r *http.Request, id string) *http.Request {
	clone := r.Clone(r.Context())
	clone.SetPathValue("id", id)
	return clone
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, err error) {
	var boundary *domain.BoundaryError
	if !errors.As(err, &boundary) {
		boundary = domain.NewInputError(domain.CodeInvalidInput, err.Error())
	}
	status := http.StatusBadRequest
	if strings.Contains(string(boundary.Code), "VERSION") || strings.Contains(string(boundary.Code), "STATE") {
		status = http.StatusConflict
	}
	writeJSON(w, status, boundary)
}
