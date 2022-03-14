package authrpc

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	aggregatepb "github.com/modernice/goes/api/proto/gen/aggregate"
	commonpb "github.com/modernice/goes/api/proto/gen/common"
	authpb "github.com/modernice/goes/api/proto/gen/contrib/auth"
	"github.com/modernice/goes/contrib/auth"
	"github.com/modernice/goes/internal/grpcstatus"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ auth.Client = (*Client)(nil)

// Server implements a gRPC server for the authorization module.
type Server struct {
	authpb.UnimplementedAuthServiceServer

	perms  auth.PermissionRepository
	lookup *auth.Lookup
}

// NewServer returns a new gRPC server for the authorization module.
func NewServer(perms auth.PermissionRepository, lookup *auth.Lookup) *Server {
	return &Server{
		perms:  perms,
		lookup: lookup,
	}
}

// GetPermissions returns the permission read-model of the given actor.
func (s *Server) GetPermissions(ctx context.Context, req *commonpb.UUID) (*authpb.Permissions, error) {
	id := req.AsUUID()
	perms, err := s.perms.Fetch(ctx, id)
	if err != nil {
		st, _ := status.New(codes.NotFound, err.Error()).WithDetails(&errdetails.LocalizedMessage{
			Locale:  "en",
			Message: "Permissions for actor %q not found.",
		})
		return nil, st.Err()
	}
	return authpb.NewPermissions(perms.PermissionsDTO), nil
}

// Allows returns whether the an actor is allowed to perform a given action.
func (s *Server) Allows(ctx context.Context, req *authpb.AllowsReq) (*authpb.AllowsResp, error) {
	actorID := req.GetActorId().AsUUID()
	ref := req.GetAggregate().AsRef()
	action := req.GetAction()

	perms, err := s.perms.Fetch(ctx, actorID)
	if err != nil {
		return nil, grpcstatus.New(codes.NotFound, err.Error(), &errdetails.LocalizedMessage{
			Locale:  "en",
			Message: fmt.Sprintf("Permissions of actor %q not found.", actorID),
		}).Err()
	}

	return &authpb.AllowsResp{Allowed: perms.Allows(action, ref)}, nil
}

// LookupActor returns the aggregate id of the actor with the given string-formatted actor id.
func (s *Server) LookupActor(ctx context.Context, req *authpb.LookupActorReq) (*commonpb.UUID, error) {
	sid := req.GetStringId()
	id, ok := s.lookup.Actor(ctx, sid)
	if !ok {
		return nil, grpcstatus.New(codes.NotFound, fmt.Sprintf("actor %q not found", sid), &errdetails.LocalizedMessage{
			Locale:  "en",
			Message: fmt.Sprintf("Actor %q not found.", sid),
		}).Err()
	}
	return commonpb.NewUUID(id), nil
}

// Client implements auth.Client.
type Client struct{ client authpb.AuthServiceClient }

// NewClient returns the gRPC client for the authorization module.
func NewClient(conn grpc.ClientConnInterface) *Client {
	return &Client{authpb.NewAuthServiceClient(conn)}
}

// Permissions implements auth.Client.
func (c *Client) Permissions(ctx context.Context, actorID uuid.UUID) (auth.PermissionsDTO, error) {
	perms, err := c.client.GetPermissions(ctx, commonpb.NewUUID(actorID))
	return perms.AsDTO(), err
}

// Allows implements auth.Client.
func (c *Client) Allows(ctx context.Context, actorID uuid.UUID, ref aggregate.Ref, action string) (bool, error) {
	resp, err := c.client.Allows(ctx, &authpb.AllowsReq{
		ActorId:   commonpb.NewUUID(actorID),
		Aggregate: aggregatepb.NewRef(ref),
		Action:    action,
	})
	return resp.GetAllowed(), err
}

// LookupActor implements auth.Client.
func (c *Client) LookupActor(ctx context.Context, sid string) (uuid.UUID, error) {
	actorID, err := c.client.LookupActor(ctx, &authpb.LookupActorReq{StringId: sid})
	return actorID.AsUUID(), err
}
