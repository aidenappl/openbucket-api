package routers

import "testing"

func TestPresignExpiration(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    int64
		wantErr bool
	}{
		{"empty uses the 1h default", "", 3600, false},
		{"within range is kept", "900", 900, false},
		{"exactly the max is kept", "86400", 86400, false},
		{"over the max is clamped, not reset to 1h", "604800", 86400, false},
		{"zero is rejected", "0", 0, true},
		{"negative is rejected", "-5", 0, true},
		{"duration strings are rejected", "24h", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := presignExpiration(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("presignExpiration(%q) err = %v, wantErr %v", tt.raw, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Fatalf("presignExpiration(%q) = %d, want %d", tt.raw, got, tt.want)
			}
		})
	}
}
