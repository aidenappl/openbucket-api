// Package timing records named spans within one request and renders them as a
// Server-Timing header (shown in the browser's network panel), e.g.
//
//	Server-Timing: auth;dur=4.1, storage;dur=212.0, total;dur=219.3
//
// middleware.LoggingMiddleware attaches a Recorder to every request; code
// that wants a span calls Track. Without a Recorder in the context Track is a
// no-op, so handlers can call it unconditionally.
package timing

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

type contextKey struct{}

// Recorder collects the spans of one request. Safe for concurrent use: the
// bulk HEAD tracks storage calls from many goroutines.
type Recorder struct {
	mu    sync.Mutex
	start time.Time
	spans []*span
}

type span struct {
	name  string
	start time.Time
	dur   time.Duration
	open  bool
}

// WithRecorder returns ctx carrying a new Recorder that starts now.
func WithRecorder(ctx context.Context) (context.Context, *Recorder) {
	rec := &Recorder{start: time.Now()}
	return context.WithValue(ctx, contextKey{}, rec), rec
}

// Track starts a span named name and returns the func that ends it. Ending
// twice is harmless. Spans with the same name are summed in the header —
// several storage calls in one request read as one `storage` total.
func Track(ctx context.Context, name string) func() {
	rec, _ := ctx.Value(contextKey{}).(*Recorder)
	if rec == nil {
		return func() {}
	}
	s := &span{name: name, start: time.Now(), open: true}
	rec.mu.Lock()
	rec.spans = append(rec.spans, s)
	rec.mu.Unlock()
	return func() {
		rec.mu.Lock()
		if s.open {
			s.dur = time.Since(s.start)
			s.open = false
		}
		rec.mu.Unlock()
	}
}

// Elapsed is the time since the request started.
func (r *Recorder) Elapsed() time.Duration {
	return time.Since(r.start)
}

// Header renders the spans so far plus `total`. A span still open (an error
// response written mid-span) counts up to now.
func (r *Recorder) Header() string {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	var order []string
	sums := map[string]time.Duration{}
	for _, s := range r.spans {
		d := s.dur
		if s.open {
			d = now.Sub(s.start)
		}
		if _, seen := sums[s.name]; !seen {
			order = append(order, s.name)
		}
		sums[s.name] += d
	}

	parts := make([]string, 0, len(order)+1)
	for _, name := range order {
		parts = append(parts, metric(name, sums[name]))
	}
	parts = append(parts, metric("total", now.Sub(r.start)))
	return strings.Join(parts, ", ")
}

func metric(name string, d time.Duration) string {
	return fmt.Sprintf("%s;dur=%.1f", name, float64(d.Microseconds())/1000)
}
