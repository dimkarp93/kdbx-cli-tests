//go:build e2e

package docker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAskGit1_CloneWithUsernameInURL(t *testing.T) {
	requireTool(t, "git")
	sb := newSandbox(t)
	store := sb.MakeStore("store.kdbx", map[string]string{"gitea-token": secret(t, "E2E_GITEA_TOKEN")})
	dest := filepath.Join(sb.Dir, "clone")

	r := sb.Run("--key-store", store, "--askpass=gitea-token",
		"--", "git", "clone", "http://alice@gitea:3000/alice/private-tool.git", dest)
	expectOK(t, r)
	if _, err := os.Stat(filepath.Join(dest, ".git")); err != nil {
		t.Fatalf("clone is missing: %v", err)
	}
	if strings.Contains(r.Stdout+r.Stderr, secret(t, "E2E_GITEA_TOKEN")) {
		t.Error("git printed the token")
	}
}

func TestAskGit2_CloneWithoutUsernameSendsTokenTwice(t *testing.T) {
	requireTool(t, "git")
	sb := newSandbox(t)
	store := sb.MakeStore("store.kdbx", map[string]string{"gitea-token": secret(t, "E2E_GITEA_TOKEN")})
	dest := filepath.Join(sb.Dir, "clone")

	start := time.Now()
	r := sb.Run("--key-store", store, "--askpass=gitea-token",
		"--", "git", "clone", "http://gitea:3000/alice/private-tool.git", dest)
	if elapsed := time.Since(start); elapsed > 15*time.Second {
		t.Errorf("git took %s: GIT_TERMINAL_PROMPT=0 should keep it from waiting", elapsed)
	}
	expectOK(t, r)
	if _, err := os.Stat(filepath.Join(dest, ".git")); err != nil {
		t.Fatalf("Gitea accepts the token as the username, so the clone should succeed: %v", err)
	}
}
