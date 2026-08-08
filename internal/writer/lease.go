package writer

import (
	"fmt"
	"sync"
	"time"
)

const DefaultLeaseTTL = 2 * time.Hour

type Lease struct {
	Holder  string
	Expires time.Time
	Branch  string
}

type WorkState struct {
	Status string
	Lease  Lease
}

// LeaseBook serializes one process's decisions. Remote serialization and fresh
// fingerprint checks are added by US-009; this type makes lease rules testable.
type LeaseBook struct {
	mu    sync.Mutex
	items map[string]WorkState
}

func NewLeaseBook() *LeaseBook { return &LeaseBook{items: make(map[string]WorkState)} }

func (b *LeaseBook) Claim(issueID, holder, branch string, now, expires time.Time) (WorkState, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if issueID == "" || holder == "" || branch == "" {
		return WorkState{}, fmt.Errorf("issue, holder, and branch are required")
	}
	if !expires.After(now) || expires.After(now.Add(DefaultLeaseTTL)) {
		return WorkState{}, fmt.Errorf("lease expiry must be after now and within %s", DefaultLeaseTTL)
	}
	state := b.items[issueID]
	if state.Status == "" {
		state.Status = "Ready"
	}
	if state.Lease.Holder != "" && state.Lease.Expires.After(now) {
		return WorkState{}, fmt.Errorf("issue already leased by %s until %s", state.Lease.Holder, state.Lease.Expires.UTC().Format(time.RFC3339))
	}
	if state.Lease.Holder != "" {
		if err := RequireTransition(state.Status, "Ready"); err != nil {
			return WorkState{}, err
		}
		state.Status = "Ready"
	}
	if err := RequireTransition(state.Status, "Claimed"); err != nil {
		return WorkState{}, err
	}
	state.Status = "Claimed"
	state.Lease = Lease{Holder: holder, Branch: branch, Expires: expires.UTC()}
	b.items[issueID] = state
	return state, nil
}

func (b *LeaseBook) Renew(issueID, holder string, now, expires time.Time) (WorkState, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	state := b.items[issueID]
	if state.Lease.Holder != holder || LeaseExpired(state.Lease.Expires.UTC().Format(time.RFC3339), now) {
		return WorkState{}, fmt.Errorf("active lease held by %q required", holder)
	}
	if !expires.After(now) || expires.After(now.Add(DefaultLeaseTTL)) {
		return WorkState{}, fmt.Errorf("lease expiry must be after now and within %s", DefaultLeaseTTL)
	}
	state.Lease.Expires = expires.UTC()
	b.items[issueID] = state
	return state, nil
}

func (b *LeaseBook) Release(issueID, holder string, now time.Time) (WorkState, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	state := b.items[issueID]
	if state.Lease.Holder != holder {
		return WorkState{}, fmt.Errorf("active lease held by %q required", holder)
	}
	if err := RequireTransition(state.Status, "Ready"); err != nil {
		return WorkState{}, err
	}
	state.Status = "Ready"
	state.Lease = Lease{Branch: state.Lease.Branch}
	b.items[issueID] = state
	return state, nil
}
