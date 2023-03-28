package command_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/modernice/goes/command"
	"github.com/modernice/goes/internal/slice"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/protobuf/proto"
)

var errUnderlying = errors.New("underlying")

func TestNewError(t *testing.T) {
	err := command.NewError(0, errUnderlying)

	if err.Code() != 0 {
		t.Fatalf("expected code 0, got %d", err.Code())
	}

	if err.Underlying() != errUnderlying {
		t.Fatalf("expected underlying error %q, got %q", errUnderlying, err.Underlying())
	}

	if err.Unwrap() != errUnderlying {
		t.Fatalf("expected to unwrap underlying error %q, got %q", errUnderlying, err.Unwrap())
	}

	if err.Error() != errUnderlying.Error() {
		t.Fatalf("expected error message %q, got %q", errUnderlying.Error(), err.Error())
	}
}

type errorCode int

func TestNewError_customCodeType(t *testing.T) {
	code := errorCode(3)
	err := command.NewError(code, errUnderlying)

	if err.Code() != code {
		t.Fatalf("expected code %d, got %d", code, err.Code())
	}
}

func TestNewError_underlyingDetails(t *testing.T) {
	underlying := &errorWithDetails{
		details: []*command.ErrDetail{
			command.LocalizeError("en", "Localized message"),
			command.LocalizeError("de", "Lokalisierter Text"),
		},
	}

	cerr := command.NewError(0, underlying)

	if len(cerr.Details()) != 2 {
		t.Fatalf("expected error to have 2 details, got %d", len(cerr.Details()))
	}

	for i, d := range cerr.Details() {
		v, err := d.Value()
		if err != nil {
			t.Fatalf("get detail value: %v", err)
		}

		uv, err := underlying.details[i].Value()
		if err != nil {
			t.Fatalf("get underlying detail value: %v", err)
		}

		if !proto.Equal(uv, v) {
			t.Fatalf("expected detail #%d to be equal to original detail", i)
		}
	}
}

func TestErr_Details(t *testing.T) {
	detailMsgs := []proto.Message{
		&errdetails.LocalizedMessage{
			Locale:  "en",
			Message: "Localized message",
		},
		&errdetails.LocalizedMessage{
			Locale:  "de",
			Message: "Lokalisierter Text",
		},
	}

	details, err := slice.MapErr(detailMsgs, command.NewErrorDetail)
	if err != nil {
		t.Fatalf("create details: %v", err)
	}

	cerr := command.NewError(0, errUnderlying, command.WithErrorDetails(details...))

	for i, d := range cerr.Details() {
		org := detailMsgs[i]

		v, err := d.Value()
		if err != nil {
			t.Fatalf("get detail value: %v", err)
		}

		if !proto.Equal(org, v) {
			t.Fatalf("expected detail #%d to be equal to original detail", i)
		}
	}
}

func TestErr_WithDetails(t *testing.T) {
	err := command.NewError(0, errUnderlying)

	d, derr := command.NewErrorDetail(&errdetails.LocalizedMessage{
		Locale:  "en",
		Message: "Localized message",
	})
	if derr != nil {
		t.Fatalf("create error detail: %v", derr)
	}

	err = err.WithDetails(d)

	if len(err.Details()) != 1 {
		t.Fatalf("expected error to have 1 detail, got %d", len(err.Details()))
	}

	if err.Details()[0] != d {
		t.Fatalf("expected error to have detail %v, got %v", d, err.Details()[0])
	}
}

func TestErr_LocalizedMessage(t *testing.T) {
	locale := "en"
	msg := "Localized message"

	err := command.NewError(0, errUnderlying, command.WithErrorDetails(command.LocalizeError(locale, msg)))

	if len(err.Details()) != 1 {
		t.Fatalf("expected error to have 1 detail, got %d", len(err.Details()))
	}

	d := err.Details()[0]
	v, verr := d.Value()
	if verr != nil {
		t.Fatalf("get detail value: %v", verr)
	}

	want := &errdetails.LocalizedMessage{
		Locale:  locale,
		Message: msg,
	}

	lm, ok := v.(*errdetails.LocalizedMessage)
	if !ok || !proto.Equal(want, lm) {
		t.Fatalf("expected detail to be %v, got %v", want, lm)
	}

	got := err.Localized(locale)
	if got != msg {
		t.Fatalf("expected LocalizedMessage to be %q, got %q", msg, got)
	}
}

func TestError_isCommandError(t *testing.T) {
	code := errorCode(3)
	cerr := command.NewError(
		code,
		errUnderlying,
		command.WithErrorDetails(command.LocalizeError("en", "Localized message")),
	)

	parsed := command.Error[errorCode](cerr)

	if parsed != cerr {
		t.Fatalf("expected parsed error to be %v, got %v", cerr, parsed)
	}

	msg := parsed.Localized("en")
	if msg != "Localized message" {
		t.Fatalf("expected LocalizedMessage(%q) to return %q, got %q", "en", "Localized message", msg)
	}
}

func TestError_nonCommandError(t *testing.T) {
	parsed := command.Error[int](errUnderlying)

	if parsed.Code() != 0 {
		t.Fatalf("expected code 0, got %d", parsed.Code())
	}

	if parsed.Underlying() != errUnderlying {
		t.Fatalf("expected underlying error %q, got %q", errUnderlying, parsed.Underlying())
	}
}

func TestError_nonCommandErrorWithCode(t *testing.T) {
	underlying := &errorWithCode{code: 10}
	parsed := command.Error[errorCode](underlying)

	if parsed.Code() != 10 {
		t.Fatalf("expected code 10, got %d", parsed.Code())
	}

	if parsed.Underlying() != underlying {
		t.Fatalf("expected underlying error %q, got %q", underlying, parsed.Underlying())
	}
}

type errorWithDetails struct {
	details []*command.ErrDetail
}

func (err *errorWithDetails) Error() string {
	return "error with details"
}

func (err *errorWithDetails) Details() []*command.ErrDetail {
	return err.details
}

type errorWithCode struct {
	code errorCode
}

func (err *errorWithCode) Error() string {
	return fmt.Sprintf("error with code %d", err.code)
}

func (err *errorWithCode) Code() errorCode {
	return err.code
}
