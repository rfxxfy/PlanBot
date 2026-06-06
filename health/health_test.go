package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/adkhorst/planbot/database"
)

func TestRootHandler(t *testing.T) {
	srv := NewServer("8080", "test-v1")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	srv.rootHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["service"] != "PlanBot" || body["version"] != "test-v1" {
		t.Errorf("unexpected body: %+v", body)
	}
}

func TestRootHandler_NotFound(t *testing.T) {
	srv := NewServer("8080", "v")
	req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	rec := httptest.NewRecorder()

	srv.rootHandler(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestHealthHandler_DBNotInitialized(t *testing.T) {
	orig := database.DB
	database.DB = nil
	t.Cleanup(func() { database.DB = orig })

	srv := NewServer("8080", "v")
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	srv.healthHandler(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}

	var status Status
	if err := json.NewDecoder(rec.Body).Decode(&status); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if status.Status != "unhealthy" || status.Database != "not initialized" {
		t.Errorf("unexpected status: %+v", status)
	}
}

func TestReadyHandler_DBNotInitialized(t *testing.T) {
	orig := database.DB
	database.DB = nil
	t.Cleanup(func() { database.DB = orig })

	srv := NewServer("8080", "v")
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	srv.readyHandler(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
	if rec.Body.String() != "Database not initialized" {
		t.Errorf("unexpected body: %q", rec.Body.String())
	}
}
