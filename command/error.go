package command

import (
	"errors"
	"fmt"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// ErrorCode is the canonical code type for command execution errors.
//
// Error codes are serialized as int64 values when transmitted by a command bus.
// The zero value denotes an unspecified error code.
type ErrorCode int64

// CodedError is an error with an error code.
type CodedError interface {
	Code() ErrorCode
}

// DetailedError is an error with details.
type DetailedError interface {
	Details() []*ErrDetail
}

// LocalizedMessage is a localized error message. This interface is implemented
// by [*errdetails.LocalizedMessage]. You can create a localized message using
// [LocalizeError]:
//
//	var msg LocalizedMessage = &errdetails.LocalizedMessage{
//		Locale:  "en",
//		Message: "hello",
//	}
type LocalizedMessage interface {
	GetLocale() string
	GetMessage() string
}

// Err wraps an error with an error code and optional details.
//
// Err is used to transmit errors from the handler back to the dispatcher,
// including the error code and optional details (e.g. localized messages).
//
// To create an [*Err], call [NewError] or [Error]. [NewError] accepts the error
// code and the underlying error, and returns a new [*Err]. If the underlying
// error implements [DetailedError], the details of the underlying error will be
// applied to the returned [*Err].
//
// [Error] tries to convert an [error] to an [*Err]. If the error is already an
// [*Err], it is returned as is. Otherwise, it first extracts the error code from
// the error, then calls [NewError] with the error code and error. If the provided
// error does not implement [CodedError], the error code is set to 0.
//
// [*Err] implements [CodedError] and [DetailedError].
type Err struct {
	code       ErrorCode
	underlying error
	details    []*ErrDetail
}

// ErrDetail is an error detail. A detail can be an arbitrary protobuf message.
//
// To create an [*ErrDetail], call [NewErrorDetail]:
//
//	d, err := NewErrorDetail(&errdetails.LocalizedMessage{
//		Locale:  "en",
//		Message: "hello",
//	})
//
// You can also use the predefined constructors (e.g. [LocalizeError]):
//
//	d := LocalizeError("en", "hello")
type ErrDetail struct {
	pb    *anypb.Any
	value proto.Message // cached value from pb.UnmarshalNew()
}

// LocalizeError creates a new [*ErrDetail] that contains the provided localized
// error message. The message is encoded as a [*errdetails.LocalizedMessage].
// The [*Err] that contains this detail will return the provided message when
// calling [Err.Localized] with the same locale.
func LocalizeError(locale, msg string) *ErrDetail {
	d, _ := NewErrorDetail(&errdetails.LocalizedMessage{
		Locale:  locale,
		Message: msg,
	})
	return d
}

// NewErrorDetail creates a new [*ErrDetail] from the provided protobuf message.
// If the provided message is not already an [*anypb.Any], it is wrapped in a new
// [*anypb.Any].
func NewErrorDetail(msg proto.Message) (*ErrDetail, error) {
	if a, ok := msg.(*anypb.Any); ok {
		return &ErrDetail{pb: a}, nil
	}

	pb, err := anypb.New(msg)
	if err != nil {
		return nil, err
	}

	return &ErrDetail{pb: pb}, nil
}

// AsAny returns the underlying [*anypb.Any] that contains the detail as a protobuf message.
func (detail *ErrDetail) AsAny() *anypb.Any {
	return detail.pb
}

// Value returns the detail as a protobuf message. Multiple calls to [Value] will
// return the same value.
func (detail *ErrDetail) Value() (proto.Message, error) {
	if detail.value != nil {
		return detail.value, nil
	}
	v, err := detail.UnmarshalNew()
	if err != nil {
		return nil, err
	}
	detail.value = v
	return v, nil
}

// UnmarshalNew unmarshals the detail as a protobuf message. This is similar to
// [Value], but it always unmarshals as a new message, instead of returning the
// cached value.
func (detail *ErrDetail) UnmarshalNew() (proto.Message, error) {
	return detail.pb.UnmarshalNew()
}

// ErrorOption is an option for creating a command error.
type ErrorOption func(*errorOptions)

type errorOptions struct {
	details []*ErrDetail
}

// WithErrorDetails adds details to the error.
func WithErrorDetails(details ...*ErrDetail) ErrorOption {
	return func(opts *errorOptions) {
		opts.details = append(opts.details, details...)
	}
}

// Error converts the error to an [*Err]. If the error is already an [*Err], it
// is returned as is. Otherwise, it first extracts the error code from the error,
// then calls [NewError] with the error code and error. If the provided error
// does not satisfy `errors.As(err, new(CodedError))`, the error code is set to 0.
func Error(err error) *Err {
	if err == nil {
		return nil
	}

	var cerr *Err
	if errors.As(err, &cerr) {
		return cerr
	}

	var (
		code  ErrorCode
		coded CodedError
	)
	if errors.As(err, &coded) {
		code = coded.Code()
	}

	return NewError(code, err)
}

// NewError creates a new [*Err] with the provided error code and underlying
// error. If the underlying error implements [DetailedError], the details of the
// underlying error will be applied to the returned [*Err].
func NewError(code ErrorCode, underlying error, opts ...ErrorOption) *Err {
	var baseOpts []ErrorOption
	if derr, ok := underlying.(DetailedError); ok {
		baseOpts = append(baseOpts, WithErrorDetails(derr.Details()...))
	}

	opts = append(baseOpts, opts...)

	var errorOpts errorOptions
	for _, opt := range opts {
		opt(&errorOpts)
	}

	return &Err{
		code:       code,
		underlying: underlying,
		details:    errorOpts.details,
	}
}

// Error implements [error]. It returns the underlying error's message if it is
// not nil, otherwise it returns a string representation of the error code formatted as:
//
//	fmt.Sprintf("<ERROR CODE %d>", code)
func (err *Err) Error() string {
	if err.underlying != nil {
		return err.underlying.Error()
	}
	return fmt.Sprintf("<ERROR CODE %d>", err.code)
}

// Code returns the error code.
func (err *Err) Code() ErrorCode {
	return err.code
}

// Underlying returns the underlying error.
func (err *Err) Underlying() error {
	return err.underlying
}

// Unwrap returns the underlying error.
func (err *Err) Unwrap() error {
	return err.underlying
}

// Details returns the details of the error.
func (err *Err) Details() []*ErrDetail {
	return err.details
}

// WithDetails returns a new [*Err] with the provided details appended to the
// details of the original error. The returned error will have the same error
// code as the original error but will not be the same instance.
func (err *Err) WithDetails(details ...*ErrDetail) *Err {
	return NewError(err.code, err.underlying, WithErrorDetails(append(err.details, details...)...))
}

// Localized returns the localized message for the given locale.
func (err *Err) Localized(locale string) string {
	for _, detail := range err.details {
		msg, err := detail.Value()
		if err != nil {
			continue
		}

		if lm, ok := msg.(LocalizedMessage); ok && lm.GetLocale() == locale {
			return lm.GetMessage()
		}
	}

	return ""
}
