package aws

import "testing"

func TestInternalEndpoints(t *testing.T) {
	orig := probe
	t.Cleanup(func() { probe = orig; InitInternalEndpoints("") })
	probe = func(u string) bool { return u != "http://10.0.0.9:8080" }

	InitInternalEndpoints(" https://cdn.appleby.cloud/ = http://10.15.10.222:8080 ," +
		"https://cdn.trailblaze.to=http://10.0.0.9:8080, garbage ,=http://x")

	tests := []struct{ in, want string }{
		{"https://cdn.appleby.cloud", "http://10.15.10.222:8080"},
		{"https://CDN.appleby.cloud/", "http://10.15.10.222:8080"},
		{"https://cdn.trailblaze.to", "https://cdn.trailblaze.to"}, // unreachable: kept public
		{"https://other.example", "https://other.example"},
		{"cdn.appleby.cloud", "http://10.15.10.222:8080"}, // bare host = https
	}
	for _, tt := range tests {
		if got := internalEndpoint(tt.in); got != tt.want {
			t.Errorf("internalEndpoint(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}

	InitInternalEndpoints("")
	if got := internalEndpoint("https://cdn.appleby.cloud"); got != "https://cdn.appleby.cloud" {
		t.Errorf("empty spec should map nothing, got %q", got)
	}
}
