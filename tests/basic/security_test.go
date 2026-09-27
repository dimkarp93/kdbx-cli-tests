//go:build e2e

package basic

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func randomSecret(t *testing.T) string {
	t.Helper()
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return "sec-" + hex.EncodeToString(b)
}

type channel struct {
	name string
	args func(sb *runtimeSandbox, title string) []string
}

const waitForRelease = `touch "$E2E_READY"; while [ ! -e "$E2E_RELEASE" ]; do sleep 0.05; done`

var channels = []channel{
	{"env", func(sb *runtimeSandbox, title string) []string {
		return []string{"--secrets=" + title + ":SEC_VALUE", "--", "sh", "-c", `[ -n "$SEC_VALUE" ] || exit 9; ` + waitForRelease}
	}},
	{"stdin", func(sb *runtimeSandbox, title string) []string {
		return []string{"--stdin=" + title, "--", "sh", "-c", `read -r v; [ -n "$v" ] || exit 9; ` + waitForRelease}
	}},
	{"files", func(sb *runtimeSandbox, title string) []string {
		return []string{"--secret-file=" + title, "--", "sh", "-c", `[ -s "$1" ] || exit 9; ` + waitForRelease, "sh", "{{" + title + "}}"}
	}},
	{"templates", func(sb *runtimeSandbox, title string) []string {
		tmplPath := filepath.Join(sb.Dir, "tmpl.txt")
		if err := os.WriteFile(tmplPath, []byte("{{"+title+"}}"), 0600); err != nil {
			sb.T.Fatal(err)
		}
		return []string{"--template=tpl:" + tmplPath, "--", "sh", "-c", `[ -s "$1" ] || exit 9; ` + waitForRelease, "sh", "{{tpl}}"}
	}},
	{"askpass", func(sb *runtimeSandbox, title string) []string {
		return []string{"--askpass=" + title, "--", "sh", "-c", `v=$("$SSH_ASKPASS" Password:); [ -n "$v" ] || exit 9; ` + waitForRelease}
	}},
}

func procCmdlinesContaining(needle string) []string {
	var hits []string
	dirs, _ := filepath.Glob("/proc/[0-9]*")
	for _, dir := range dirs {
		data, err := os.ReadFile(filepath.Join(dir, "cmdline"))
		if err != nil {
			continue
		}
		if bytes.Contains(data, []byte(needle)) {
			hits = append(hits, dir+": "+strings.ReplaceAll(string(data), "\x00", " "))
		}
	}
	return hits
}

func filesContaining(needle string, roots ...string) []string {
	var hits []string
	for _, root := range roots {
		filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if path == "/tmp/gocache" || strings.HasPrefix(d.Name(), "go-build") {
					return filepath.SkipDir
				}
				return nil
			}
			if !d.Type().IsRegular() {
				return nil
			}
			if info, err := d.Info(); err != nil || info.Size() > 16<<20 {
				return nil
			}
			data, err := os.ReadFile(path)
			if err == nil && bytes.Contains(data, []byte(needle)) {
				hits = append(hits, path)
			}
			return nil
		})
	}
	return hits
}

