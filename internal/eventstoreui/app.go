package eventstoreui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type configuredStore struct {
	info     StoreInfo
	reader   Reader
	streams  *streamCatalog
	overview *overviewProjection
}

type App struct {
	cfg    Config
	auth   *authenticator
	stores map[string]configuredStore
	mux    http.Handler
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func New(cfg Config, decoder Decoder) (*App, error) {
	if err := cfg.validateAuthentication(); err != nil {
		return nil, err
	}
	if decoder == nil {
		decoder = JSONDecoder{}
	}
	if cfg.StreamPollInterval <= 0 {
		cfg.StreamPollInterval = defaultStreamPoll
	}
	appCtx, cancel := context.WithCancel(context.Background())
	app := &App{
		cfg: cfg, auth: newAuthenticator(cfg), stores: make(map[string]configuredStore, len(cfg.Stores)), cancel: cancel,
	}
	for _, storeCfg := range cfg.Stores {
		var (
			reader Reader
			err    error
		)
		switch storeCfg.Driver {
		case "postgres":
			reader, err = newPostgresReader(storeCfg, decoder)
		case "mongo":
			reader, err = newMongoReader(storeCfg, decoder)
		default:
			err = fmt.Errorf("unsupported driver %q", storeCfg.Driver)
		}
		if err != nil {
			app.Close()
			return nil, fmt.Errorf("configure store %q: %w", storeCfg.ID, err)
		}
		indexCtx, cancelIndexes := context.WithTimeout(appCtx, 10*time.Minute)
		indexErr := reader.EnsureIndexes(indexCtx)
		cancelIndexes()
		if indexErr != nil {
			reader.Close()
			app.Close()
			return nil, fmt.Errorf("ensure indexes for store %q: %w", storeCfg.ID, indexErr)
		}
		loadedAt := time.Now().UTC()
		loadCtx, cancelLoad := context.WithTimeout(appCtx, 2*time.Minute)
		streams, summary, facets, loadErr := loadInitialStoreState(loadCtx, reader)
		cancelLoad()
		if loadErr != nil {
			reader.Close()
			app.Close()
			return nil, fmt.Errorf("load state for store %q: %w", storeCfg.ID, loadErr)
		}
		overview := newOverviewProjection(summary, facets)
		overlap := pollingOverlap(cfg.StreamPollInterval)
		seedCtx, cancelSeed := context.WithTimeout(appCtx, 2*time.Minute)
		recentEvents, seedErr := reader.EventMetadata(seedCtx, loadedAt.Add(-overlap))
		cancelSeed()
		if seedErr != nil {
			reader.Close()
			app.Close()
			return nil, fmt.Errorf("seed overview for store %q: %w", storeCfg.ID, seedErr)
		}
		overview.seed(recentEvents, loadedAt.Add(-overlap))
		store := configuredStore{
			info:     StoreInfo{ID: storeCfg.ID, Name: storeCfg.Name, Driver: storeCfg.Driver},
			reader:   reader,
			streams:  newStreamCatalog(streams),
			overview: overview,
		}
		app.stores[storeCfg.ID] = store
		app.startStreamPoller(appCtx, storeCfg.ID, store, loadedAt)
		app.startOverviewPoller(appCtx, storeCfg.ID, store, loadedAt)
	}

	handler, err := app.routes()
	if err != nil {
		app.Close()
		return nil, err
	}
	app.mux = handler
	return app, nil
}

func loadInitialStoreState(ctx context.Context, reader Reader) ([]streamMetadata, Summary, Facets, error) {
	var (
		streams    []streamMetadata
		summary    Summary
		facets     Facets
		streamErr  error
		summaryErr error
		facetsErr  error
		wg         sync.WaitGroup
	)
	wg.Add(3)
	go func() {
		defer wg.Done()
		streams, streamErr = reader.StreamStarts(ctx, time.Time{})
	}()
	go func() {
		defer wg.Done()
		summary, summaryErr = reader.Summary(ctx)
	}()
	go func() {
		defer wg.Done()
		facets, facetsErr = reader.Facets(ctx)
	}()
	wg.Wait()

	if streamErr != nil {
		return nil, Summary{}, Facets{}, fmt.Errorf("load streams: %w", streamErr)
	}
	if summaryErr != nil {
		return nil, Summary{}, Facets{}, fmt.Errorf("load summary: %w", summaryErr)
	}
	if facetsErr != nil {
		return nil, Summary{}, Facets{}, fmt.Errorf("load facets: %w", facetsErr)
	}
	return streams, summary, facets, nil
}

func pollingOverlap(interval time.Duration) time.Duration {
	return max(2*interval, time.Minute)
}

func (app *App) Handler() http.Handler { return app.mux }

func (app *App) Close() {
	if app.cancel != nil {
		app.cancel()
	}
	app.wg.Wait()
	for _, store := range app.stores {
		store.reader.Close()
	}
}

func (app *App) startStreamPoller(
	ctx context.Context,
	storeID string,
	store configuredStore,
	watermark time.Time,
) {
	interval := app.cfg.StreamPollInterval
	overlap := pollingOverlap(interval)
	app.wg.Add(1)
	go func() {
		defer app.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				pollStarted := time.Now().UTC()
				pollCtx, cancel := context.WithTimeout(ctx, max(interval, 30*time.Second))
				streams, err := store.reader.StreamStarts(pollCtx, watermark.Add(-overlap))
				cancel()
				if err != nil {
					if ctx.Err() == nil {
						log.Printf("poll streams for store %q: %v", storeID, err)
					}
					continue
				}
				store.streams.merge(streams)
				watermark = pollStarted
			}
		}
	}()
}

