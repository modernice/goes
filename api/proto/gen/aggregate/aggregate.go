package aggregatepb

import (
	"github.com/modernice/goes/aggregate"
	commonpb "github.com/modernice/goes/api/proto/gen/common"
)

// NewRef convers an aggregate.Ref to a *Ref.
func NewRef(ref aggregate.Ref) *Ref {
	return &Ref{
		Name: ref.Name,
		Id:   commonpb.NewUUID(ref.ID),
	}
}

// AsRef converts the *Ref to an aggregate.Ref.
func (ref *Ref) AsRef() aggregate.Ref {
	return aggregate.Ref{
		Name: ref.GetName(),
		ID:   ref.GetId().AsUUID(),
	}
}
