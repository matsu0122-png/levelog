// Package httpx contains small HTTP helpers shared by every handler:
// consistent JSON responses, error translation, and request decoding.
package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"levelog/backend/internal/apperror"
	"levelog/backend/internal/reqid"
)

func WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("write json response", "err", err)
	}
}

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// WriteError translates a service/repository error into an HTTP response.
// *apperror.Error carries its own status code and message; a context
// deadline/cancellation (the request-scoped timeout set by
// middleware.Timeout expiring, usually while waiting on the database) maps
// to 504; anything else is treated as an unexpected internal error, logged
// with the request's correlation ID, and never leaked to the client.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	ctx := r.Context()

	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		slog.WarnContext(ctx, "request timeout", "request_id", reqid.FromContext(ctx), "err", err)
		WriteJSON(w, http.StatusGatewayTimeout, errorBody{Error: errorDetail{Code: "TIMEOUT", Message: "リクエストがタイムアウトしました"}})
		return
	}

	var aerr *apperror.Error
	if errors.As(err, &aerr) {
		if aerr.Status >= http.StatusInternalServerError {
			slog.ErrorContext(ctx, "handler error", "request_id", reqid.FromContext(ctx), "code", aerr.Code, "err", err)
		}
		WriteJSON(w, aerr.Status, errorBody{Error: errorDetail{Code: aerr.Code, Message: aerr.Message}})
		return
	}

	slog.ErrorContext(ctx, "internal error", "request_id", reqid.FromContext(ctx), "err", err)
	WriteJSON(w, http.StatusInternalServerError, errorBody{Error: errorDetail{Code: "INTERNAL", Message: "予期しないエラーが発生しました"}})
}

// DecodeJSON reads and decodes a JSON request body, rejecting unknown
// fields and returning a client-facing *apperror.Error on failure.
func DecodeJSON(r *http.Request, v interface{}) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return apperror.BadRequest("リクエストの形式が正しくありません")
	}
	return nil
}
