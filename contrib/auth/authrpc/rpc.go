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
	"google.golang.org/protobuf/types/known/emptypb"
)

var (
	_ auth.QueryClient   = (*Client)(nil)
	_ auth.CommandClient = (*Client)(nil)
)

// Server implements a gRPC server for the authorization module.
type Server struct {
	authpb.UnimplementedAuthServiceServer

	perms  auth.PermissionRepository
	lookup auth.Lookup

	// if handlesCommands is true, actors and roles are non-nil
	handlesCommands bool
	actors          auth.ActorRepositories
	roles           auth.RoleRepository
}

// ServerOption is an option for the *Server.
type ServerOption func(*Server)

// HandleCommands returns a ServerOption that enables command handling.
// Specifically, the following methods are enabled:
//
//   - GrantToActor()
//   - GrantToRole()
//   - RevokeFromRole()
//   - RevokeFromActor()
//
// When trying to call these methods on a server that doesn't handle commands,
// the server will return an error. HandleCommands panics if the provided actor
// or role repository is nil.
func HandleCommands(actors auth.ActorRepositories, roles auth.RoleRepository) ServerOption {
	if actors == nil {
		panic("[goes/contrib/auth/authrpc.HandleCommands] ActorRepositories is nil")
	}

	if roles == nil {
		panic("[goes/contrib/auth/authrpc.HandleCommands] RoleRepository is nil")
	}

	return func(s *Server) {
		s.actors = actors
		s.roles = roles
		s.handlesCommands = true
	}
}

