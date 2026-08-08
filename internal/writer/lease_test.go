package writer

import (
	"sync"
	"testing"
	"time"
)

func TestConcurrentClaimYieldsOneLease(t *testing.T) {
	b := NewLeaseBook()
	now := time.Date(2026, 8, 8, 20, 0, 0, 0, time.UTC)
	expires := now.Add(time.Hour)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, holder := range []string{"a", "b"} {
		wg.Add(1)
		go func(holder string) {
			defer wg.Done()
			_, err := b.Claim("I1", holder, "001/work", now, expires)
			results <- err
		}(holder)
	}
	wg.Wait()
	close(results)
	ok := 0
	for err := range results {
		if err == nil {
			ok++
		}
	}
	if ok != 1 {
		t.Fatalf("claims accepted=%d, want 1", ok)
	}
}

func TestExpiredLeaseCanBeReclaimed(t *testing.T) {
	b := NewLeaseBook()
	now := time.Date(2026, 8, 8, 20, 0, 0, 0, time.UTC)
	if _, err := b.Claim("I1", "a", "001/work", now, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	state, err := b.Claim("I1", "b", "002/takeover", now.Add(2*time.Hour), now.Add(3*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if state.Lease.Holder != "b" || state.Status != "Claimed" {
		t.Fatalf("got %+v", state)
	}
}
