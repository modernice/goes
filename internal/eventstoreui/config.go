package eventstoreui

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	defaultListenAddress = ":8080"
	defaultAssetsDir     = "ui/.output/public"
	defaultSessionTTL    = 12 * time.Hour
	defaultStreamPoll    = 10 * time.Second
)

var storeIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

// Config contains the complete runtime configuration for the standalone UI.
type Config struct {
	ListenAddress      string
	AssetsDir          string
	Username           string
	Password           string
	SessionSecret      []byte
	SessionTTL         time.Duration
	StreamPollInterval time.Duration
	SecureCookies      bool
	Stores             []StoreConfig
}

// StoreConfig describes one event-store connection. The connected user must
// have full access, including permission to create the UI's required indexes.
type StoreConfig struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Driver     string `json:"driver"`
	URL        string `json:"url"`
	Database   string `json:"database,omitempty"`
	Table      string `json:"table,omitempty"`
	Collection string `json:"collection,omitempty"`
}

// AuthenticationEnabled reports whether static username/password authentication
// is configured. Both values must be present to enable authentication.
func (cfg Config) AuthenticationEnabled() bool {
	return strings.TrimSpace(cfg.Username) != "" && strings.TrimSpace(cfg.Password) != ""
}

func (cfg Config) validateAuthentication() error {
	hasUsername := strings.TrimSpace(cfg.Username) != ""
	hasPassword := strings.TrimSpace(cfg.Password) != ""
	if hasUsername != hasPassword {
		return errors.New("GOES_UI_AUTH_USERNAME and GOES_UI_AUTH_PASSWORD must either both be set or both be empty")
	}
	if hasUsername && len(cfg.SessionSecret) < 32 {
		return errors.New("GOES_UI_SESSION_SECRET must be at least 32 characters when authentication is enabled")
	}
	return nil
}

// LoadConfig loads and validates configuration from GOES_UI_* environment variables.
func LoadConfig() (Config, error) {
	storesJSON, err := secretValue("GOES_UI_STORES")
	if err != nil {
		return Config{}, err
	}
	if strings.TrimSpace(storesJSON) == "" {
		return Config{}, errors.New("GOES_UI_STORES or GOES_UI_STORES_FILE is required")
	}

	var stores []StoreConfig
	if err := json.Unmarshal([]byte(storesJSON), &stores); err != nil {
		return Config{}, fmt.Errorf("parse GOES_UI_STORES: %w", err)
	}
	if len(stores) == 0 {
		return Config{}, errors.New("GOES_UI_STORES must contain at least one store")
	}

	seen := make(map[string]struct{}, len(stores))
	for i := range stores {
		store := &stores[i]
		store.ID = strings.TrimSpace(store.ID)
		store.Name = strings.TrimSpace(store.Name)
		store.Driver = strings.ToLower(strings.TrimSpace(store.Driver))
		store.URL = strings.TrimSpace(store.URL)
		store.Database = strings.TrimSpace(store.Database)
		store.Table = strings.TrimSpace(store.Table)
		store.Collection = strings.TrimSpace(store.Collection)

		if !storeIDPattern.MatchString(store.ID) {
			return Config{}, fmt.Errorf("store %d has invalid id %q", i, store.ID)
		}
		if _, ok := seen[store.ID]; ok {
			return Config{}, fmt.Errorf("store id %q is configured more than once", store.ID)
		}
		seen[store.ID] = struct{}{}
		if store.Name == "" {
			store.Name = store.ID
		}
		if store.URL == "" {
			return Config{}, fmt.Errorf("store %q is missing url", store.ID)
		}

		switch store.Driver {
		case "postgres", "postgresql":
			store.Driver = "postgres"
			if store.Table == "" {
				store.Table = "events"
			}
		case "mongo", "mongodb":
			store.Driver = "mongo"
			if store.Collection == "" {
				store.Collection = "events"
			}
		case "":
			return Config{}, fmt.Errorf("store %q is missing driver", store.ID)
		default:
			return Config{}, fmt.Errorf("store %q has unsupported driver %q", store.ID, store.Driver)
		}
	}

	username, err := secretValue("GOES_UI_AUTH_USERNAME")
	if err != nil {
		return Config{}, err
	}
	password, err := secretValue("GOES_UI_AUTH_PASSWORD")
	if err != nil {
		return Config{}, err
	}
	secret, err := secretValue("GOES_UI_SESSION_SECRET")
	if err != nil {
		return Config{}, err
	}
	cfg := Config{
		ListenAddress:      envOr("GOES_UI_LISTEN_ADDR", defaultListenAddress),
		AssetsDir:          envOr("GOES_UI_ASSETS_DIR", defaultAssetsDir),
		Username:           username,
		Password:           password,
		SessionSecret:      []byte(secret),
		SessionTTL:         defaultSessionTTL,
		StreamPollInterval: defaultStreamPoll,
		SecureCookies:      parseBool(os.Getenv("GOES_UI_SECURE_COOKIES")),
		Stores:             stores,
	}
	if err := cfg.validateAuthentication(); err != nil {
		return Config{}, err
	}

	if raw := strings.TrimSpace(os.Getenv("GOES_UI_SESSION_TTL")); raw != "" {
		ttl, err := time.ParseDuration(raw)
		if err != nil || ttl <= 0 {
			return Config{}, fmt.Errorf("invalid GOES_UI_SESSION_TTL %q", raw)
		}
		cfg.SessionTTL = ttl
	}
	if raw := strings.TrimSpace(os.Getenv("GOES_UI_STREAM_POLL_INTERVAL")); raw != "" {
		interval, err := time.ParseDuration(raw)
		if err != nil || interval <= 0 {
			return Config{}, fmt.Errorf("invalid GOES_UI_STREAM_POLL_INTERVAL %q", raw)
		}
		cfg.StreamPollInterval = interval
	}

	return cfg, nil
}

func secretValue(name string) (string, error) {
	if file := strings.TrimSpace(os.Getenv(name + "_FILE")); file != "" {
		contents, err := os.ReadFile(file)
		if err != nil {
			return "", fmt.Errorf("read %s_FILE: %w", name, err)
		}
		return strings.TrimSpace(string(contents)), nil
	}
	return strings.TrimSpace(os.Getenv(name)), nil
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func parseBool(value string) bool {
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	return err == nil && parsed
}
