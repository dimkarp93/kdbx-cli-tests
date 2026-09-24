//go:build e2e

package docker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dimkarp93/kdbx-cli-tests/internal/harness"
)

func TestEnvPG1_PasswordFromEnv(t *testing.T) {
	requireTool(t, "psql")
	sb := newSandbox(t)
	sb.storeWith(map[string]string{"pg-env": secret(t, "E2E_PG_ENV_PW")}, map[string]harness.Section{
		"psql": {Secrets: map[string]string{"pg-env": "PGPASSWORD"}},
	})

	r := sb.Run("--", "psql", "-h", "postgres", "-U", "app_env", "-d", "postgres", "-Atc", "select current_user")
	expectOK(t, r)
	if got := strings.TrimSpace(r.Stdout); got != "app_env" {
		t.Errorf("current_user: got %q, want app_env", got)
	}
}

func ghSections() map[string]harness.Section {
	return map[string]harness.Section{
		"github_install.sh": {Secrets: map[string]string{"GH_PAT": "GITHUB_TOKEN"}},
	}
}

func ghInstallArgs() []string {
	return []string{ghInstaller, "--user-only", "-s", mockGitHubURL, "acme/private-tool", "hello"}
}

func TestEnvGH1_InstallFromPrivateRepo(t *testing.T) {
	requireTool(t, "curl", "tar", "sha256sum")
	resetMockGitHub(t)
	token := secret(t, "E2E_GH_TOKEN")
	sb := newSandbox(t)
	sb.recordMockRequests()
	sb.storeWith(map[string]string{"GH_PAT": token}, ghSections())

	expectOK(t, sb.Run(append([]string{"--"}, ghInstallArgs()...)...))

	hello := filepath.Join(sb.Home, ".local", "bin", "hello")
	r := sb.Exec(sb.BaseEnv(), "", hello)
	expectOK(t, r)
	if got := strings.TrimSpace(r.Stdout); got != "hello-ok" {
		t.Errorf("hello: got %q, want hello-ok", got)
	}

	reqs := mockGitHubRequests(t)
	if len(reqs) == 0 {
		t.Fatal("mockgithub received no requests")
	}
	for _, req := range reqs {
		if req.Auth != "valid" {
			t.Errorf("request without a valid token: %+v", req)
		}
		if strings.Contains(req.Query, token) {
			t.Errorf("token leaked into the query string: %+v", req)
		}
	}
}

func TestEnvGH2_InstallWithoutKdbxCliFails(t *testing.T) {
	resetMockGitHub(t)
	sb := newSandbox(t)
	sb.recordMockRequests()

	r := sb.Exec(sb.BaseEnv(), "", ghInstallArgs()[0], ghInstallArgs()[1:]...)
	expectFail(t, r)
	if !strings.Contains(r.Stderr, "GitHub API returned HTTP 404") {
		t.Errorf("expected the installer's 404 error:\n%s", r.Stderr)
	}
	if _, err := os.Stat(filepath.Join(sb.Home, ".local", "bin", "hello")); err == nil {
		t.Error("hello was installed without a token")
	}
}

func TestEnvGH3_SectionChosenByBasename(t *testing.T) {
	sb := newSandbox(t)
	sb.storeWith(map[string]string{"GH_PAT": "x"}, ghSections())
	want := `"github_install.sh" (merged over "default")`

	byPath := sb.RunNoPassword("--dry-run", "--", ghInstaller, "--help")
	expectOK(t, byPath)
	byName := sb.RunEnv([]string{"PATH=/opt/install:" + os.Getenv("PATH")}, "", "--dry-run", "--", "github_install.sh", "--help")
	expectOK(t, byName)
	for label, r := range map[string]string{"full path": byPath.Stdout, "basename": byName.Stdout} {
		if !strings.Contains(r, want) {
			t.Errorf("%s: section %q not selected:\n%s", label, want, r)
		}
	}
}

func TestEnvGH4_DryRunTouchesNothing(t *testing.T) {
	resetMockGitHub(t)
	sb := newSandbox(t)
	sb.recordMockRequests()
	sb.storeWith(map[string]string{"GH_PAT": secret(t, "E2E_GH_TOKEN")}, ghSections())

	r := sb.RunNoPassword(append([]string{"--dry-run", "--"}, ghInstallArgs()...)...)
	expectOK(t, r)
	if !strings.Contains(r.Stdout, "GITHUB_TOKEN=<secret from GH_PAT>") {
		t.Errorf("dry-run plan is missing the env mapping:\n%s", r.Stdout)
	}
	if strings.Contains(r.Stdout+r.Stderr, secret(t, "E2E_GH_TOKEN")) {
		t.Error("dry-run printed the secret")
	}
	if reqs := mockGitHubRequests(t); len(reqs) != 0 {
		t.Errorf("dry-run reached the network: %+v", reqs)
	}
}

func TestEnvMerge1_DefaultAndToolSecretsReachTool(t *testing.T) {
	requireTool(t, "curl")
	token := secret(t, "E2E_GH_TOKEN")
	sb := newSandbox(t)
	sb.storeWith(map[string]string{"GH_PAT": token, "EXTRA_VAL": "extra-" + token[:8]}, map[string]harness.Section{
		"default": {Secrets: map[string]string{"GH_PAT": "GITHUB_TOKEN"}},
		"sh":      {Secrets: map[string]string{"EXTRA_VAL": "EXTRA"}},
	})

	r := sb.Run("--", "sh", "-c", `curl -sS -H "Authorization: token $GITHUB_TOKEN" -H "X-Extra: $EXTRA" `+mockGitHubURL+"/__echo")
	expectOK(t, r)
	for _, want := range []string{"Authorization: token " + token, "X-Extra: extra-" + token[:8]} {
		if !strings.Contains(r.Stdout, want) {
			t.Errorf("echoed headers missing %q:\n%s", want, r.Stdout)
		}
	}
}
