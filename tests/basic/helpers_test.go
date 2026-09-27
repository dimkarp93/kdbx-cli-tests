//go:build e2e

package basic

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/dimkarp93/kdbx-cli-tests/internal/harness"
)

func secret(t *testing.T, name string) string {
	t.Helper()
	v := os.Getenv(name)
	if v == "" {
		t.Fatalf("%s is not set", name)
	}
	return v
}

func requireTool(t *testing.T, names ...string) {
	t.Helper()
	for _, name := range names {
		if _, err := exec.LookPath(name); err != nil {
			t.Fatalf("%s is missing from the runner image", name)
		}
	}
}

func expectOK(t *testing.T, r harness.Result) {
	t.Helper()
	if r.ExitCode != 0 {
		t.Fatalf("exit %d\nstdout:\n%s\nstderr:\n%s", r.ExitCode, r.Stdout, r.Stderr)
	}
}

func expectFail(t *testing.T, r harness.Result) {
	t.Helper()
	if r.ExitCode == 0 {
		t.Fatalf("expected a non-zero exit\nstdout:\n%s\nstderr:\n%s", r.Stdout, r.Stderr)
	}
}

func storeWith(sb *harness.Sandbox, entries map[string]string, sections map[string]harness.Section) string {
	sb.T.Helper()
	store := sb.MakeStore("store.kdbx", entries)
	if sections == nil {
		sections = map[string]harness.Section{}
	}
	def := sections["default"]
	def.KeyStore = store
	sections["default"] = def
	sb.WriteConfig(sections)
	return store
}

func resticRepo(sb *harness.Sandbox, password string) string {
	sb.T.Helper()
	repo := filepath.Join(sb.Dir, "restic-repo")
	env := append(sb.BaseEnv(), "RESTIC_PASSWORD="+password, "RESTIC_REPOSITORY="+repo)
	expectOK(sb.T, sb.Exec(env, "", "restic", "init"))
	return repo
}

type runtimeSandbox struct {
	*harness.Sandbox
	runtimeDir string
}

func newRuntimeSandbox(t *testing.T) *runtimeSandbox {
	t.Helper()
	sb := harness.NewSandbox(t, binaryPath)
	runtimeDir := filepath.Join(sb.Dir, "run")
	if err := os.MkdirAll(runtimeDir, 0700); err != nil {
		t.Fatal(err)
	}
	sb.Env = append(sb.Env, "XDG_RUNTIME_DIR="+runtimeDir)
	return &runtimeSandbox{Sandbox: sb, runtimeDir: runtimeDir}
}
