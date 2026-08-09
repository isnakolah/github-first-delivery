package github

import (
	"os"
	"testing"
)

func TestAuthTokenPrefersEnvironment(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", " test-token ")
	if got := authToken(); got != "test-token" {
		t.Fatalf("authToken() = %q, want environment token", got)
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
