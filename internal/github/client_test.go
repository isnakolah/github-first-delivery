package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestAuthTokenPrefersEnvironment(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", " test-token ")
	if got := authToken(); got != "test-token" {
		t.Fatalf("authToken() = %q, want environment token", got)
	}
}

func TestSetIssueStatePatchesCanonicalEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/repos/o/r/issues/7" {
			t.Fatalf("request %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	client := &Client{BaseURL: server.URL, HTTP: server.Client()}
	if err := client.SetIssueState(context.Background(), "o", "r", 7, "closed"); err != nil {
		t.Fatal(err)
	}
	if err := client.SetIssueState(context.Background(), "o", "r", 7, "invalid"); err == nil {
		t.Fatal("expected invalid state rejection")
	}
}

func TestNewClientDoesNotPersistToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	client := NewClient()
	if client.Token != "test-token" {
		t.Fatalf("client token = %q", client.Token)
	}
	if _, err := os.Stat(".gfd-token"); !os.IsNotExist(err) {
		t.Fatalf("token cache was created: %v", err)
	}
}