func (app *App) startOverviewPoller(
	ctx context.Context,
	storeID string,
	store configuredStore,
	watermark time.Time,
) {
	interval := app.cfg.StreamPollInterval
	overlap := pollingOverlap(interval)
	app.wg.Add(1)
	go func() {
		defer app.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				pollStarted := time.Now().UTC()
				pollCtx, cancel := context.WithTimeout(ctx, max(interval, 30*time.Second))
				events, err := store.reader.EventMetadata(pollCtx, watermark.Add(-overlap))
				cancel()
				if err != nil {
					if ctx.Err() == nil {
						log.Printf("poll overview for store %q: %v", storeID, err)
					}
					continue
				}
				store.overview.merge(events, pollStarted.Add(-overlap))
				watermark = pollStarted
			}
		}
	}()
}

func (app *App) routes() (http.Handler, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("POST /api/v1/auth/login", app.login)
	mux.HandleFunc("POST /api/v1/auth/logout", app.logout)
	mux.HandleFunc("GET /api/v1/auth/session", app.session)

	protected := http.NewServeMux()
	protected.HandleFunc("GET /api/v1/stores", app.listStores)
	protected.HandleFunc("GET /api/v1/stores/{storeID}/summary", app.storeSummary)
	protected.HandleFunc("GET /api/v1/stores/{storeID}/facets", app.storeFacets)
	protected.HandleFunc("GET /api/v1/stores/{storeID}/events", app.storeEvents)
	protected.HandleFunc("GET /api/v1/stores/{storeID}/events/{eventID}", app.storeEvent)
	protected.HandleFunc("GET /api/v1/stores/{storeID}/streams", app.storeStreams)
	protected.HandleFunc("GET /api/v1/stores/{storeID}/streams/{aggregateName}/{aggregateID}/events", app.streamEvents)
	mux.Handle("/api/v1/stores", app.requireAuth(protected))
	mux.Handle("/api/v1/stores/", app.requireAuth(protected))

	spa, err := newSPAHandler(app.cfg.AssetsDir)
	if err != nil {
		return nil, err
	}
	mux.Handle("/", spa)
	return securityHeaders(mux), nil
}

