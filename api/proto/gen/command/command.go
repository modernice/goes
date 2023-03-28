package commandpb

import (
	"errors"

	"github.com/modernice/goes/command"
	"github.com/modernice/goes/internal/slice"
	"golang.org/x/exp/constraints"
)

func NewError[Code constraints.Integer](err *command.Err[Code]) *Error {
	if err == nil {
		return &Error{}
	}
	return &Error{
		Code:    int64(err.Code()),
		Message: err.Error(),
		Details: slice.Map(err.Details(), NewErrorDetail),
	}
}

func (err *Error) AsError() *command.Err[int] {
	return AsError[int](err)
}

func AsError[Code constraints.Integer](err *Error) *command.Err[Code] {
	if err == nil {
		return command.NewError[Code](0, nil)
	}
	details := slice.Map(err.Details, (*ErrorDetail).AsErrorDetail)
	return command.NewError(Code(err.GetCode()), errors.New(err.GetMessage()), command.WithErrorDetails(details...))
}

func NewErrorDetail(detail *command.ErrDetail) *ErrorDetail {
	return &ErrorDetail{
		Detail: detail.AsAny(),
	}
}

func (detail *ErrorDetail) AsErrorDetail() *command.ErrDetail {
	out, err := command.NewErrorDetail(detail.GetDetail())
	if err != nil {
		panic(err)
	}
	return out
}
