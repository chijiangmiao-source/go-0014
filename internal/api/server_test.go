package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"steam-sterilization-thermal-validation/internal/api"
	"steam-sterilization-thermal-validation/internal/store"
	"steam-sterilization-thermal-validation/internal/workflow"
)

func TestServerCreateRunAndReadRevision(t *testing.T) {
	svc := workflow.NewService(store.NewMemoryRepository(), workflow.RealClock{})
	handler := api.NewServer(svc)
	body := bytes.NewBufferString(`{"device_id":"device-1","business_key":"run-1"}`)
	create := httptest.NewRecorder()
	handler.ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/api/v1/sterilization-runs", body))
	if create.Code != http.StatusCreated {
		t.Fatalf("create status = %d body %s", create.Code, create.Body.String())
	}
	var payload struct {
		ID string `json:"revision_id"`
	}
	if err := json.NewDecoder(create.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	read := httptest.NewRecorder()
	handler.ServeHTTP(read, httptest.NewRequest(http.MethodGet, "/api/v1/revisions/"+payload.ID, nil))
	if read.Code != http.StatusOK {
		t.Fatalf("read status = %d body %s", read.Code, read.Body.String())
	}
}

func TestServerReadyz(t *testing.T) {
	handler := api.NewServer(workflow.NewService(store.NewMemoryRepository(), workflow.RealClock{}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("ready status = %d", rec.Code)
	}
}
