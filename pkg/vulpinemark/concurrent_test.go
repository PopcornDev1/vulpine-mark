package vulpinemark

import (
	"sync"
	"testing"
)

// TestMarkLastResultConcurrent hammers LastResult from one goroutine
// while another goroutine stores into lastResult under the mutex. Run
// with -race to verify there's no data race on the cached result.
// We simulate the Annotate write path directly instead of running a
// real CDP enumeration.
func TestMarkLastResultConcurrent(t *testing.T) {
	m := &Mark{}

	const iterations = 2000
	var wg sync.WaitGroup
	wg.Add(2)

	// Writer: mutates lastResult the same way annotate() does.
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			res := &Result{
				Elements: map[string]Element{
					"@1": {Label: "@1", X: float64(i), Y: 0, Width: 10, Height: 10},
				},
				Labels: []string{"@1"},
			}
			m.mu.Lock()
			m.lastResult = res
			m.mu.Unlock()
		}
	}()

	// Reader: reads via the public API.
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			_ = m.LastResult()
		}
	}()

	wg.Wait()

	last := m.LastResult()
	if last == nil {
		t.Fatal("LastResult nil after concurrent writes")
	}
	if _, ok := last.Elements["@1"]; !ok {
		t.Errorf("expected @1 in final result: %+v", last.Elements)
	}
}
