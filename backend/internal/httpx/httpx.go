// Package httpx contains small HTTP helpers shared by every handler:
// consistent JSON responses, error translation, and request decoding.
package httpx

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"levelog/backend/internal/apperror"
)

func WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write json response: %v", err)
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
// *apperror.Error carries its own status code and message; anything else is
// treated as an unexpected internal error and logged (never leaking details
// to the client).
func WriteError(w http.ResponseWriter, err error) {
	var aerr *apperror.Error
	if errors.As(err, &aerr) {
		WriteJSON(w, aerr.Status, errorBody{Error: errorDetail{Code: aerr.Code, Message: aerr.Message}})
		return
	}
	log.Printf("internal error: %v", err)
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
