package storageprobe

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCheck(t *testing.T) {
	RetryDelay = 10 * time.Millisecond
	healthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/_health" {
			t.Errorf("probed %s, want /_health", r.URL.Path)
		}
		w.Write([]byte("ok"))
	}))
	defer healthy.Close()
	sick := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer sick.Close()
	hung := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(6 * time.Second) // longer than the client timeout
	}))
	defer hung.Close()

	tests := []struct {
		name, url  string
		ok         bool
		status     int
		wantErr    bool
		maxLatency time.Duration
	}{
		{"healthy", healthy.URL, true, 200, false, time.Second},
		{"503 from /_health", sick.URL, false, 503, true, time.Second},
		{"hung server times out", hung.URL, false, 0, true, 6 * time.Second},
		{"unreachable", "http://127.0.0.1:1", false, 0, true, time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := Check(context.Background(), "https://cdn.example", "lan", tt.url)
			if r.OK() != tt.ok || r.Status != tt.status || (r.Error != "") != tt.wantErr {
				t.Fatalf("got %+v", r)
			}
			if time.Duration(r.LatencyMS)*time.Millisecond > tt.maxLatency {
				t.Fatalf("took %dms", r.LatencyMS)
			}
		})
	}
}

func TestCheckRetriesOnce(t *testing.T) {
	RetryDelay = 10 * time.Millisecond
	calls := 0
	flaky := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusBadGateway)
		}
	}))
	defer flaky.Close()
	r := Check(context.Background(), "https://cdn.example", "public", flaky.URL)
	if !r.OK() || r.Attempts != 2 || calls != 2 {
		t.Fatalf("got %+v after %d calls, want healthy on the retry", r, calls)
	}
}

func TestRoundChecksPublicAndLAN(t *testing.T) {
	var got []Result
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	// Duplicates (case, trailing slash) are checked once.
	Round(context.Background(), []string{srv.URL, srv.URL + "/", strings.ToUpper(srv.URL[:4]) + srv.URL[4:]}, func(r Result) { got = append(got, r) })
	// No LAN mapping configured in tests: one public check per endpoint.
	if len(got) != 1 || got[0].Via != "public" || got[0].Endpoint != srv.URL || !got[0].OK() {
		t.Fatalf("got %+v", got)
	}
}

func TestTrackerEscalatesOnlyAfterThreshold(t *testing.T) {
	ok := Result{Endpoint: "https://cdn.example", Status: 200}
	bad := Result{Endpoint: "https://cdn.example", Error: "timeout"}
	tests := []struct {
		name  string
		via   string
		seq   []Result
		want  []Level
		final int
	}{
		{"one public blip is a warning", "public", []Result{ok, bad, ok}, []Level{LEVEL_INFO, LEVEL_WARN, LEVEL_INFO}, 0},
		{"public errors on the third failed round", "public", []Result{bad, bad, bad, bad}, []Level{LEVEL_WARN, LEVEL_WARN, LEVEL_ERROR, LEVEL_ERROR}, 4},
		{"a success resets the run", "public", []Result{bad, bad, ok, bad, bad}, []Level{LEVEL_WARN, LEVEL_WARN, LEVEL_INFO, LEVEL_WARN, LEVEL_WARN}, 2},
		{"lan errors on the second failed round", "lan", []Result{bad, bad}, []Level{LEVEL_WARN, LEVEL_ERROR}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := NewTracker()
			var last Result
			for i, r := range tt.seq {
				r.Via = tt.via
				if got := tr.Observe(&r); got != tt.want[i] {
					t.Fatalf("round %d: level %s, want %s", i+1, got, tt.want[i])
				}
				last = r
			}
			if last.ConsecutiveFailures != tt.final || last.Threshold != FailThreshold(tt.via) {
				t.Fatalf("last = %+v, want %d consecutive failures", last, tt.final)
			}
		})
	}
}

func TestTrackerKeepsRoutesApart(t *testing.T) {
	tr := NewTracker()
	pub := Result{Endpoint: "https://cdn.example", Via: "public", Error: "timeout"}
	lan := Result{Endpoint: "https://cdn.example", Via: "lan", Status: 200}
	other := Result{Endpoint: "https://s3.example", Via: "public", Error: "timeout"}
	tr.Observe(&pub)
	tr.Observe(&lan)
	tr.Observe(&other)
	pub2 := pub
	if tr.Observe(&pub2); pub2.ConsecutiveFailures != 2 {
		t.Fatalf("public run = %d, want 2 (lan and other endpoints are separate)", pub2.ConsecutiveFailures)
	}
}

func TestNextDelayIsJittered(t *testing.T) {
	seen := map[time.Duration]bool{}
	for i := 0; i < 50; i++ {
		d := nextDelay()
		if d < Interval || d >= Interval+JITTER {
			t.Fatalf("delay %s outside [%s, %s)", d, Interval, Interval+JITTER)
		}
		seen[d] = true
	}
	if len(seen) < 2 {
		t.Fatal("no jitter")
	}
}
