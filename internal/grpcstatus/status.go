package grpcstatus

import (
	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// New returns a new grpc *Status with the provided details and ignores any
// errors that happen while adding the details.
func New(code codes.Code, message string, details ...proto.Message) *status.Status {
	st := status.New(code, message)
	if len(details) > 0 {
		st, _ = st.WithDetails(details...)
	}
	return st
}