func (app *App) login(w http.ResponseWriter, r *http.Request) {
	if err := requireSameOrigin(r); err != nil {
		writeError(w, http.StatusForbidden, "invalid_origin", err.Error())
		return
	}
	if !app.auth.enabled() {
		writeJSON(w, http.StatusOK, app.sessionResponse(true))
		return
	}
	var credentials struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&credentials); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "expected a username and password")
		return
	}
	if !app.auth.validCredentials(credentials.Username, credentials.Password) {
		time.Sleep(250 * time.Millisecond)
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid username or password")
		return
	}
	if err := app.auth.setSession(w, r); err != nil {
		writeError(w, http.StatusInternalServerError, "session_error", "could not create session")
		return
	}
	writeJSON(w, http.StatusOK, app.sessionResponse(true))
}

func (app *App) logout(w http.ResponseWriter, r *http.Request) {
	if err := requireSameOrigin(r); err != nil {
		writeError(w, http.StatusForbidden, "invalid_origin", err.Error())
		return
	}
	if app.auth.enabled() {
		app.auth.clearSession(w, r)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (app *App) session(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, app.sessionResponse(app.auth.authenticated(r)))
}

func (app *App) sessionResponse(authenticated bool) map[string]any {
	response := map[string]any{
		"authenticated":         authenticated,
		"authenticationEnabled": app.auth.enabled(),
	}
	if authenticated && app.auth.enabled() {
		response["username"] = app.cfg.Username
	}
	return response
}

func (app *App) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !app.auth.authenticated(r) {
			writeError(w, http.StatusUnauthorized, "unauthorized", "sign in to continue")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (app *App) listStores(w http.ResponseWriter, r *http.Request) {
	infos := make([]StoreInfo, 0, len(app.cfg.Stores))
	for _, cfg := range app.cfg.Stores {
		infos = append(infos, app.stores[cfg.ID].info)
	}
	var wg sync.WaitGroup
	for i := range infos {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
			defer cancel()
			if err := app.stores[infos[index].ID].reader.Ping(ctx); err != nil {
				infos[index].Status = "unavailable"
				infos[index].Error = publicStoreError(err)
				return
			}
			infos[index].Status = "online"
		}(i)
	}
	wg.Wait()
	writeJSON(w, http.StatusOK, map[string]any{"items": infos})
}

func (app *App) storeSummary(w http.ResponseWriter, r *http.Request) {
	store, ok := app.store(w, r)
	if !ok {
		return
	}
	if store.overview != nil {
		aggregateStreams := int64(0)
		if store.streams != nil {
			aggregateStreams = int64(store.streams.len())
		}
		summary, _ := store.overview.snapshot(aggregateStreams)
		writeJSON(w, http.StatusOK, summary)
		return
	}
	summary, err := store.reader.Summary(r.Context())
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if store.streams != nil {
		summary.AggregateStreams = int64(store.streams.len())
	}
	writeJSON(w, http.StatusOK, summary)
}

func (app *App) storeFacets(w http.ResponseWriter, r *http.Request) {
	store, ok := app.store(w, r)
	if !ok {
		return
	}
	if store.overview != nil {
		_, facets := store.overview.snapshot(0)
		writeJSON(w, http.StatusOK, facets)
		return
	}
	facets, err := store.reader.Facets(r.Context())
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, facets)
}

func (app *App) storeEvents(w http.ResponseWriter, r *http.Request) {
	store, ok := app.store(w, r)
	if !ok {
		return
	}
	filter, err := parseEventFilter(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_filter", err.Error())
		return
	}
	page, err := store.reader.Events(r.Context(), filter)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (app *App) storeEvent(w http.ResponseWriter, r *http.Request) {
	store, ok := app.store(w, r)
	if !ok {
		return
	}
	event, err := store.reader.Event(r.Context(), r.PathValue("eventID"))
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "event not found")
		return
	}
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, event)
}

