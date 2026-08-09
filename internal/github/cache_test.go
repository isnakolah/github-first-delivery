package github

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMetadataCacheStoresAtomically(t *testing.T) {
	cache := MetadataCache{Root: t.TempDir()}
	if err := cache.StoreJSON("issues/42", map[string]string{"source": "GitHub"}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(cache.Root, "issues", "42.json"))
	if err != nil || !strings.Contains(string(raw), "GitHub") {
		t.Fatalf("cache=%q err=%v", raw, err)
	}
	if err := cache.StoreJSON("../escape", map[string]string{}); err == nil {
		t.Fatal("expected traversal rejection")
	}
}
