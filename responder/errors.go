package responder

import (
	"encoding/json"
	"net/http"
)

type ErrorResponse struct {
	Error        any    `json:"error"`
	ErrorMessage string `json:"error_message"`
	ErrorCode    int    `json:"error_code"`
}

func SendError(w http.ResponseWriter, status int, errMessage string, err ...error) {
	errResp := ErrorResponse{
		Error:        nil,
		ErrorMessage: errMessage,
		ErrorCode:    1000,
	}
	if len(err) > 0 && err[0] != nil {
		errResp.Error = err[0].Error()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(errResp)
}

func SendErrorWithParams(w http.ResponseWriter, err string, status int, errorCode *int, errorMessage *string) {
	errResp := ErrorResponse{
		Error:        err,
		ErrorMessage: "",
		ErrorCode:    1000,
	}

	if errorCode != nil && *errorCode > 0 {
		errResp.ErrorCode = *errorCode
	}

	if errorMessage != nil && len(*errorMessage) > 0 {
		errResp.ErrorMessage = *errorMessage
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(errResp)
}

func ErrMissingParam(w http.ResponseWriter, field string) {
	SendError(w, http.StatusBadRequest, "missing required param: "+field)
}

func ErrMissingKey(w http.ResponseWriter, key string) {
	SendError(w, http.StatusBadRequest, "missing required key: "+key)
}
