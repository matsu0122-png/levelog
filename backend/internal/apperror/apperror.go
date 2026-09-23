// Package apperror defines typed application errors that carry an HTTP
// status code, so handlers can translate service-layer failures into
// responses without re-deriving status codes from error strings.
package apperror

import "net/http"

type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string {
	return e.Message
}

func New(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

func NotFound(message string) *Error {
	return New(http.StatusNotFound, "NOT_FOUND", message)
}

func Forbidden(message string) *Error {
	return New(http.StatusForbidden, "FORBIDDEN", message)
}

func BadRequest(message string) *Error {
	return New(http.StatusBadRequest, "BAD_REQUEST", message)
}

func Unauthorized(message string) *Error {
	return New(http.StatusUnauthorized, "UNAUTHORIZED", message)
}

func Conflict(message string) *Error {
	return New(http.StatusConflict, "CONFLICT", message)
}

func TooManyRequests(message string) *Error {
	return New(http.StatusTooManyRequests, "TOO_MANY_REQUESTS", message)
}

func Internal(message string) *Error {
	return New(http.StatusInternalServerError, "INTERNAL", message)
}
