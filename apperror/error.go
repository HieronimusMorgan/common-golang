// Package apperror separates public API messages from internal causes.
package apperror

import (
	"errors"
	"net/http"
)

type Code string

const (
	InvalidArgument Code = "INVALID_ARGUMENT"
	Unauthenticated Code = "UNAUTHENTICATED"
	Forbidden       Code = "FORBIDDEN"
	NotFound        Code = "NOT_FOUND"
	Conflict        Code = "CONFLICT"
	TooManyRequests Code = "TOO_MANY_REQUESTS"
	Unavailable     Code = "UNAVAILABLE"
	Internal        Code = "INTERNAL"
)

type Error struct {
	code    Code
	message string
	cause   error
}

func New(code Code, publicMessage string) *Error { return Wrap(code, publicMessage, nil) }
func Wrap(code Code, publicMessage string, cause error) *Error {
	if Status(code) == http.StatusInternalServerError {
		code = Internal
		publicMessage = "Internal server error"
	}
	if publicMessage == "" {
		publicMessage = http.StatusText(Status(code))
	}
	return &Error{code: code, message: publicMessage, cause: cause}
}
func (e *Error) Error() string         { return string(e.code) + ": " + e.message }
func (e *Error) Unwrap() error         { return e.cause }
func (e *Error) Code() Code            { return e.code }
func (e *Error) PublicMessage() string { return e.message }
func (e *Error) Status() int           { return Status(e.code) }
func Status(code Code) int {
	switch code {
	case InvalidArgument:
		return 400
	case Unauthenticated:
		return 401
	case Forbidden:
		return 403
	case NotFound:
		return 404
	case Conflict:
		return 409
	case TooManyRequests:
		return 429
	case Unavailable:
		return 503
	default:
		return 500
	}
}
func From(err error) *Error {
	var e *Error
	if errors.As(err, &e) && e != nil {
		return e
	}
	return Wrap(Internal, "", err)
}
