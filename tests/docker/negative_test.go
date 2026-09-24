//go:build e2e

package docker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dimkarp93/kdbx-cli-tests/internal/harness"
)

func TestNeg5_InstallerWithWrongToken(t *testing.T) {
	resetMockGitHub(t)
	sb := newSandbox(t)
	sb.recordMockRequests()
	sb.storeWith(map[string]string{"GH_PAT": "ghp_wrong_token"}, ghSections())

	r := sb.Run(append([]string{"--"}, ghInstallArgs()...)...)
	expectFail(t, r)
	if !strings.Contains(r.Stderr, "GitHub API returned HTTP 404") {
		t.Errorf("expected the installer's 404 error:\n%s", r.Stderr)
	}
	reqs := mockGitHubRequests(t)
	if len(reqs) == 0 {
		t.Fatal("mockgithub received no requests")
	}
	for _, req := range reqs {
		if req.Auth != "invalid" {
			t.Errorf("expected only requests with the wrong token, got %+v", req)
		}
	}
}

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

func TestNeg3_MissingEntry(t *testing.T) {
	requireTool(t, "psql")
	sb := newSandbox(t)
	sb.storeWith(map[string]string{"present": "x"}, map[string]harness.Section{
		"psql": {Secrets: map[string]string{"absent": "PGPASSWORD"}},
	})

	r := sb.Run("--", "psql", "-h", "postgres", "-U", "app_env", "-d", "postgres", "-Atc", "select 1")
	if r.ExitCode != 1 {
		t.Errorf("exit code: got %d, want 1\n%s", r.ExitCode, r.Stderr)
	}
	if !strings.Contains(r.Stderr, "cannot resolve secrets") {
		t.Errorf("expected 'cannot resolve secrets':\n%s", r.Stderr)
	}
}

func TestNeg4_ToolExitCodePropagated(t *testing.T) {
	requireTool(t, "psql")
	sb := newSandbox(t)
	sb.storeWith(map[string]string{"pg-env": "wrong-password"}, map[string]harness.Section{
		"psql": {Secrets: map[string]string{"pg-env": "PGPASSWORD"}},
	})

	r := sb.Run("--", "psql", "-h", "postgres", "-U", "app_env", "-d", "postgres", "-Atc", "select 1")
	if r.ExitCode != 2 {
		t.Errorf("exit code: got %d, want psql's 2\n%s", r.ExitCode, r.Stderr)
	}
	if !strings.Contains(r.Stderr, "password authentication failed") {
		t.Errorf("expected the psql auth error:\n%s", r.Stderr)
	}
}
