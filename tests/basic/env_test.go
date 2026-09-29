//go:build e2e

package basic

import (
	"os"
	"strings"
	"testing"

	"github.com/dimkarp93/kdbx-cli-tests/internal/harness"
)

const ghInstaller = "/opt/install/github_install.sh"

func TestEnvGH3_SectionChosenByBasename(t *testing.T) {
	sb := newSandbox(t)
	storeWith(sb, map[string]string{"GH_PAT": "x"}, map[string]harness.Section{
		"github_install.sh": {Secrets: map[string]string{"GITHUB_TOKEN": "GH_PAT"}},
	})
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
