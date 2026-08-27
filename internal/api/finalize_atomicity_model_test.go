package api_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"testing"

	"steam-sterilization-thermal-validation/internal/api"
	"steam-sterilization-thermal-validation/internal/domain"
	"steam-sterilization-thermal-validation/internal/store"
	"steam-sterilization-thermal-validation/internal/workflow"

	_ "modernc.org/sqlite"
)

func TestModel_FinalizeSQLiteAtomicPersistence(t *testing.T) {
	tests := []struct {
		name       string
		conclusion domain.Conclusion
		sealed     domain.RevisionState
	}{
		{name: "pass", conclusion: domain.ConclusionPass, sealed: domain.StateSealedPass},
		{name: "fail", conclusion: domain.ConclusionFail, sealed: domain.StateSealedFail},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dbPath := filepath.Join(t.TempDir(), "finalize.db")
			repo, err := store.OpenSQLite(dbPath)
			if err != nil {
				t.Fatal(err)
			}

			revision, err := repo.CreateRevision("sterilizer-1", "cycle-"+tc.name)
			if err != nil {
				t.Fatal(err)
			}
			for _, step := range []struct {
				to     domain.RevisionState
				reason string
			}{
				{to: domain.StateFrozen, reason: "freeze"},
				{to: domain.StateAnalyzing, reason: "start"},
				{to: domain.StateResultReady, reason: "complete"},
			} {
				next, event, err := workflow.ApplyTransition(revision, revision.Version, step.to, step.reason)
				if err != nil {
					t.Fatal(err)
				}
				if step.to == domain.StateFrozen {
					next.SnapshotHash = "frozen-snapshot-hash"
					next.AlgorithmVersion = "model-test-v1"
				}
				if step.to == domain.StateResultReady {
					next.Conclusion = tc.conclusion
					next.Result = &domain.AnalysisResult{Conclusion: tc.conclusion}
				}
				if err := repo.SaveRevision(next, event); err != nil {
					t.Fatal(err)
				}
				revision = next
			}
			beforeEvents, err := repo.Events(revision.ID)
			if err != nil {
				t.Fatal(err)
			}

			faultDB, err := sql.Open("sqlite", dbPath)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = faultDB.Close() })
			if _, err := faultDB.Exec(`create trigger reject_evidence before insert on evidence_packages begin select raise(abort, 'injected evidence failure'); end`); err != nil {
				t.Fatal(err)
			}

			request := func(handler http.Handler, method, path string, version int64) *httptest.ResponseRecorder {
				t.Helper()
				body := bytes.NewBufferString(fmt.Sprintf(`{"expected_version":%d}`, version))
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, httptest.NewRequest(method, path, body))
				return rec
			}
			finalizePath := "/api/v1/revisions/" + revision.ID + ":finalize"
			handler := api.NewServer(workflow.NewService(repo, workflow.RealClock{}))
			failed := request(handler, http.MethodPost, finalizePath, revision.Version)
			if failed.Code != http.StatusBadRequest {
				t.Fatalf("injected archive failure status = %d, body = %s", failed.Code, failed.Body.String())
			}
			if err := repo.Close(); err != nil {
				t.Fatal(err)
			}

			restarted, err := store.OpenSQLite(dbPath)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = restarted.Close() })
			persisted, err := restarted.GetRevision(revision.ID)
			if err != nil {
				t.Fatal(err)
			}
			if persisted.State != domain.StateResultReady || persisted.Version != revision.Version {
				t.Fatalf("failed finalize persisted terminal revision: got state=%s version=%d, want state=%s version=%d", persisted.State, persisted.Version, domain.StateResultReady, revision.Version)
			}
			afterEvents, err := restarted.Events(revision.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(afterEvents, beforeEvents) {
				t.Fatalf("failed finalize persisted state event: before=%+v after=%+v", beforeEvents, afterEvents)
			}
			if _, err := restarted.Evidence(revision.ID); err == nil {
				t.Fatal("evidence became visible after its write failed")
			}

			if _, err := faultDB.Exec(`drop trigger reject_evidence`); err != nil {
				t.Fatal(err)
			}
			handler = api.NewServer(workflow.NewService(restarted, workflow.RealClock{}))
			succeeded := request(handler, http.MethodPost, finalizePath, revision.Version)
			if succeeded.Code != http.StatusOK {
				t.Fatalf("retry status = %d, body = %s", succeeded.Code, succeeded.Body.String())
			}
			var sealed domain.Revision
			if err := json.NewDecoder(succeeded.Body).Decode(&sealed); err != nil {
				t.Fatal(err)
			}
			if sealed.State != tc.sealed || sealed.Version != revision.Version+1 {
				t.Fatalf("retry revision = %+v, want state=%s version=%d", sealed, tc.sealed, revision.Version+1)
			}

			download := httptest.NewRecorder()
			handler.ServeHTTP(download, httptest.NewRequest(http.MethodGet, "/api/v1/revisions/"+revision.ID+"/evidence", nil))
			if download.Code != http.StatusOK || download.Body.Len() == 0 || download.Header().Get("X-SHA-256") == "" {
				t.Fatalf("evidence download status=%d hash=%q length=%d", download.Code, download.Header().Get("X-SHA-256"), download.Body.Len())
			}
			firstBytes := append([]byte(nil), download.Body.Bytes()...)
			firstHash := download.Header().Get("X-SHA-256")

			repeated := request(handler, http.MethodPost, finalizePath, sealed.Version)
			if repeated.Code != http.StatusOK {
				t.Fatalf("idempotent finalize status = %d, body = %s", repeated.Code, repeated.Body.String())
			}
			download = httptest.NewRecorder()
			handler.ServeHTTP(download, httptest.NewRequest(http.MethodGet, "/api/v1/revisions/"+revision.ID+"/evidence", nil))
			if download.Code != http.StatusOK || download.Header().Get("X-SHA-256") != firstHash || !bytes.Equal(download.Body.Bytes(), firstBytes) {
				t.Fatalf("idempotent finalize changed evidence: status=%d hash=%q", download.Code, download.Header().Get("X-SHA-256"))
			}
			conflict := request(handler, http.MethodPost, finalizePath, revision.Version)
			if conflict.Code != http.StatusConflict {
				t.Fatalf("stale finalize status = %d, body = %s", conflict.Code, conflict.Body.String())
			}
		})
	}
}
