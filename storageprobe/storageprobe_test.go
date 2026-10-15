package storageprobe

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCheck(t *testing.T) {
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

func TestRoundChecksPublicAndLAN(t *testing.T) {
	var got []Result
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	Round(context.Background(), []string{srv.URL}, func(r Result) { got = append(got, r) })
	// No LAN mapping configured in tests: one public check per endpoint.
	if len(got) != 1 || got[0].Via != "public" || got[0].Endpoint != srv.URL || !got[0].OK() {
		t.Fatalf("got %+v", got)
	}
}
