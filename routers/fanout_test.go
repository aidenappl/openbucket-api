package routers

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestForEachLimited(t *testing.T) {
	var running, peak, done atomic.Int32
	forEachLimited(100, func(i int) {
		n := running.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		time.Sleep(2 * time.Millisecond)
		running.Add(-1)
		done.Add(1)
	})
	if done.Load() != 100 {
		t.Fatalf("ran %d of 100", done.Load())
	}
	if peak.Load() > bulkConcurrency {
		t.Fatalf("peak concurrency %d exceeds %d", peak.Load(), bulkConcurrency)
	}
}
