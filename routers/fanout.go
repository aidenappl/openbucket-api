package routers

import "sync"

// bulkConcurrency caps the storage calls one bulk request (HEAD, ACL) runs at
// once. It used to start all 100 at once, and the console sends 4 bulk
// requests in parallel: ~400 simultaneous calls deadlocked openbucket-go's
// connection pool (cdn.trailblaze.to, 2026-10-15). aws.storageHTTPClient also
// caps connections per storage server.
const bulkConcurrency = 16

// forEachLimited runs fn(i) for i in [0, n), at most bulkConcurrency at a
// time, and returns when all are done.
func forEachLimited(n int, fn func(i int)) {
	sem := make(chan struct{}, bulkConcurrency)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer func() { <-sem; wg.Done() }()
			fn(i)
		}(i)
	}
	wg.Wait()
}