// NewServer returns a new gRPC server for the authorization module.
func NewServer(perms auth.PermissionRepository, lookup auth.Lookup, opts ...ServerOption) *Server {
	s := &Server{
		perms:  perms,
		lookup: lookup,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// GetPermissions implements authpb.AuthServiceServer.
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

// Allows implements authpb.AuthServiceServer.
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

// LookupActor implements authpb.AuthServiceServer.
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

// LookupRole implements authpb.AuthServiceServer.
func (s *Server) LookupRole(ctx context.Context, req *authpb.LookupRoleReq) (*commonpb.UUID, error) {
	name := req.GetName()
	id, ok := s.lookup.Role(ctx, name)
	if !ok {
		return nil, grpcstatus.New(codes.NotFound, fmt.Sprintf("role %q not found", name), &errdetails.LocalizedMessage{
			Locale:  "en",
			Message: fmt.Sprintf("Role %q not found.", name),
		}).Err()
	}
	return commonpb.NewUUID(id), nil
}

// GrantToActor implements authpb.AuthServiceServer.
func (s *Server) GrantToActor(ctx context.Context, req *authpb.GrantRevokeReq) (*emptypb.Empty, error) {
	if err := s.checkHandlesCommands(); err != nil {
		return nil, err
	}

	actorID := req.GetRoleOrActor().AsUUID()
	target := req.GetTarget().AsRef()
	actions := req.GetActions()

	actors, err := s.actors.Repository(auth.UUIDActor)
	if err != nil {
		return nil, status.Errorf(codes.Unimplemented, "get UUIDActor repository: %v", err)
	}

	if err := actors.Use(ctx, actorID, func(a *auth.Actor) error {
		return a.Grant(target, actions...)
	}); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &emptypb.Empty{}, nil
}

// GrantToRole implements authpb.AuthServiceServer.
func (s *Server) GrantToRole(ctx context.Context, req *authpb.GrantRevokeReq) (*emptypb.Empty, error) {
	if err := s.checkHandlesCommands(); err != nil {
		return nil, err
	}

	roleID := req.GetRoleOrActor().AsUUID()
	target := req.GetTarget().AsRef()
	actions := req.GetActions()

	if err := s.roles.Use(ctx, roleID, func(r *auth.Role) error {
		return r.Grant(target, actions...)
	}); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &emptypb.Empty{}, nil
}

// RevokeFromActor implements authpb.AuthServiceServer.
func (s *Server) RevokeFromActor(ctx context.Context, req *authpb.GrantRevokeReq) (*emptypb.Empty, error) {
	if err := s.checkHandlesCommands(); err != nil {
		return nil, err
	}

	actorID := req.GetRoleOrActor().AsUUID()
	target := req.GetTarget().AsRef()
	actions := req.GetActions()

	actors, err := s.actors.Repository(auth.UUIDActor)
	if err != nil {
		return nil, status.Errorf(codes.Unimplemented, "get UUIDActor repository: %v", err)
	}

	if err := actors.Use(ctx, actorID, func(a *auth.Actor) error {
		return a.Revoke(target, actions...)
	}); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &emptypb.Empty{}, nil
}

// RevokeFromRole implements authpb.AuthServiceServer.
func (s *Server) RevokeFromRole(ctx context.Context, req *authpb.GrantRevokeReq) (*emptypb.Empty, error) {
	if err := s.checkHandlesCommands(); err != nil {
		return nil, err
	}

	roleID := req.GetRoleOrActor().AsUUID()
	target := req.GetTarget().AsRef()
	actions := req.GetActions()

	if err := s.roles.Use(ctx, roleID, func(r *auth.Role) error {
		return r.Revoke(target, actions...)
	}); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &emptypb.Empty{}, nil
}

func (s *Server) checkHandlesCommands() error {
	if !s.handlesCommands {
		return status.Error(codes.Unimplemented, "command handling not enabled")
	}
	return nil
}

// Client implements auth.QueryClient.
type Client struct{ client authpb.AuthServiceClient }

// NewClient returns the gRPC client for the authorization module.
func NewClient(conn grpc.ClientConnInterface) *Client {
	return &Client{authpb.NewAuthServiceClient(conn)}
}

// Permissions implements auth.QueryClient.
func (c *Client) Permissions(ctx context.Context, actorID uuid.UUID) (auth.PermissionsDTO, error) {
	perms, err := c.client.GetPermissions(ctx, commonpb.NewUUID(actorID))
	return perms.AsDTO(), err
}

// Allows implements auth.QueryClient.
func (c *Client) Allows(ctx context.Context, actorID uuid.UUID, ref aggregate.Ref, action string) (bool, error) {
	resp, err := c.client.Allows(ctx, &authpb.AllowsReq{
		ActorId:   commonpb.NewUUID(actorID),
		Aggregate: aggregatepb.NewRef(ref),
		Action:    action,
	})
	return resp.GetAllowed(), err
}

// LookupActor implements auth.QueryClient.
func (c *Client) LookupActor(ctx context.Context, sid string) (uuid.UUID, error) {
	actorID, err := c.client.LookupActor(ctx, &authpb.LookupActorReq{StringId: sid})
	return actorID.AsUUID(), err
}

// LookupRole implements auth.QueryClient.
func (c *Client) LookupRole(ctx context.Context, name string) (uuid.UUID, error) {
	roleID, err := c.client.LookupRole(ctx, &authpb.LookupRoleReq{Name: name})
	return roleID.AsUUID(), err
}

// GrantToActor implements auth.CommandClient.
func (c *Client) GrantToActor(ctx context.Context, actorID uuid.UUID, ref aggregate.Ref, actions ...string) error {
	_, err := c.client.GrantToActor(ctx, &authpb.GrantRevokeReq{
		RoleOrActor: commonpb.NewUUID(actorID),
		Target:      aggregatepb.NewRef(ref),
		Actions:     actions,
	})
	return err
}

// GrantToRole implements auth.CommandClient.
func (c *Client) GrantToRole(ctx context.Context, roleID uuid.UUID, ref aggregate.Ref, actions ...string) error {
	_, err := c.client.GrantToRole(ctx, &authpb.GrantRevokeReq{
		RoleOrActor: commonpb.NewUUID(roleID),
		Target:      aggregatepb.NewRef(ref),
		Actions:     actions,
	})
	return err
}

// RevokeFromActor implements auth.CommandClient.
func (c *Client) RevokeFromActor(ctx context.Context, actorID uuid.UUID, ref aggregate.Ref, actions ...string) error {
	_, err := c.client.RevokeFromActor(ctx, &authpb.GrantRevokeReq{
		RoleOrActor: commonpb.NewUUID(actorID),
		Target:      aggregatepb.NewRef(ref),
		Actions:     actions,
	})
	return err
}

// RevokeFromRole implements auth.CommandClient.
func (c *Client) RevokeFromRole(ctx context.Context, roleID uuid.UUID, ref aggregate.Ref, actions ...string) error {
	_, err := c.client.RevokeFromRole(ctx, &authpb.GrantRevokeReq{
		RoleOrActor: commonpb.NewUUID(roleID),
		Target:      aggregatepb.NewRef(ref),
		Actions:     actions,
	})
	return err
}
