package responder

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
)

type ResponseStructure struct {
	Success    bool        `json:"success"`
	Message    string      `json:"message"`
	Data       interface{} `json:"data"`
	Pagination interface{} `json:"pagination,omitempty"`
}

// Truncation is the pagination block for listings capped server-side: the
// client gets the first Limit entries and must say so rather than present
// them as complete.
type Truncation struct {
	Truncated bool `json:"truncated"`
	Limit     int  `json:"limit"`
	// NextCursor continues a truncated listing (GET /contents?cursor=).
	NextCursor string `json:"next_cursor,omitempty"`
}

func New(w http.ResponseWriter, data interface{}, message ...string) {
	write(w, data, nil, message...)
}

// NewWithPagination is New plus a `pagination` block in the envelope.
func NewWithPagination(w http.ResponseWriter, data interface{}, pagination interface{}, message ...string) {
	write(w, data, pagination, message...)
}

func write(w http.ResponseWriter, data interface{}, pagination interface{}, message ...string) {
	// Ensure nil slices are serialized as [] instead of null
	if data != nil {
		v := reflect.ValueOf(data)
		if v.Kind() == reflect.Slice && v.IsNil() {
			data = reflect.MakeSlice(v.Type(), 0, 0).Interface()
		}
	}

	response := ResponseStructure{
		Success:    true,
		Data:       data,
		Pagination: pagination,
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
