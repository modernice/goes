package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/contrib/auth"
)

const authorizedActorsCtxKey = ctxKey("authorized_actors")

type ctxKey string

// Authorizer is provided by the Authorize() middleware and is used to authorize
// actors for the current request.
type Authorizer interface {
	// Lookup returns the aggregate id of the actor with the given actor id.
	Lookup(sid string) (uuid.UUID, bool)

	// Authorize adds the given actor to the authorized actors of the current request.
	Authorize(actorID uuid.UUID)
}

// AuthorizedActors returns the ids of the currently authorized actors.
func AuthorizedActors(ctx context.Context) []uuid.UUID {
	actors, _ := ctx.Value(authorizedActorsCtxKey).([]uuid.UUID)
	return actors
}

// Authorize returns a middleware that authorizes actors of a request.
// When a request is made and the middleware is called, the middleware calls
// the provided authorize function to authorize the actors of the request.
// The authorize function gets passed an Authorizer which allows authorization
// of multiple actors. Adding AuthorizeXXX() middleware to a handler does not
// automatically protect the routes from unauthorized access. PermissionXXX()
// middleware must be added to actually protect the routes. AuthorizeXXX()
// middleware must be called before PermissionXXX middleware is called.
// Otherwise the PermissionXXX middleware will always return 403 Forbidden.
func Authorize(lookup auth.Lookup, authorize func(Authorizer, *http.Request)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cloned, err := cloneRequest(r)
			if err != nil {
				http.Error(w, "failed to authorize", http.StatusInternalServerError)
				return
			}

			auth := &authorizer{
				lookup:         lookup,
				requestContext: r.Context(),
			}

			authorize(auth, cloned)

			next.ServeHTTP(w, r.WithContext(auth.requestContext))
		})
	}
}

// AuthorizeField returns a middleware that authorizes actors of a request.
// AuthorizeField differs from Authorize in that it extracts the aggregate id
// of the authorized actor from the request body. The request body is parsed as
// JSON and the JSON-field with the given name is then parsed as using uuid.Parse.
func AuthorizeField(field string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cloned, err := cloneRequest(r)
			if err != nil {
				http.Error(w, "failed to authorize", http.StatusInternalServerError)
				return
			}

			// If the request body is not valid JSON or does not contain the
			// field, just call the next handler and let the PermissionXXX
			// middleware fail if the request is unauthorized.
			body := make(map[string]any)

			if err := json.NewDecoder(cloned.Body).Decode(&body); err != nil {
				next.ServeHTTP(w, r)
				return
			}

			val, ok := body[field].(string)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			id, err := uuid.Parse(val)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			r = r.WithContext(withAuthorizedActor(r.Context(), id))
			next.ServeHTTP(w, r)
		})
	}
}

// Permission returns a middleware that protects routes from unauthorized access.
// When called, the middleware extracts the aggregate that the user wants to act
// on from the request body by calling the provided extractRef function.
// The middleware then checks if any of the authorized actors has the permission
// to perform the given action on the given aggregate. Only if an authorized
// actor is allowed to perform the action, the next handler is called. Otherwise
// the middleware returns 403 Forbidden.
func Permission(perms auth.PermissionFetcher, action string, extractRef func(*http.Request) aggregate.Ref) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cloned, err := cloneRequest(r)
			if err != nil {
				http.Error(w, "failed to authorize", http.StatusInternalServerError)
				return
			}

			ref := extractRef(cloned)
			if ref.IsZero() {
				forbidden(w)
				return
			}

			if !allowed(r.Context(), perms, ref, action) {
				forbidden(w)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// PermissionField returns a middleware that protects routes from unauthorized access.
// PermissionField differs from Permission in that it requires the aggregate
// name to be passed as an argument and that it extracts the aggregate id from
// the request body.
func PermissionField(perms auth.PermissionFetcher, action, aggregateName, field string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cloned, err := cloneRequest(r)
			if err != nil {
				forbidden(w)
				return
			}

			body := make(map[string]any)
			if err := json.NewDecoder(cloned.Body).Decode(&body); err != nil {
				forbidden(w)
				return
			}

			val, ok := body[field].(string)
			if !ok {
				forbidden(w)
				return
			}

			id, err := uuid.Parse(val)
			if err != nil {
				forbidden(w)
				return
			}

			ref := aggregate.Ref{
				Name: aggregateName,
				ID:   id,
			}

			if ref.IsZero() {
				forbidden(w)
				return
			}

			if !allowed(r.Context(), perms, ref, action) {
				forbidden(w)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func cloneRequest(req *http.Request) (*http.Request, error) {
	b, err := io.ReadAll(req.Body)
	if err != nil {
		return req, fmt.Errorf("read request body: %w", err)
	}

	out := req.Clone(req.Context())
	req.Body = io.NopCloser(bytes.NewReader(b))
	out.Body = io.NopCloser(bytes.NewReader(b))

	return out, nil
}

type authorizer struct {
	sync.Mutex
	lookup         auth.Lookup
	requestContext context.Context
}

// Lookup returns the aggregate id of the actor with the given actor id. It is a
// method of the Authorizer interface provided by the Authorize() middleware and
// is used to authorize actors for the current request.
func (a *authorizer) Lookup(sid string) (uuid.UUID, bool) {
	return a.lookup.Actor(a.requestContext, sid)
}

//jotbot:ignore
func (a *authorizer) Authorize(actorID uuid.UUID) {
	a.Lock()
	defer a.Unlock()
	a.requestContext = withAuthorizedActor(a.requestContext, actorID)
}

func withAuthorizedActor(ctx context.Context, actorID uuid.UUID) context.Context {
	actors := AuthorizedActors(ctx)
	for _, a := range actors {
		if a == actorID {
			return ctx
		}
	}
	return context.WithValue(ctx, authorizedActorsCtxKey, append(actors, actorID))
}

func allowed(ctx context.Context, permissions auth.PermissionFetcher, ref aggregate.Ref, action string) bool {
	actors := AuthorizedActors(ctx)

	for _, actor := range actors {
		perms, err := permissions.Fetch(ctx, actor)
		if err != nil {
			continue
		}

		if perms.Allows(action, ref) {
			return true
		}
	}

	return false
}

func forbidden(w http.ResponseWriter) {
	http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
}
