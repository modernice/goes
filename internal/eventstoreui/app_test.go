package eventstoreui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestAppLoginAndProtectedStoreList(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<!doctype html><title>goes</title>"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		AssetsDir: dir, Username: "developer", Password: "secret", SessionSecret: []byte(strings.Repeat("s", 32)),
		SessionTTL: defaultSessionTTL,
		Stores:     []StoreConfig{{ID: "orders", Name: "Orders", Driver: "postgres"}},
	}
	app := &App{
		cfg:  cfg,
		auth: newAuthenticator(cfg),
		stores: map[string]configuredStore{
			"orders": {info: StoreInfo{ID: "orders", Name: "Orders", Driver: "postgres"}, reader: fakeReader{}},
		},
	}
	handler, err := app.routes()
	if err != nil {
		t.Fatal(err)
	}

	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "http://example.test/api/v1/stores", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized store list; got %d", unauthorized.Code)
	}

	login := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(http.MethodPost, "http://example.test/api/v1/auth/login", strings.NewReader(`{"username":"developer","password":"secret"}`))
	loginRequest.Header.Set("Origin", "http://example.test")
	handler.ServeHTTP(login, loginRequest)
	if login.Code != http.StatusOK {
		t.Fatalf("expected successful login; got %d: %s", login.Code, login.Body.String())
	}
	cookies := login.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected session cookie; got %#v", cookies)
	}

	stores := httptest.NewRecorder()
	storesRequest := httptest.NewRequest(http.MethodGet, "http://example.test/api/v1/stores", nil)
	storesRequest.AddCookie(cookies[0])
	handler.ServeHTTP(stores, storesRequest)
	if stores.Code != http.StatusOK {
		t.Fatalf("expected store list; got %d: %s", stores.Code, stores.Body.String())
	}
	var response struct {
		Items []StoreInfo `json:"items"`
	}
	if err := json.Unmarshal(stores.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Items) != 1 || response.Items[0].ID != "orders" || response.Items[0].Status != "online" {
		t.Fatalf("unexpected store response: %#v", response.Items)
	}
}

func TestAppWithoutAuthentication(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<!doctype html><title>goes</title>"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		AssetsDir:  dir,
		SessionTTL: defaultSessionTTL,
		Stores:     []StoreConfig{{ID: "orders", Name: "Orders", Driver: "postgres"}},
	}
	app := &App{
		cfg:  cfg,
		auth: newAuthenticator(cfg),
		stores: map[string]configuredStore{
			"orders": {info: StoreInfo{ID: "orders", Name: "Orders", Driver: "postgres"}, reader: fakeReader{}},
		},
	}
	handler, err := app.routes()
	if err != nil {
		t.Fatal(err)
	}

	stores := httptest.NewRecorder()
	handler.ServeHTTP(stores, httptest.NewRequest(http.MethodGet, "http://example.test/api/v1/stores", nil))
	if stores.Code != http.StatusOK {
		t.Fatalf("expected unauthenticated store access; got %d: %s", stores.Code, stores.Body.String())
	}

	session := httptest.NewRecorder()
	handler.ServeHTTP(session, httptest.NewRequest(http.MethodGet, "http://example.test/api/v1/auth/session", nil))
	if session.Code != http.StatusOK {
		t.Fatalf("expected session status; got %d: %s", session.Code, session.Body.String())
	}
	var status struct {
		Authenticated         bool `json:"authenticated"`
		AuthenticationEnabled bool `json:"authenticationEnabled"`
	}
	if err := json.Unmarshal(session.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if !status.Authenticated || status.AuthenticationEnabled {
		t.Fatalf("unexpected unsecured session response: %#v", status)
	}
}

