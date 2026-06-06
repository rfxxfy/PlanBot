package googlecal

import (
	"testing"
)

func TestConfigFromEnv_Missing(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "")
	t.Setenv("GOOGLE_CLIENT_SECRET", "")

	_, err := ConfigFromEnv()
	if err == nil {
		t.Error("expected error when Google credentials are missing")
	}
}

func TestConfigFromEnv_OK(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "test-client-id")
	t.Setenv("GOOGLE_CLIENT_SECRET", "test-secret")

	cfg, err := ConfigFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ClientID != "test-client-id" || cfg.ClientSecret != "test-secret" {
		t.Errorf("unexpected config: %+v", cfg)
	}
}

func TestNewFromAccessToken_Empty(t *testing.T) {
	_, err := NewFromAccessToken(t.Context(), "")
	if err == nil {
		t.Error("expected error for empty access token")
	}
}
