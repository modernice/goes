package authrpc_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/repository"
	aggregatepb "github.com/modernice/goes/api/proto/gen/aggregate"
	commonpb "github.com/modernice/goes/api/proto/gen/common"
	authpb "github.com/modernice/goes/api/proto/gen/contrib/auth"
	"github.com/modernice/goes/contrib/auth"
	"github.com/modernice/goes/contrib/auth/authrpc"
	"github.com/modernice/goes/event/eventbus"
	"github.com/modernice/goes/event/eventstore"
	"github.com/modernice/goes/internal/grpcstatus"
	"github.com/modernice/goes/internal/testutil"
	"github.com/modernice/goes/projection"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

var testRef = aggregate.Ref{
	Name: "foo",
	ID:   uuid.New(),
}

func TestServer_GetPermissions(t *testing.T) {
	actor, perms, repo := givenAnActorWithViewPermission(t)

	nonExistentActorID := uuid.New()

	tests := []struct {
		name    string
		actorID uuid.UUID
		want    auth.PermissionsDTO
	}{
		{
			name:    "non existent actor",
			actorID: nonExistentActorID,
			want: auth.PermissionsDTO{
				ActorID: nonExistentActorID,
				OfActor: make(auth.Actions),
				OfRoles: make(auth.Actions),
			},
		},
		{
			name:    "existing actor",
			actorID: actor.AggregateID(),
			want:    perms.PermissionsDTO,
		},
	}

	srv := authrpc.NewServer(repo, nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			resp, err := srv.GetPermissions(ctx, commonpb.NewUUID(tt.actorID))
			if err != nil {
				t.Fatalf("GetPermissions() failed with %q", err)
			}

			got := resp.AsDTO()
			if !got.Equal(tt.want) {
				t.Fatalf("GetPermissions() returned wrong permissions.\n\n%s", cmp.Diff(tt.want, got))
			}
		})
	}
}

func TestServer_Allows(t *testing.T) {
	actor, _, repo := givenAnActorWithViewPermission(t)

	nonExistentActorID := uuid.New()

	tests := []struct {
		name    string
		actorID uuid.UUID
		want    bool
	}{
		{
			name:    "non existent actor",
			actorID: nonExistentActorID,
			want:    false,
		},
		{
			name:    "existing actor",
			actorID: actor.AggregateID(),
			want:    true,
		},
	}

	srv := authrpc.NewServer(repo, nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			resp, err := srv.Allows(ctx, &authpb.AllowsReq{
				ActorId:   commonpb.NewUUID(tt.actorID),
				Aggregate: aggregatepb.NewRef(testRef),
				Action:    "view",
			})
			if err != nil {
				t.Fatalf("Allows() failed with %q", err)
			}

			if !tt.want {
				if resp.GetAllowed() {
					t.Fatalf("%q action should be disallowed", "view")
				}
				return
			}

			if !resp.GetAllowed() {
				t.Fatalf("%q action should be allowed", "view")
			}
		})
	}
}

func TestServer_LookupActor(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := eventbus.New()
	store := eventstore.WithBus(eventstore.New(), bus)
	lookup := auth.NewLookup(store, bus)
	repo := repository.New(store)
	actorRepos := auth.NewActorRepositories(repo, nil)
	actors, _ := actorRepos.Repository(auth.StringActor)

	errs, err := lookup.Run(ctx)
	if err != nil {
		t.Fatalf("run lookup: %v", err)
	}
	go testutil.PanicOn(errs)

	actor := auth.NewStringActor(uuid.New())
	actor.Identify("foo")

	if err := actors.Save(ctx, actor); err != nil {
		t.Fatalf("save actor: %v", err)
	}

	<-time.After(100 * time.Millisecond)

	tests := []struct {
		sid       string
		want      uuid.UUID
		wantError *status.Status
	}{
		{
			sid: "bar",
			wantError: grpcstatus.New(codes.NotFound, fmt.Sprintf("actor %q not found", "bar"), &errdetails.LocalizedMessage{
				Locale:  "en",
				Message: fmt.Sprintf("Actor %q not found.", "bar"),
			}),
		},
		{
			sid:  "foo",
			want: actor.AggregateID(),
		},
	}

	srv := authrpc.NewServer(nil, lookup)

	for _, tt := range tests {
		t.Run(tt.sid, func(t *testing.T) {
			resp, err := srv.LookupActor(ctx, &authpb.LookupActorReq{StringId: tt.sid})

			if tt.wantError != nil {
				st := status.Convert(err)

				if !proto.Equal(st.Proto(), tt.wantError.Proto()) {
					t.Fatalf("LookupActor() returned wrong error.\n\t%v != %v", tt.wantError, st)
				}

				return
			}

			actorID := resp.AsUUID()

			if tt.want != actorID {
				t.Fatalf("LookupActor() returned wrong actor id. want=%v got=%v", tt.want, actorID)
			}
		})
	}
}

func TestServer_LookupRole(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := eventbus.New()
	store := eventstore.WithBus(eventstore.New(), bus)
	lookup := auth.NewLookup(store, bus)
	repo := repository.New(store)
	roles := auth.NewRoleRepository(repo)

	errs, err := lookup.Run(ctx)
	if err != nil {
		t.Fatalf("run lookup: %v", err)
	}
	go testutil.PanicOn(errs)

	role := auth.NewRole(uuid.New())
	role.Identify("foo")

	if err := roles.Save(ctx, role); err != nil {
		t.Fatalf("save role: %v", err)
	}

	<-time.After(100 * time.Millisecond)

	tests := []struct {
		name      string
		want      uuid.UUID
		wantError *status.Status
	}{
		{
			name: "bar",
			wantError: grpcstatus.New(codes.NotFound, fmt.Sprintf("role %q not found", "bar"), &errdetails.LocalizedMessage{
				Locale:  "en",
				Message: fmt.Sprintf("Role %q not found.", "bar"),
			}),
		},
		{
			name: "foo",
			want: role.AggregateID(),
		},
	}

	srv := authrpc.NewServer(nil, lookup)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := srv.LookupRole(ctx, &authpb.LookupRoleReq{Name: tt.name})

			if tt.wantError != nil {
				st := status.Convert(err)

				if !proto.Equal(st.Proto(), tt.wantError.Proto()) {
					t.Fatalf("LookupRole() returned wrong error.\n\t%v != %v", tt.wantError, st)
				}

				return
			}

			roleID := resp.AsUUID()

			if tt.want != roleID {
				t.Fatalf("LookupRole() returned wrong role id. want=%v got=%v", tt.want, roleID)
			}
		})
	}
}

func givenAnActorWithViewPermission(t *testing.T) (*auth.Actor, *auth.Permissions, auth.PermissionRepository) {
	repo := auth.InMemoryPermissionRepository()

	actor := auth.NewUUIDActor(uuid.New())
	actor.Grant(testRef, "view")

	perms := auth.PermissionsOf(actor.AggregateID())
	projection.Apply(perms, actor.AggregateChanges())

	if err := repo.Save(context.Background(), perms); err != nil {
		t.Fatalf("save permissions: %v", err)
	}

	return actor, perms, repo
}
