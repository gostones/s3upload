package internal

import (
	"sync"
	"testing"
)

func TestCounter(t *testing.T) {
	var c Counter
	var wg sync.WaitGroup

	n := 10
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			t.Log(c.Increment(1), c.Get())
			wg.Done()
		}()
	}
	wg.Wait()

	t.Log(c.Get())
}
