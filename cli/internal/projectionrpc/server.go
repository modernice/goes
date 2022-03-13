package projectionrpc

import (
	"context"
	"errors"

	protoprojection "github.com/modernice/goes/api/proto/gen/projection"
	"github.com/modernice/goes/projection"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type server struct {
	protoprojection.UnimplementedProjectionServiceServer

	svc *projection.Service
}

// NewServer returns the projection gRPC server.
func NewServer(svc *projection.Service) protoprojection.ProjectionServiceServer {
	return &server{
		svc: svc,
	}
}

func (s *server) Trigger(ctx context.Context, req *protoprojection.TriggerReq) (*protoprojection.TriggerResp, error) {
	var opts []projection.TriggerOption
	if req.GetReset_() {
		opts = append(opts, projection.Reset(true))
	}

	err := s.svc.Trigger(ctx, req.GetSchedule(), opts...)
	if err == nil {
		return &protoprojection.TriggerResp{Accepted: true}, nil
	}

	if errors.Is(err, projection.ErrUnhandledTrigger) {
		return nil, status.Errorf(
			codes.NotFound,
			"Trigger for %q schedule not accepted. Forgot to register the schedule "+
				"in a projection service?",
			req.GetSchedule(),
		)
	}

	return nil, status.Error(codes.Unknown, err.Error())
}
