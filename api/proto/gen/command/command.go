package commandpb

import (
	"errors"

	"github.com/modernice/goes/command"
	"github.com/modernice/goes/internal/slice"
	"golang.org/x/exp/constraints"
)

// NewError returns a new instance of Error, an error type that wraps a
// command.Err and adds support for error details. The function takes a pointer
// to a command.Err as input and returns a pointer to an Error. If the input is
// nil, NewError returns an empty Error. The returned Error has its Code,
// Message, and Details fields set to the corresponding values of the input
// command.Err. NewError is generic over the integer type of the Code field of
// the input command.Err.
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

// AsError returns the *command.Err[int] representation of an *Error.
func (err *Error) AsError() *command.Err[int] {
	return AsError[int](err)
}

// AsError returns a command.Err value that wraps an Error.
func AsError[Code constraints.Integer](err *Error) *command.Err[Code] {
	if err == nil {
		return command.NewError[Code](0, nil)
	}
	details := slice.Map(err.Details, (*ErrorDetail).AsErrorDetail)
	return command.NewError(Code(err.GetCode()), errors.New(err.GetMessage()), command.WithErrorDetails(details...))
}

// NewErrorDetail creates a new ErrorDetail from a command.ErrDetail. The
// created ErrorDetail is used to add additional details to an Error.
func NewErrorDetail(detail *command.ErrDetail) *ErrorDetail {
	return &ErrorDetail{
		Detail: detail.AsAny(),
	}
}

// AsErrorDetail converts an *ErrorDetail to a *command.ErrDetail.
func (detail *ErrorDetail) AsErrorDetail() *command.ErrDetail {
	out, err := command.NewErrorDetail(detail.GetDetail())
	if err != nil {
		panic(err)
	}
	return out
}
