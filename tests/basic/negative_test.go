//go:build e2e

package basic

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNeg1_MultilineSecretOverStdin(t *testing.T) {
	sb := newSandbox(t)
	store := sb.MakeStore("store.kdbx", map[string]string{"multi": "line1\nline2"})

	r := sb.Run("--key-store", store, "--stdin=multi", "--", "sh", "-c", "echo should-not-run")
	expectFail(t, r)
	if !strings.Contains(r.Stderr, "contains a newline") {
		t.Errorf("expected the newline error:\n%s", r.Stderr)
	}
	if strings.Contains(r.Stdout, "should-not-run") {
		t.Error("child ran despite the rejected secret")
	}
}

func TestNeg2_SecretFileWithoutPlaceholder(t *testing.T) {
	requireTool(t, "restic")
	sb := newSandbox(t)
	store := sb.MakeStore("store.kdbx", map[string]string{"restic-pw": "pw"})
	repo := filepath.Join(sb.Dir, "never-created")

	r := sb.Run("--key-store", store, "--secret-file=restic-pw", "--", "restic", "init", "-r", repo)
	expectFail(t, r)
	if !strings.Contains(r.Stderr, "{{restic-pw}}") {
		t.Errorf("expected the placeholder in stderr:\n%s", r.Stderr)
	}
	if _, err := os.Stat(repo); err == nil {
		t.Error("restic ran despite the missing placeholder")
	}
}
