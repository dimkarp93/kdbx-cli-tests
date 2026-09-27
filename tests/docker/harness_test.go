//go:build e2e

package docker

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dimkarp93/kdbx-cli-tests/internal/harness"
)

var binaryPath string

var serviceAddrs = []string{
	"postgres:5432",
	"sshd:22",
}

func TestMain(m *testing.M) {
	if os.Getenv("E2E_IN_DOCKER") != "1" {
		fmt.Fprintln(os.Stderr, "docker e2e tests must run inside the runner container (make test)")
		os.Exit(2)
	}
	bin, _, err := harness.CheckBinary()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	binaryPath = bin
	if err := os.MkdirAll(os.Getenv("KDBX_CLI_E2E_ROOT"), 0755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	for _, addr := range serviceAddrs {
		if err := waitTCP(addr, 120*time.Second); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
	os.Exit(m.Run())
}

func waitTCP(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		c, err := net.DialTimeout("tcp", addr, 2*time.Second)
		if err == nil {
			c.Close()
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("service %s is not reachable: %w", addr, err)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

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

type sandbox struct {
	*harness.Sandbox
	runtimeDir string
}

func newSandbox(t *testing.T) *sandbox {
	t.Helper()
	sb := harness.NewSandbox(t, binaryPath)
	runtimeDir := filepath.Join(sb.Dir, "run")
	if err := os.MkdirAll(runtimeDir, 0700); err != nil {
		t.Fatal(err)
	}
	sb.Env = append(sb.Env, "XDG_RUNTIME_DIR="+runtimeDir)
	return &sandbox{Sandbox: sb, runtimeDir: runtimeDir}
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