func TestSecArgvAndDisk_AllChannels(t *testing.T) {
	for _, ch := range channels {
		t.Run(ch.name, func(t *testing.T) {
			sb := newRuntimeSandbox(t)
			value := randomSecret(t)
			store := sb.MakeStore("store.kdbx", map[string]string{"sec": value})
			ready := filepath.Join(sb.Dir, "ready")
			release := filepath.Join(sb.Dir, "release")

			cmd := exec.Command(binaryPath, append([]string{"--key-store", store}, ch.args(sb, "sec")...)...)
			cmd.Dir = sb.Dir
			cmd.Env = append(sb.BaseEnv(), "KDBX_CLI_PASSWORD="+sb.Password, "E2E_READY="+ready, "E2E_RELEASE="+release)
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			t.Cleanup(func() {
				os.WriteFile(release, nil, 0600)
				cmd.Process.Kill()
			})

			deadline := time.Now().Add(30 * time.Second)
			for {
				if _, err := os.Stat(ready); err == nil {
					break
				}
				select {
				case err := <-done:
					t.Fatalf("child exited before it was ready: %v\n%s", err, stderr.String())
				default:
				}
				if time.Now().After(deadline) {
					t.Fatalf("child never became ready\n%s", stderr.String())
				}
				time.Sleep(20 * time.Millisecond)
			}

			if hits := procCmdlinesContaining(value); len(hits) > 0 {
				t.Errorf("secret found in argv:\n%s", strings.Join(hits, "\n"))
			}
			if hits := filesContaining(value, sb.Home, "/tmp", "/var/tmp", sb.runtimeDir); len(hits) > 0 {
				t.Errorf("secret found on disk while running: %v", hits)
			}

			os.WriteFile(release, nil, 0600)
			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("kdbx-cli failed: %v\n%s", err, stderr.String())
				}
			case <-time.After(30 * time.Second):
				t.Fatal("child did not exit after release")
			}

			if hits := filesContaining(value, sb.Home, "/tmp", "/var/tmp", sb.runtimeDir); len(hits) > 0 {
				t.Errorf("secret left on disk after exit: %v", hits)
			}
		})
	}
}

func TestSecAskpass1_PrivateDirRemoved(t *testing.T) {
	for _, exit := range []int{0, 3} {
		t.Run(map[int]string{0: "success", 3: "failure"}[exit], func(t *testing.T) {
			sb := newRuntimeSandbox(t)
			store := sb.MakeStore("store.kdbx", map[string]string{"pw": randomSecret(t)})

			script := `for d in "$XDG_RUNTIME_DIR"/kdbx-cli-*; do stat -c %a "$d"; done; exit ` + map[int]string{0: "0", 3: "3"}[exit]
			r := sb.Run("--key-store", store, "--askpass=pw", "--", "sh", "-c", script)
			if r.ExitCode != exit {
				t.Fatalf("exit code: got %d, want %d\n%s", r.ExitCode, exit, r.Stderr)
			}
			if got := strings.TrimSpace(r.Stdout); got != "700" {
				t.Errorf("askpass dir mode while running: got %q, want 700", got)
			}
			left, _ := filepath.Glob(filepath.Join(sb.runtimeDir, "kdbx-cli-*"))
			if len(left) > 0 {
				t.Errorf("askpass dir left behind: %v", left)
			}
		})
	}
}

func TestSecEnv1_SecretOnlyInChildEnviron(t *testing.T) {
	sb := newSandbox(t)
	value := randomSecret(t)
	store := sb.MakeStore("store.kdbx", map[string]string{"sec": value})

	script := `has() { tr '\0' '\n' < "/proc/$1/environ" | grep -qxF "SEC_VALUE=$SEC_VALUE" && echo yes || echo no; }; echo "parent=$(has $PPID) child=$(has $$)"`
	r := sb.Run("--key-store", store, "--secrets=sec:SEC_VALUE", "--", "sh", "-c", script)
	expectOK(t, r)
	if got := strings.TrimSpace(r.Stdout); got != "parent=no child=yes" {
		t.Errorf("environ check: got %q, want parent=no child=yes", got)
	}
}

func TestSecScannersDetectPlantedLeaks(t *testing.T) {
	sb := newRuntimeSandbox(t)
	value := randomSecret(t)

	planted := filepath.Join(sb.runtimeDir, "leak")
	if err := os.WriteFile(planted, []byte("x"+value+"x"), 0600); err != nil {
		t.Fatal(err)
	}
	if hits := filesContaining(value, sb.Home, sb.runtimeDir); len(hits) != 1 || hits[0] != planted {
		t.Errorf("disk scanner missed the planted file: %v", hits)
	}

	cmd := exec.Command("sh", "-c", "sleep 30", "sh", value)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { cmd.Process.Kill(); cmd.Wait() }()
	if hits := procCmdlinesContaining(value); len(hits) == 0 {
		t.Error("argv scanner missed the planted process")
	}
}
