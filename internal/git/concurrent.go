package git

import (
	"sync"
)

// concurrencyCap limits parallel git subprocesses.
const concurrencyCap = 8

// ConcurrentWork is one unit of work for FetchConcurrent.
type ConcurrentWork struct {
	Key string // arbitrary key for the result map (e.g. session name or "session:wi:pi")
	Dir string // directory to run git in
}

// FetchConcurrent runs git.Fetch for each work item in parallel, capped at
// concurrencyCap goroutines. Results that error are silently omitted from the
// returned map — callers should log if needed.
func FetchConcurrent(work []ConcurrentWork, timeoutMs int) map[string]Info {
	results := make(map[string]Info, len(work))
	if len(work) == 0 {
		return results
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrencyCap)
	for _, w := range work {
		wg.Add(1)
		w := w
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			info, err := Fetch(w.Dir, timeoutMs)
			if err == nil {
				mu.Lock()
				results[w.Key] = info
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return results
}
