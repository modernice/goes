package command

import (
	"errors"
	"fmt"

	"golang.org/x/exp/constraints"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

type Err[Code constraints.Integer] struct {
	code       Code
	underlying error
	details    []*ErrDetail
}

type ErrDetail struct {
	pb    *anypb.Any
	value proto.Message // cached value from pb.UnmarshalNew()
}

type CodedError[Code constraints.Integer] interface {
	Code() Code
}

type DetailedError interface {
	Details() []*ErrDetail
}

type LocalizedMessage interface {
	GetLocale() string
	GetMessage() string
}

func LocalizeError(locale, msg string) *ErrDetail {
	d, _ := NewErrorDetail(&errdetails.LocalizedMessage{
		Locale:  locale,
		Message: msg,
	})
	return d
}

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

func (detail *ErrDetail) AsAny() *anypb.Any {
	return detail.pb
}

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

func (detail *ErrDetail) UnmarshalNew() (proto.Message, error) {
	return detail.pb.UnmarshalNew()
}

type ErrorOption func(*errorOptions)

type errorOptions struct {
	details []*ErrDetail
}

func WithErrorDetails(details ...*ErrDetail) ErrorOption {
	return func(opts *errorOptions) {
		opts.details = append(opts.details, details...)
	}
}

func Error[Code constraints.Integer](err error) *Err[Code] {
	if err == nil {
		return nil
	}

	var cerr *Err[Code]
	if errors.As(err, &cerr) {
		return cerr
	}

	var (
		code  Code
		coded CodedError[Code]
	)
	if errors.As(err, &coded) {
		code = coded.Code()
	}

	return NewError(code, err)
}

func NewError[Code constraints.Integer](code Code, underlying error, opts ...ErrorOption) *Err[Code] {
	var baseOpts []ErrorOption
	if derr, ok := underlying.(DetailedError); ok {
		baseOpts = append(baseOpts, WithErrorDetails(derr.Details()...))
	}

	opts = append(baseOpts, opts...)

	var errorOpts errorOptions
	for _, opt := range opts {
		opt(&errorOpts)
	}

	return &Err[Code]{
		code:       code,
		underlying: underlying,
		details:    errorOpts.details,
	}
}

func (err *Err[Code]) Error() string {
	if err.underlying != nil {
		return err.underlying.Error()
	}
	return fmt.Sprintf("<ERROR CODE %d>", err.code)
}

func (err *Err[Code]) Code() Code {
	return err.code
}

func (err *Err[Code]) Underlying() error {
	return err.underlying
}

func (err *Err[Code]) Unwrap() error {
	return err.underlying
}

func (err *Err[Code]) Details() []*ErrDetail {
	return err.details
}

func (err *Err[Code]) WithDetails(details ...*ErrDetail) *Err[Code] {
	return NewError(err.code, err.underlying, WithErrorDetails(append(err.details, details...)...))
}

func (err *Err[Code]) Localized(locale string) string {
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
