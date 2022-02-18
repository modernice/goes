package goes

import (
	"fmt"
)

type ID interface {
	comparable
	fmt.Stringer
}

type SID string

func StringID(id string) SID {
	return SID(id)
}

func (id SID) String() string {
	return string(id)
}

type AID struct {
	ID any
}

func AnyID(id any) AID {
	return AID{id}
}

func (id AID) String() string {
	return fmt.Sprint(id.ID)
}

func UnwrapID[Out ID, In ID](id In) Out {
	aid := any(id)
	if id, ok := aid.(Out); ok {
		return id
	}

	if id, ok := aid.(AID); ok {
		if id, ok := id.ID.(Out); ok {
			return id
		}
		var zero Out
		panic(fmt.Errorf("[goes.ResolveID] %T is not a %T", id.ID, zero))
	}

	var zero Out
	return zero
}
