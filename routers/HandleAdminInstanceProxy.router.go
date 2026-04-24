package routers

import (
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aidenappl/openbucket-api/db"
	"github.com/aidenappl/openbucket-api/query"
	"github.com/aidenappl/openbucket-api/responder"
	"github.com/gorilla/mux"
)

var proxyClient = &http.Client{
	Timeout: 30 * time.Second,
}

// HandleAdminInstanceProxy proxies requests to an openbucket-go instance's admin API.
// Route: /admin/instances/{id}/proxy/{path:.*}
func HandleAdminInstanceProxy(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	proxyPath := vars["path"]

	id, err := strconv.Atoi(idStr)
	if err != nil {
		responder.SendError(w, http.StatusBadRequest, "invalid instance ID")
		return
	}

	inst, err := query.GetInstanceByID(db.DB, id)
	if err != nil {
		responder.SendError(w, http.StatusNotFound, "instance not found")
		return
	}

	if !inst.Active {
		responder.SendError(w, http.StatusBadRequest, "instance is not active")
		return
	}

	// Build the upstream URL
	endpoint := strings.TrimRight(inst.Endpoint, "/")
	targetURL := endpoint + "/admin/" + proxyPath
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}

	// Create the proxied request
	proxyReq, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, r.Body)
	if err != nil {
		responder.SendError(w, http.StatusInternalServerError, "failed to create proxy request")
		return
	}

	proxyReq.Header.Set("Authorization", "Bearer "+inst.AdminToken)
	proxyReq.Header.Set("Content-Type", r.Header.Get("Content-Type"))

	resp, err := proxyClient.Do(proxyReq)
	if err != nil {
		responder.SendError(w, http.StatusBadGateway, "failed to reach instance")
		return
	}
	defer resp.Body.Close()

	// Forward the response headers and body
	for key, values := range resp.Header {
		for _, v := range values {
			w.Header().Add(key, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Printf("proxy response copy error: %v", err)
	}
}
