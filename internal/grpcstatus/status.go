package grpcstatus

import (
	protov1 "github.com/golang/protobuf/proto"
	"github.com/modernice/goes/internal/slice"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// New returns a new grpc *Status with the provided details and ignores any
// errors that happen while adding the details.
func New(code codes.Code, message string, details ...proto.Message) *status.Status {
	st := status.New(code, message)
	if len(details) > 0 {
		detailsv1 := slice.Map(details, func(msg proto.Message) protov1.Message {
			return protov1.MessageV1(msg)
		})

		st, _ = st.WithDetails(detailsv1...)
	}
	return st
}