func (app *App) storeStreams(w http.ResponseWriter, r *http.Request) {
	store, ok := app.store(w, r)
	if !ok {
		return
	}
	limit, err := parseLimit(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_filter", err.Error())
		return
	}
	query := r.URL.Query()
	page, err := store.streams.page(StreamFilter{
		AggregateNames: compactStrings(query["aggregateName"]),
		AggregateID:    strings.TrimSpace(query.Get("aggregateId")),
		Cursor:         query.Get("cursor"),
		Limit:          limit,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_filter", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (app *App) streamEvents(w http.ResponseWriter, r *http.Request) {
	store, ok := app.store(w, r)
	if !ok {
		return
	}
	limit, err := parseLimit(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_filter", err.Error())
		return
	}
	var cursor versionCursor
	if err := decodeCursor(r.URL.Query().Get("cursor"), &cursor); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_filter", err.Error())
		return
	}
	page, err := store.reader.StreamEvents(
		r.Context(), r.PathValue("aggregateName"), r.PathValue("aggregateID"), cursor.Version, limit,
	)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (app *App) store(w http.ResponseWriter, r *http.Request) (configuredStore, bool) {
	store, ok := app.stores[r.PathValue("storeID")]
	if !ok {
		writeError(w, http.StatusNotFound, "store_not_found", "event store not found")
	}
	return store, ok
}

func parseEventFilter(r *http.Request) (EventFilter, error) {
	limit, err := parseLimit(r)
	if err != nil {
		return EventFilter{}, err
	}
	query := r.URL.Query()
	filter := EventFilter{
		Names:          compactStrings(query["name"]),
		AggregateNames: compactStrings(query["aggregateName"]),
		AggregateID:    strings.TrimSpace(query.Get("aggregateId")),
		Cursor:         query.Get("cursor"),
		Limit:          limit,
	}
	if raw := strings.TrimSpace(query.Get("from")); raw != "" {
		parsed, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			return EventFilter{}, fmt.Errorf("from must be an RFC3339 timestamp")
		}
		filter.From = &parsed
	}
	if raw := strings.TrimSpace(query.Get("to")); raw != "" {
		parsed, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			return EventFilter{}, fmt.Errorf("to must be an RFC3339 timestamp")
		}
		filter.To = &parsed
	}
	return filter, nil
}

func parseLimit(r *http.Request) (int, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("limit"))
	if raw == "" {
		return defaultPageSize, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit < 1 || limit > maxPageSize {
		return 0, fmt.Errorf("limit must be between 1 and %d", maxPageSize)
	}
	return limit, nil
}

func compactStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func writeStoreError(w http.ResponseWriter, err error) {
	writeError(w, http.StatusBadGateway, "store_error", publicStoreError(err))
}

func publicStoreError(err error) string {
	message := err.Error()
	for _, marker := range []string{"postgres://", "postgresql://", "mongodb://", "mongodb+srv://"} {
		if strings.Contains(strings.ToLower(message), marker) {
			return "could not query the configured event store"
		}
	}
	return message
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		return
	}
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; font-src 'self'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

type spaHandler struct {
	root  fs.FS
	files http.Handler
	index []byte
}

func newSPAHandler(directory string) (http.Handler, error) {
	indexPath := filepath.Join(directory, "index.html")
	index, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, fmt.Errorf("read UI assets at %q: %w", directory, err)
	}
	root := os.DirFS(directory)
	return &spaHandler{root: root, files: http.FileServer(http.FS(root)), index: index}, nil
}

func (handler *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		handler.serveIndex(w)
		return
	}
	if info, err := fs.Stat(handler.root, path); err == nil && !info.IsDir() {
		if strings.HasPrefix(path, "_nuxt/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		handler.files.ServeHTTP(w, r)
		return
	}
	if filepath.Ext(path) != "" {
		http.NotFound(w, r)
		return
	}
	handler.serveIndex(w)
}

func (handler *spaHandler) serveIndex(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(handler.index)
}
