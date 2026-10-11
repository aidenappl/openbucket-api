package routers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/aidenappl/openbucket-api/sso"
)

// HandleSSOConfig returns the public SSO configuration for the frontend login page.
func HandleSSOConfig(w http.ResponseWriter, r *http.Request) {
	cfg := sso.Config()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"success":true,"data":%s}`, mustJSON(cfg))
}

func mustJSON(v map[string]any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}
