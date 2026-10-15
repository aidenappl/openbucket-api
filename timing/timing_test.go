package timing

import (
	"context"
	"regexp"
	"strconv"
	"testing"
	"time"
)

func TestTrackWithoutRecorderIsNoop(t *testing.T) {
	stop := Track(context.Background(), "storage")
	stop()
	stop()
}

func TestHeader(t *testing.T) {
	tests := []struct {
		name  string
		spans []string
		want  string
	}{
		{"total only", nil, `^total;dur=\d+\.\d$`},
		{"one span", []string{"auth"}, `^auth;dur=\d+\.\d, total;dur=\d+\.\d$`},
		{"same name summed, first-seen order", []string{"auth", "storage", "storage"}, `^auth;dur=\d+\.\d, storage;dur=\d+\.\d, total;dur=\d+\.\d$`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, rec := WithRecorder(context.Background())
			for _, s := range tt.spans {
				Track(ctx, s)()
			}
			if got := rec.Header(); !regexp.MustCompile(tt.want).MatchString(got) {
				t.Fatalf("Header() = %q, want match %s", got, tt.want)
			}
		})
	}
}

func TestSumsAndOpenSpans(t *testing.T) {
	ctx, rec := WithRecorder(context.Background())
	for i := 0; i < 2; i++ {
		stop := Track(ctx, "storage")
		time.Sleep(10 * time.Millisecond)
		stop()
		stop() // a second stop must not extend the span
	}
	Track(ctx, "auth") // left open: counts up to now
	time.Sleep(5 * time.Millisecond)

	m := regexp.MustCompile(`storage;dur=(\d+\.\d), auth;dur=(\d+\.\d)`).FindStringSubmatch(rec.Header())
	if m == nil {
		t.Fatalf("unexpected header %q", rec.Header())
	}
	storage, _ := strconv.ParseFloat(m[1], 64)
	auth, _ := strconv.ParseFloat(m[2], 64)
	if storage < 20 {
		t.Errorf("storage should sum both 10ms spans, got %.1fms", storage)
	}
	if auth < 5 {
		t.Errorf("an open span should count up to now, got %.1fms", auth)
	}
}
