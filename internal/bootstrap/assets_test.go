package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallWritesCompleteDynamicContract(t *testing.T) {
	root := t.TempDir()
	if err := Install(root, "octo", 7); err != nil {
		t.Fatal(err)
	}
	for path := range files {
		if _, err := os.Stat(filepath.Join(root, path)); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
	config, err := os.ReadFile(filepath.Join(root, ".github/ISSUE_TEMPLATE/config.yml"))
	if err != nil || !strings.Contains(string(config), "https://github.com/users/octo/projects/7") {
		t.Fatalf("config=%q err=%v", config, err)
	}
	if err := Install(root, "octo", 7); err == nil {
		t.Fatal("expected overwrite refusal")
	}
}
