package responder

import (
	"encoding/json"
	"net/http"
	"strings"
)

type ResponseStructure struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func New(w http.ResponseWriter, data interface{}, message ...string) {
	response := ResponseStructure{
		Success: true,
		Data:    data,
	}

	if len(message) > 0 {
		response.Message = message[0]
	} else {
		response.Message = "request was successful"
	}

	// set message to lowercase
	response.Message = strings.ToLower(response.Message)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