func TestStreamCatalogRoutes(t *testing.T) {
	id := uuid.New()
	createdAt := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<!doctype html><title>goes</title>"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := Config{AssetsDir: dir, Stores: []StoreConfig{{ID: "orders", Name: "Orders", Driver: "mongo"}}}
	app := &App{
		cfg:  cfg,
		auth: newAuthenticator(cfg),
		stores: map[string]configuredStore{
			"orders": {
				info:   StoreInfo{ID: "orders", Name: "Orders", Driver: "mongo"},
				reader: fakeReader{},
				streams: newStreamCatalog([]streamMetadata{{
					AggregateName: "order", AggregateID: id, CreatedAt: createdAt,
				}}),
				overview: newOverviewProjection(
					Summary{TotalEvents: 2},
					Facets{EventNames: []Facet{{Value: "order.created", Count: 2}}},
				),
			},
		},
	}
	handler, err := app.routes()
	if err != nil {
		t.Fatal(err)
	}

	streams := httptest.NewRecorder()
	handler.ServeHTTP(streams, httptest.NewRequest(http.MethodGet, "http://example.test/api/v1/stores/orders/streams", nil))
	if streams.Code != http.StatusOK {
		t.Fatalf("expected stream page; got %d: %s", streams.Code, streams.Body.String())
	}
	var page StreamPage
	if err := json.Unmarshal(streams.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].AggregateID != id.String() {
		t.Fatalf("unexpected stream page: %#v", page)
	}

	streams = httptest.NewRecorder()
	handler.ServeHTTP(streams, httptest.NewRequest(http.MethodGet, "http://example.test/api/v1/stores/orders/streams?aggregateName=missing&aggregateName=order", nil))
	if streams.Code != http.StatusOK {
		t.Fatalf("expected filtered stream page; got %d: %s", streams.Code, streams.Body.String())
	}
	if err := json.Unmarshal(streams.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].AggregateID != id.String() {
		t.Fatalf("unexpected multi-filtered stream page: %#v", page)
	}

	summary := httptest.NewRecorder()
	handler.ServeHTTP(summary, httptest.NewRequest(http.MethodGet, "http://example.test/api/v1/stores/orders/summary", nil))
	if summary.Code != http.StatusOK {
		t.Fatalf("expected summary; got %d: %s", summary.Code, summary.Body.String())
	}
	var result Summary
	if err := json.Unmarshal(summary.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.TotalEvents != 2 || result.EventTypes != 1 || result.AggregateStreams != 1 {
		t.Fatalf("expected one materialized stream; got %#v", result)
	}

	facetsResponse := httptest.NewRecorder()
	handler.ServeHTTP(facetsResponse, httptest.NewRequest(http.MethodGet, "http://example.test/api/v1/stores/orders/facets", nil))
	if facetsResponse.Code != http.StatusOK {
		t.Fatalf("expected facets; got %d: %s", facetsResponse.Code, facetsResponse.Body.String())
	}
	var facets Facets
	if err := json.Unmarshal(facetsResponse.Body.Bytes(), &facets); err != nil {
		t.Fatal(err)
	}
	if len(facets.EventNames) != 1 || facets.EventNames[0].Count != 2 {
		t.Fatalf("unexpected materialized facets: %#v", facets)
	}
}

type fakeReader struct{}

func (fakeReader) EnsureIndexes(context.Context) error      { return nil }
func (fakeReader) Ping(context.Context) error               { return nil }
func (fakeReader) Summary(context.Context) (Summary, error) { return Summary{}, nil }
func (fakeReader) Facets(context.Context) (Facets, error)   { return Facets{}, nil }
func (fakeReader) EventMetadata(context.Context, time.Time) ([]eventMetadata, error) {
	return nil, nil
}
func (fakeReader) Events(context.Context, EventFilter) (EventPage, error) { return EventPage{}, nil }
func (fakeReader) Event(context.Context, string) (Event, error)           { return Event{}, ErrNotFound }
func (fakeReader) StreamStarts(context.Context, time.Time) ([]streamMetadata, error) {
	return nil, nil
}
func (fakeReader) StreamEvents(context.Context, string, string, int, int) (EventPage, error) {
	return EventPage{}, nil
}
func (fakeReader) Close() {}
