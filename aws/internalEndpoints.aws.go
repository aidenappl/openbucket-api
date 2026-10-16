package aws

import (
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// internalEndpoints maps a session's public storage endpoint to a LAN
// address of the same storage server, e.g.
// "https://cdn.appleby.cloud" -> "http://10.15.10.222:8080".
//
// The public endpoints sit behind Cloudflare while the API and the storage
// servers share a LAN, so every API -> storage call left the network and came
// back: ~135 ms per listing page and ~550 ms of a 1 s listing of
// atlas/hotels/. SigV4 signs the Host actually used, so signatures hold.
// Presigned URLs still use the public endpoint (GetPublicS3Client).
var (
	internalMu        sync.RWMutex
	internalEndpoints = map[string]string{}
)

// probe reports whether something answers HTTP at u. Any status counts (the
// storage server answers an unsigned GET / with 403).
var probe = func(u string) bool {
	c := http.Client{Timeout: 3 * time.Second}
	resp, err := c.Get(u + "/")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return true
}

// normalizeEndpoint lowercases, trims a trailing "/" and adds "https://" when
// there is no scheme — older sessions store bare hosts ("cdn.trailblaze.to"),
// which the AWS SDK treats as https.
func normalizeEndpoint(e string) string {
	e = strings.ToLower(strings.TrimRight(strings.TrimSpace(e), "/"))
	if e != "" && !strings.Contains(e, "://") {
		e = "https://" + e
	}
	return e
}

// NormalizeEndpoint is normalizeEndpoint, for the storage prober.
func NormalizeEndpoint(e string) string { return normalizeEndpoint(e) }

// InitInternalEndpoints parses OB_STORAGE_INTERNAL_ENDPOINTS
// ("public=internal,public=internal") and keeps the entries whose internal
// address answers now. An unreachable or malformed entry is logged and
// skipped, so a bad setting falls back to the public endpoint instead of
// breaking storage.
func InitInternalEndpoints(spec string) {
	m := map[string]string{}
	for _, pair := range strings.Split(spec, ",") {
		if strings.TrimSpace(pair) == "" {
			continue
		}
		pub, internal, ok := strings.Cut(pair, "=")
		pub, internal = normalizeEndpoint(pub), normalizeEndpoint(internal)
		if !ok || pub == "" || internal == "" {
			log.Printf("storage: ignoring malformed OB_STORAGE_INTERNAL_ENDPOINTS entry %q", pair)
			continue
		}
		if !probe(internal) {
			log.Printf("storage: %s unreachable, %s stays on its public endpoint", internal, pub)
			continue
		}
		m[pub] = internal
		log.Printf("storage: %s -> %s (LAN)", pub, internal)
	}
	internalMu.Lock()
	internalEndpoints = m
	internalMu.Unlock()
}

// InternalEndpoint is internalEndpoint, for the storage prober.
func InternalEndpoint(endpoint string) string { return internalEndpoint(endpoint) }

// internalEndpoint returns the LAN address for endpoint, or endpoint itself.
func internalEndpoint(endpoint string) string {
	internalMu.RLock()
	defer internalMu.RUnlock()
	if v, ok := internalEndpoints[normalizeEndpoint(endpoint)]; ok {
		return v
	}
	return endpoint
}
