//go:build e2e

package docker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dimkarp93/kdbx-cli-tests/internal/harness"
)

const (
	giteaURL       = "http://gitea:3000"
	giteaInstaller = "/opt/install/gitea_install.sh"
)

func giteaInstallArgs() []string {
	return []string{giteaInstaller, "--user-only", "-s", giteaURL, "alice/private-tool", "hello"}
}

func TestEnvGitea1_InstallFromPrivateRepo(t *testing.T) {
	requireTool(t, "curl", "tar", "sha256sum")
	sb := newSandbox(t)
	sb.storeWith(map[string]string{"gitea-token": secret(t, "E2E_GITEA_TOKEN")}, map[string]harness.Section{
		"gitea_install.sh": {Secrets: map[string]string{"gitea-token": "GITEA_TOKEN"}},
	})
	hello := filepath.Join(sb.Home, ".local", "bin", "hello")

	control := sb.Exec(sb.BaseEnv(), "", giteaInstallArgs()[0], giteaInstallArgs()[1:]...)
	expectFail(t, control)
	if _, err := os.Stat(hello); err == nil {
		t.Fatal("hello was installed without a token")
	}

	expectOK(t, sb.Run(append([]string{"--"}, giteaInstallArgs()...)...))
	r := sb.Exec(sb.BaseEnv(), "", hello)
	expectOK(t, r)
	if got := strings.TrimSpace(r.Stdout); got != "hello-ok" {
		t.Errorf("hello: got %q, want hello-ok", got)
	}
}
