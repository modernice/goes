package middleware_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/contrib/auth"
	"github.com/modernice/goes/contrib/auth/http/middleware"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus"
	"github.com/modernice/goes/event/eventstore"
)

func TestAuthorize(t *testing.T) {
	actors := []uuid.UUID{uuid.New(), uuid.New()}

	bus := eventbus.New()
	store := eventstore.New()
	look := auth.NewLookup(store, bus)

	mw := middleware.Authorize(look, func(auth middleware.Authorizer, _ *http.Request) {
		for _, actor := range actors {
			auth.Authorize(actor)
		}
	})

	var authorizedActors []uuid.UUID
	h := mw(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		authorizedActors = middleware.AuthorizedActors(r.Context())
	}))

	srv := httptest.NewServer(h)
	defer srv.Close()

	req := httptest.NewRequest("GET", srv.URL, nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if len(authorizedActors) != len(actors) {
		t.Fatalf("%d actors should be authorized; got %d", len(actors), len(authorizedActors))
	}

	for i, actor := range actors {
		if authorizedActors[i] != actor {
			t.Fatalf("actor %q should be authorized", actor)
		}
	}
}

func TestAuthorizeField(t *testing.T) {
	actors := []uuid.UUID{uuid.New(), uuid.New()}

	mws := []func(http.Handler) http.Handler{
		middleware.AuthorizeField("fooId1"),
		middleware.AuthorizeField("fooId2"),
	}

	var authorizedActors []uuid.UUID
	var h http.Handler = http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		authorizedActors = middleware.AuthorizedActors(r.Context())
	})

	for _, mw := range mws {
		h = mw(h)
	}

	srv := httptest.NewServer(h)
	defer srv.Close()

	req := httptest.NewRequest("POST", srv.URL, strings.NewReader(fmt.Sprintf(`{
		"fooId1": %q,
		"fooId2": %q
	}`, actors[0], actors[1])))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if len(authorizedActors) != len(actors) {
		t.Fatalf("%d actors should be authorized; got %d", len(actors), len(authorizedActors))
	}

L:
	for _, actor := range actors {
		for _, aactor := range authorizedActors {
			if aactor == actor {
				continue L
			}
		}
		t.Fatalf("actor %q should be authorized", actor)
	}
}

func TestPermission_notGranted(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	actors := []uuid.UUID{
		uuid.New(),
		uuid.New(),
	}

	test := newPermissionTest(ctx, t, actors)

	ref := aggregate.Ref{
		Name: "foo",
		ID:   uuid.New(),
	}

	permission := middleware.Permission(test.fetcher, "view", func(*http.Request) aggregate.Ref { return ref })

	h := test.authorizeMiddleware(permission(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))

	srv := httptest.NewServer(h)
	defer srv.Close()

	req := httptest.NewRequest("GET", srv.URL, nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Result().StatusCode != http.StatusForbidden {
		t.Fatalf("StatusCode should be %v; is %v", http.StatusForbidden, rec.Result().StatusCode)
	}
}

func TestPermission_granted(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	actors := []uuid.UUID{
		uuid.New(),
		uuid.New(),
	}

	test := newPermissionTest(ctx, t, actors)

	ref := aggregate.Ref{
		Name: "foo",
		ID:   uuid.New(),
	}

	actor := auth.NewUUIDActor(actors[1])
	actor.Grant(ref, "view")

	if err := test.actors.Save(ctx, actor); err != nil {
		t.Fatalf("save actor: %v", err)
	}

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.NewTimer(3 * time.Second)
	defer timeout.Stop()

	permission := middleware.Permission(test.fetcher, "view", func(*http.Request) aggregate.Ref { return ref })
	h := test.authorizeMiddleware(permission(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))

	srv := httptest.NewServer(h)
	defer srv.Close()

	var lastStatusCode int
	for {
		select {
		case <-timeout.C:
			t.Fatalf("StatusCode should be %v; is %v", http.StatusNoContent, lastStatusCode)
		case <-ticker.C:
		}

		req := httptest.NewRequest("GET", srv.URL, nil)
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if lastStatusCode = rec.Result().StatusCode; lastStatusCode == http.StatusNoContent {
			return
		}
	}
}

type PermissionTest struct {
	bus                 event.Bus
	store               event.Store
	look                *auth.Lookup
	actors              auth.ActorRepository
	perms               auth.PermissionRepository
	fetcher             middleware.PermissionFetcher
	proj                *auth.PermissionProjector
	authorizeMiddleware func(http.Handler) http.Handler
}

func newPermissionTest(ctx context.Context, t *testing.T, actors []uuid.UUID) *PermissionTest {
	bus := eventbus.New()
	store := eventstore.WithBus(eventstore.New(), bus)
	look := auth.NewLookup(store, bus)
	repo := repository.New(store)
	actorRepo := auth.NewUUIDActorRepository(repo)
	perms := auth.InMemoryPermissionRepository()
	roles := auth.NewRoleRepository(repo)
	fetcher := middleware.RepositoryPermissionFetcher(perms)
	proj := auth.NewPermissionProjector(perms, roles, bus, store)

	errs, err := look.Run(ctx)
	if err != nil {
		t.Fatalf("run lookup: %v", err)
	}
	go panicOn(errs)

	errs, err = proj.Run(ctx)
	if err != nil {
		t.Fatalf("run projector: %v", err)
	}
	go panicOn(errs)

	authorize := middleware.Authorize(look, func(auth middleware.Authorizer, _ *http.Request) {
		for _, actor := range actors {
			auth.Authorize(actor)
		}
	})

	return &PermissionTest{
		bus:                 bus,
		store:               store,
		look:                look,
		actors:              actorRepo,
		perms:               perms,
		fetcher:             fetcher,
		proj:                proj,
		authorizeMiddleware: authorize,
	}
}

func panicOn(errs <-chan error) {
	for err := range errs {
		if !errors.Is(err, context.Canceled) {
			panic(err)
		}
	}
}
