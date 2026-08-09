package writer

import (
	"strings"
	"testing"
	"time"
)

func TestApplyLifecycleClaimStartRenewRelease(t *testing.T) {
	now := time.Date(2026, 8, 9, 3, 0, 0, 0, time.UTC)
	request := Request{Action: "claim", Actor: "codex/isnakolah/session", Branch: "011/writer", LeaseExpiresAt: now.Add(time.Hour).Format(time.RFC3339)}
	state, err := ApplyLifecycle(WorkState{Status: "Ready"}, request, now)
	if err != nil || state.Status != "Claimed" || state.Lease.Holder != request.Actor {
		t.Fatalf("claim state=%+v err=%v", state, err)
	}
	request.Action = "start"
	state, err = ApplyLifecycle(state, request, now.Add(time.Minute))
	if err != nil || state.Status != "In progress" {
		t.Fatalf("start state=%+v err=%v", state, err)
	}
	request.Action = "renew"
	request.LeaseExpiresAt = now.Add(90 * time.Minute).Format(time.RFC3339)
	state, err = ApplyLifecycle(state, request, now.Add(2*time.Minute))
	if err != nil || !state.Lease.Expires.Equal(now.Add(90*time.Minute)) {
		t.Fatalf("renew state=%+v err=%v", state, err)
	}
	request.Action = "release"
	state, err = ApplyLifecycle(state, request, now.Add(3*time.Minute))
	if err != nil || state.Status != "Ready" || state.Lease.Holder != "" {
		t.Fatalf("release state=%+v err=%v", state, err)
	}
}

func TestApplyLifecycleRejectsInvalidOrContendedState(t *testing.T) {
	now := time.Date(2026, 8, 9, 3, 0, 0, 0, time.UTC)
	_, err := ApplyLifecycle(WorkState{Status: "Backlog"}, Request{Action: "claim", Actor: "a", Branch: "001/a"}, now)
	if err == nil || !strings.Contains(err.Error(), "Backlog -> Claimed") {
		t.Fatalf("err=%v", err)
	}
	_, err = ApplyLifecycle(WorkState{Status: "Ready", Lease: Lease{Holder: "other", Expires: now.Add(time.Hour)}}, Request{Action: "claim", Actor: "a", Branch: "001/a"}, now)
	if err == nil || !strings.Contains(err.Error(), "already leased") {
		t.Fatalf("err=%v", err)
	}
}

func TestApplyLifecycleExpiresLeaseBeforeRenew(t *testing.T) {
	now := time.Date(2026, 8, 9, 3, 0, 0, 0, time.UTC)
	_, err := ApplyLifecycle(WorkState{Status: "Claimed", Lease: Lease{Holder: "a", Expires: now}}, Request{Action: "renew", Actor: "a"}, now)
	if err == nil || !strings.Contains(err.Error(), "active lease") {
		t.Fatalf("err=%v", err)
	}
}

func TestApplyLifecycleInitialBacklogStatus(t *testing.T) {
	state, err := ApplyLifecycle(WorkState{}, Request{Action: "status", Status: "Backlog"}, time.Now())
	if err != nil || state.Status != "Backlog" {
		t.Fatalf("state=%+v err=%v", state, err)
	}
	if _, err := ApplyLifecycle(WorkState{Status: "Ready"}, Request{Action: "status", Status: "Backlog"}, time.Now()); err == nil {
		t.Fatal("expected backwards status rejection")
	}
}

func TestReclaimExpiredPreservesBranch(t *testing.T) {
	now := time.Date(2026, 8, 9, 3, 0, 0, 0, time.UTC)
	next, reclaimed := ReclaimExpired(WorkState{Status: "In progress", Lease: Lease{Holder: "a", Expires: now, Branch: "011/work"}}, now)
	if !reclaimed || next.Status != "Ready" || next.Lease.Holder != "" || next.Lease.Branch != "011/work" {
		t.Fatalf("next=%+v reclaimed=%t", next, reclaimed)
	}
}

func TestApplyLifecycleEvidenceRequiresCompletePayload(t *testing.T) {
	now := time.Date(2026, 8, 9, 3, 0, 0, 0, time.UTC)
	evidence := &Evidence{FinalSHA: "sha", CIURL: "ci", Commands: "go test", Environments: "CI", Criteria: "pass", Artifacts: "none", Documentation: "none", Risks: "none", Boundary: "CI"}
	state, err := ApplyLifecycle(WorkState{Status: "In review"}, Request{Action: "evidence.submit", PR: "https://example.test/pr/1", Evidence: evidence}, now)
	if err != nil || state.Status != "Evidence pending" {
		t.Fatalf("state=%+v err=%v", state, err)
	}
	_, err = ApplyLifecycle(WorkState{Status: "In review"}, Request{Action: "evidence.submit", Evidence: evidence}, now)
	if err == nil || !strings.Contains(err.Error(), "pull request") {
		t.Fatalf("err=%v", err)
	}
}

func TestApplyLifecyclePRLinkMovesReview(t *testing.T) {
	state, err := ApplyLifecycle(WorkState{Status: "In progress"}, Request{Action: "pr.link", PR: "https://example.test/pr/1"}, time.Now())
	if err != nil || state.Status != "In review" {
		t.Fatalf("state=%+v err=%v", state, err)
	}
}
