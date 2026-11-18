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
//
// ⚠️ The public check hairpins: this API's host goes out through the site's
// WAN to Cloudflare and back in to NPM. In bursts (10-28, 10-29, 11-05,
// 11-11) single requests on that path never reached NPM for minutes at a time
// while NPM served everyone else normally and the LAN checks stayed green
// (APL-182). So a check is reported as an error only after it has failed for
// FailThreshold rounds in a row (Tracker); shorter runs are warnings.
package storageprobe

import (
	"context"
	"log"
	"math/rand/v2"
	"net/http"
	"sync"
	"time"

	monitor "github.com/aidenappl/go-monitor"
	"github.com/aidenappl/openbucket-api/aws"
	"github.com/aidenappl/openbucket-api/db"
	"github.com/aidenappl/openbucket-api/query"
)

// Interval between rounds of checks; each round waits Interval plus up to
// JITTER, so the probe doesn't fire in lockstep with anything else on a
// 30-second clock.
const (
	Interval = 30 * time.Second
	JITTER   = 5 * time.Second
)

// Rounds a check must fail in a row before it is an error (APL-182). A public
// stall on the hairpin path lasts under a minute or two; three rounds is
// about 1.5 minutes. The LAN path has no Cloudflare in it, so two.
const (
	PUBLIC_FAIL_THRESHOLD = 3
	LAN_FAIL_THRESHOLD    = 2
)

// Result is one check of one storage server.
type Result struct {
	Endpoint  string `json:"endpoint"` // the public endpoint sessions use
	Via       string `json:"via"`      // "lan" or "public"
	URL       string `json:"url"`
	Status    int    `json:"status"` // 0 when no response
	LatencyMS int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
	Attempts  int    `json:"attempts"`
	// ConsecutiveFailures counts failed rounds in a row for this endpoint and
	// route, including this one; 0 when healthy.
	ConsecutiveFailures int `json:"consecutive_failures"`
	// Threshold is the run of failures at which the result is an error.
	Threshold int `json:"threshold"`
}

// OK reports a 200 from /_health.
func (r Result) OK() bool { return r.Status == http.StatusOK }

var client = &http.Client{Timeout: 5 * time.Second}

// RetryDelay is the pause before re-checking a failed endpoint. A single
// failure is usually a Cloudflare connection reset, not an outage, so a
// result is unhealthy only when two checks in a row fail.
var RetryDelay = 2 * time.Second

// Check probes base + "/_health", retrying once after RetryDelay on failure.
func Check(ctx context.Context, endpoint, via, base string) Result {
	r := checkOnce(ctx, endpoint, via, base)
	r.Attempts = 1
	if r.OK() {
		return r
	}
	select {
	case <-ctx.Done():
		return r
	case <-time.After(RetryDelay):
	}
	r = checkOnce(ctx, endpoint, via, base)
	r.Attempts = 2
	return r
}

func checkOnce(ctx context.Context, endpoint, via, base string) Result {
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

// Level is how a result is reported.
type Level string

const (
	LEVEL_INFO  Level = "info"  // healthy
	LEVEL_WARN  Level = "warn"  // failing, but not for FailThreshold rounds yet
	LEVEL_ERROR Level = "error" // failing for at least FailThreshold rounds in a row
)

// FailThreshold is the number of failed rounds in a row that makes via's
// result an error.
func FailThreshold(via string) int {
	if via == "lan" {
		return LAN_FAIL_THRESHOLD
	}
	return PUBLIC_FAIL_THRESHOLD
}

// Tracker remembers consecutive failures per endpoint and route.
type Tracker struct {
	mu    sync.Mutex
	fails map[string]int
}

func NewTracker() *Tracker { return &Tracker{fails: map[string]int{}} }

// Observe records r, fills in its ConsecutiveFailures and Threshold, and
// returns the level to report it at.
func (t *Tracker) Observe(r *Result) Level {
	t.mu.Lock()
	defer t.mu.Unlock()
	key := r.Endpoint + " " + r.Via
	r.Threshold = FailThreshold(r.Via)
	if r.OK() {
		delete(t.fails, key)
		r.ConsecutiveFailures = 0
		return LEVEL_INFO
	}
	t.fails[key]++
	r.ConsecutiveFailures = t.fails[key]
	if r.ConsecutiveFailures >= r.Threshold {
		return LEVEL_ERROR
	}
	return LEVEL_WARN
}

// Reporter sends results to Monitor at the level its Tracker decides.
func Reporter(t *Tracker) func(Result) {
	return func(r Result) {
		switch t.Observe(&r) {
		case LEVEL_INFO:
			monitor.Info(context.Background(), "storage.health", r)
		case LEVEL_WARN:
			monitor.Warn(context.Background(), "storage.health", r)
		default:
			log.Printf("storage: %s via %s unhealthy for %d rounds: %s", r.Endpoint, r.Via, r.ConsecutiveFailures, r.Error)
			monitor.Error(context.Background(), "storage.health", r)
		}
	}
}

// nextDelay is Interval plus a random part of JITTER.
func nextDelay() time.Duration {
	return Interval + time.Duration(rand.Int64N(int64(JITTER)))
}

// Start runs a round every Interval (plus jitter) until ctx ends. Endpoints are re-read
// from the sessions table each round, so a new storage server is covered.
func Start(ctx context.Context) {
	report := Reporter(NewTracker())
	go func() {
		for {
			endpoints, err := query.ListSessionEndpoints(db.DB)
			if err != nil {
				log.Printf("storage: probe could not list endpoints: %v", err)
			} else {
				Round(ctx, endpoints, report)
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(nextDelay()):
			}
		}
	}()
}
