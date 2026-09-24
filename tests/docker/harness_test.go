//go:build e2e

package docker

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dimkarp93/kdbx-cli-tests/internal/harness"
)

var binaryPath string

var serviceAddrs = []string{
	"postgres:5432",
	"mockgithub:8080",
	"gitea:3000",
	"registry:5000",
	"mariadb:3306",
	"sshd:22",
}

const seedDir = "/seed"

var seedEnv = map[string]string{}

func TestMain(m *testing.M) {
	if os.Getenv("E2E_IN_DOCKER") != "1" {
		fmt.Fprintln(os.Stderr, "docker e2e tests must run inside the runner container (make test)")
		os.Exit(2)
	}
	bin, version, err := harness.CheckBinary()
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
	if err := loadSeed(120 * time.Second); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	harness.WriteSummaryHeader("tests/docker", version)
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

func loadSeed(timeout time.Duration) error {
	ready := filepath.Join(seedDir, "ready")
	deadline := time.Now().Add(timeout)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%s did not appear: seed has not run", ready)
		}
		time.Sleep(500 * time.Millisecond)
	}
	files, _ := filepath.Glob(filepath.Join(seedDir, "*.env"))
	for _, path := range files {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			if k, v, ok := strings.Cut(strings.TrimSpace(sc.Text()), "="); ok {
				seedEnv[k] = v
			}
		}
		f.Close()
	}
	return nil
}

func secret(t *testing.T, name string) string {
	t.Helper()
	if v, ok := seedEnv[name]; ok && v != "" {
		return v
	}
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
	for _, kv := range os.Environ() {
		if k, v, ok := strings.Cut(kv, "="); ok && strings.HasPrefix(k, "E2E_") {
			sb.Transcript.AddSecret(v, k)
		}
	}
	for k, v := range seedEnv {
		sb.Transcript.AddSecret(v, k)
	}
	return &sandbox{Sandbox: sb, runtimeDir: runtimeDir}
}

func (s *sandbox) storeWith(entries map[string]string, sections map[string]harness.Section) string {
	s.T.Helper()
	store := s.MakeStore("store.kdbx", entries)
	def := sections["default"]
	def.KeyStore = store
	if sections == nil {
		sections = map[string]harness.Section{}
	}
	sections["default"] = def
	s.WriteConfig(sections)
	return store
}

func (s *sandbox) sh(script string, args ...string) harness.Result {
	s.T.Helper()
	return s.Exec(s.BaseEnv(), "", "sh", append([]string{"-c", script, "sh"}, args...)...)
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

func requireNoControllingTTY(t *testing.T) {
	t.Helper()
	if f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0); err == nil {
		f.Close()
		t.Fatal("the test process has a controlling terminal; run the runner with -T")
	}
}
