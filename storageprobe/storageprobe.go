// Package storageprobe checks every storage server this API uses and reports
// each check to Monitor as a `storage.health` event, so a storage outage
// raises an alert.
//
// ⚠️ On 2026-10-15 cdn.trailblaze.to hung for ~15 hours and nothing noticed:
// no OpenBucket service reported to Monitor, and the storage server still
// answered requests that don't touch its database (openbucket-go#33). The
// probe hits openbucket-go's GET /_health, which is 200 only when a pooled
// database connection works within 2s.
//
// Each endpoint is checked two ways: over the LAN (the address this API uses,
// when OB_STORAGE_INTERNAL_ENDPOINTS maps it) and through its public URL (the
// path CDN users take, via Cloudflare).
package storageprobe

import (
	"context"
	"log"
	"net/http"
	"time"

	monitor "github.com/aidenappl/go-monitor"
	"github.com/aidenappl/openbucket-api/aws"
	"github.com/aidenappl/openbucket-api/db"
	"github.com/aidenappl/openbucket-api/query"
)

// Interval between rounds of checks.
const Interval = 30 * time.Second

// Result is one check of one storage server.
type Result struct {
	Endpoint  string `json:"endpoint"` // the public endpoint sessions use
	Via       string `json:"via"`      // "lan" or "public"
	URL       string `json:"url"`
	Status    int    `json:"status"` // 0 when no response
	LatencyMS int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

// OK reports a 200 from /_health.
func (r Result) OK() bool { return r.Status == http.StatusOK }

var client = &http.Client{Timeout: 5 * time.Second}

// Check probes base + "/_health".
func Check(ctx context.Context, endpoint, via, base string) Result {
	r := Result{Endpoint: endpoint, Via: via, URL: base + "/_health"}
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.URL, nil)
	if err == nil {
		var resp *http.Response
		resp, err = client.Do(req)
		if err == nil {
			r.Status = resp.StatusCode
			resp.Body.Close()
		}
	}
	r.LatencyMS = time.Since(start).Milliseconds()
	if err != nil {
		r.Error = err.Error()
	} else if !r.OK() {
		r.Error = http.StatusText(r.Status)
	}
	return r
}

// Targets returns the checks for one endpoint: public, plus LAN when mapped.
func Targets(endpoint string) [][2]string {
	t := [][2]string{{"public", endpoint}}
	if lan := aws.InternalEndpoint(endpoint); lan != endpoint {
		t = append(t, [2]string{"lan", lan})
	}
	return t
}

// Round checks every endpoint once and reports each result through emit.
// Endpoints are normalised ("cdn.example" = "https://cdn.example") and each
// is checked once.
func Round(ctx context.Context, endpoints []string, emit func(Result)) {
	seen := map[string]bool{}
	for _, raw := range endpoints {
		e := aws.NormalizeEndpoint(raw)
		if e == "" || seen[e] {
			continue
		}
		seen[e] = true
		for _, t := range Targets(e) {
			emit(Check(ctx, e, t[0], t[1]))
		}
	}
}

// Report sends a result to Monitor: info when healthy, error otherwise.
func Report(r Result) {
	if r.OK() {
		monitor.Info(context.Background(), "storage.health", r)
		return
	}
	log.Printf("storage: %s via %s unhealthy: %s", r.Endpoint, r.Via, r.Error)
	monitor.Error(context.Background(), "storage.health", r)
}

// Start runs a round every Interval until ctx ends. Endpoints are re-read
// from the sessions table each round, so a new storage server is covered.
func Start(ctx context.Context) {
	go func() {
		t := time.NewTicker(Interval)
		defer t.Stop()
		for {
			endpoints, err := query.ListSessionEndpoints(db.DB)
			if err != nil {
				log.Printf("storage: probe could not list endpoints: %v", err)
			} else {
				Round(ctx, endpoints, Report)
			}
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}()
}
