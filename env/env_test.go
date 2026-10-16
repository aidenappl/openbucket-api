package env

import "testing"

func TestIngestURL(t *testing.T) {
	for in, want := range map[string]string{
		"": "",
		"https://appleby-monitor-api.appleby.cloud":             "https://appleby-monitor-api.appleby.cloud/v1/events",
		"https://appleby-monitor-api.appleby.cloud/":            "https://appleby-monitor-api.appleby.cloud/v1/events",
		"https://appleby-monitor-api.appleby.cloud/v1/events":   "https://appleby-monitor-api.appleby.cloud/v1/events",
		" https://appleby-monitor-api.appleby.cloud/v1/events/": "https://appleby-monitor-api.appleby.cloud/v1/events",
	} {
		if got := ingestURL(in); got != want {
			t.Errorf("ingestURL(%q) = %q, want %q", in, got, want)
		}
	}
}
