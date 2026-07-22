package eventstoreui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	t.Setenv("GOES_UI_STORES", `[
		{"id":"orders","name":"Orders","driver":"postgresql","url":"postgres://db/orders"},
		{"id":"billing","driver":"mongodb","url":"mongodb://db/billing"}
	]`)
	t.Setenv("GOES_UI_AUTH_USERNAME", "developer")
	t.Setenv("GOES_UI_AUTH_PASSWORD", "secret")
	t.Setenv("GOES_UI_SESSION_SECRET", strings.Repeat("x", 32))
	t.Setenv("GOES_UI_SESSION_TTL", "2h")
	t.Setenv("GOES_UI_STREAM_POLL_INTERVAL", "3s")
	t.Setenv("GOES_UI_SECURE_COOKIES", "true")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Stores) != 2 {
		t.Fatalf("expected two stores; got %d", len(cfg.Stores))
	}
	if cfg.Stores[0].Driver != "postgres" || cfg.Stores[0].Table != "events" {
		t.Fatalf("unexpected postgres defaults: %#v", cfg.Stores[0])
	}
	if cfg.Stores[1].Driver != "mongo" || cfg.Stores[1].Collection != "events" || cfg.Stores[1].Name != "billing" {
		t.Fatalf("unexpected mongo defaults: %#v", cfg.Stores[1])
	}
	if cfg.SessionTTL != 2*time.Hour || cfg.StreamPollInterval != 3*time.Second || !cfg.SecureCookies {
		t.Fatalf("unexpected runtime config: session ttl=%s stream poll=%s secure=%t", cfg.SessionTTL, cfg.StreamPollInterval, cfg.SecureCookies)
	}
}

func TestLoadConfigRejectsInvalidStreamPollInterval(t *testing.T) {
	t.Setenv("GOES_UI_STORES", `[{"id":"events","driver":"mongo","url":"mongodb://db/event"}]`)
	t.Setenv("GOES_UI_STREAM_POLL_INTERVAL", "0s")

	_, err := LoadConfig()
	if err == nil || !strings.Contains(err.Error(), "GOES_UI_STREAM_POLL_INTERVAL") {
		t.Fatalf("expected stream poll interval error; got %v", err)
	}
}

func TestLoadConfigSupportsSecretFiles(t *testing.T) {
	dir := t.TempDir()
	values := map[string]string{
		"GOES_UI_STORES":         `[{"id":"events","driver":"mongo","url":"mongodb://db/event"}]`,
		"GOES_UI_AUTH_USERNAME":  "developer",
		"GOES_UI_AUTH_PASSWORD":  "secret",
		"GOES_UI_SESSION_SECRET": strings.Repeat("s", 32),
	}
	for name, value := range values {
		path := filepath.Join(dir, strings.ToLower(name))
		if err := os.WriteFile(path, []byte(value+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv(name+"_FILE", path)
		t.Setenv(name, "")
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Username != "developer" || cfg.Password != "secret" || len(cfg.SessionSecret) != 32 {
		t.Fatalf("unexpected secret-backed config: %#v", cfg)
	}
}

func TestLoadConfigAllowsDisabledAuthentication(t *testing.T) {
	t.Setenv("GOES_UI_STORES", `[{"id":"events","driver":"mongo","url":"mongodb://db/event"}]`)
	t.Setenv("GOES_UI_AUTH_USERNAME", "")
	t.Setenv("GOES_UI_AUTH_PASSWORD", "")
	t.Setenv("GOES_UI_SESSION_SECRET", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AuthenticationEnabled() {
		t.Fatal("expected authentication to be disabled")
	}
}

func TestLoadConfigRejectsPartialAuthentication(t *testing.T) {
	t.Setenv("GOES_UI_STORES", `[{"id":"events","driver":"mongo","url":"mongodb://db/event"}]`)
	t.Setenv("GOES_UI_AUTH_USERNAME", "developer")
	t.Setenv("GOES_UI_AUTH_PASSWORD", "")
	t.Setenv("GOES_UI_SESSION_SECRET", "")

	_, err := LoadConfig()
	if err == nil || !strings.Contains(err.Error(), "both be set or both be empty") {
		t.Fatalf("expected partial authentication error; got %v", err)
	}
}

func TestLoadConfigRequiresSessionSecretWhenAuthenticationEnabled(t *testing.T) {
	t.Setenv("GOES_UI_STORES", `[{"id":"events","driver":"mongo","url":"mongodb://db/event"}]`)
	t.Setenv("GOES_UI_AUTH_USERNAME", "developer")
	t.Setenv("GOES_UI_AUTH_PASSWORD", "secret")
	t.Setenv("GOES_UI_SESSION_SECRET", "short")

	_, err := LoadConfig()
	if err == nil || !strings.Contains(err.Error(), "when authentication is enabled") {
		t.Fatalf("expected session secret error; got %v", err)
	}
}

func TestLoadConfigRejectsDuplicateStoreIDs(t *testing.T) {
	t.Setenv("GOES_UI_STORES", `[
		{"id":"events","driver":"mongo","url":"mongodb://one/event"},
		{"id":"events","driver":"postgres","url":"postgres://two/events"}
	]`)
	t.Setenv("GOES_UI_AUTH_USERNAME", "developer")
	t.Setenv("GOES_UI_AUTH_PASSWORD", "secret")
	t.Setenv("GOES_UI_SESSION_SECRET", strings.Repeat("x", 32))

	_, err := LoadConfig()
	if err == nil || !strings.Contains(err.Error(), "configured more than once") {
		t.Fatalf("expected duplicate id error; got %v", err)
	}
}
