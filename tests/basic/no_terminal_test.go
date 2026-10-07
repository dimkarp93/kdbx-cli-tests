//go:build e2e

package basic

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/dimkarp93/kdbx-cli-tests/internal/harness"
)

const detachedTimeout = 15 * time.Second

const echoStdinAndSecret = `read -r line; echo "got:$line:$MARK"`

type detachedRun struct {
	foreignTerminal bool
	stdin           string
	env             []string
	whileRunning    func() error
}

type detachedResult struct {
	harness.Result
	terminal string
}

func detachedEnv(sb *harness.Sandbox) []string {
	var env []string
	for _, kv := range sb.BaseEnv() {
		name, _, _ := strings.Cut(kv, "=")
		switch name {
		case "KDBX_CLI_PASSWORD", "DISPLAY", "WAYLAND_DISPLAY":
			continue
		}
		env = append(env, kv)
	}
	return env
}

func runDetached(sb *harness.Sandbox, run detachedRun, args ...string) detachedResult {
	sb.T.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), detachedTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmd.Dir = sb.Dir
	cmd.Env = append(detachedEnv(sb), run.env...)
	cmd.WaitDelay = 2 * time.Second
	cmd.Stdin = strings.NewReader(run.stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	terminal := make(chan string, 1)
	var slave *os.File
	if run.foreignTerminal {
		master, s, err := pty.Open()
		if err != nil {
			sb.T.Fatal(err)
		}
		defer master.Close()
		slave = s
		cmd.ExtraFiles = []*os.File{slave}
		cmd.SysProcAttr.Setctty = true
		cmd.SysProcAttr.Ctty = 3
		go func() {
			var buf bytes.Buffer
			_, _ = io.Copy(&buf, master)
			terminal <- buf.String()
		}()
	} else {
		terminal <- ""
	}

	err := cmd.Start()
	if slave != nil {
		slave.Close()
	}
	if err != nil {
		sb.T.Fatal(err)
	}
	answered := make(chan error, 1)
	if run.whileRunning != nil {
		go func() { answered <- run.whileRunning() }()
	} else {
		answered <- nil
	}
	err = cmd.Wait()
	if ctx.Err() != nil {
		sb.T.Fatalf("kdbx-cli hung for %s (waiting for a password on a terminal it does not own?)\nstdout:\n%s\nstderr:\n%s", detachedTimeout, stdout.String(), stderr.String())
	}
	if aerr := <-answered; aerr != nil {
		sb.T.Errorf("answering the password prompt: %v", aerr)
	}
	code := 0
	if err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			sb.T.Fatalf("run kdbx-cli %v: %v", args, err)
		}
		code = ee.ExitCode()
	}
	var term string
	select {
	case term = <-terminal:
	case <-time.After(2 * time.Second):
	}
	return detachedResult{Result: harness.Result{Stdout: stdout.String(), Stderr: stderr.String(), ExitCode: code}, terminal: term}
}

func promptStore(sb *harness.Sandbox, gui string) string {
	sb.T.Helper()
	store := markerStore(sb)
	cfg := harness.Config{
		Sections: map[string]harness.Section{"default": {KeyStore: store, Secrets: map[string]string{"MARK": "MARK"}}},
	}
	if gui != "" {
		cfg.Prompt = &harness.PromptConfig{GUI: gui}
	}
	sb.WriteFullConfig(cfg)
	return store
}

func pathWithOnly(t *testing.T, dir string, tools ...string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range tools {
		target, err := exec.LookPath(name)
		if err != nil {
			t.Fatalf("%s is missing from the runner image", name)
		}
		if err := os.Symlink(target, filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func expectNoStart(t *testing.T, r detachedResult) {
	t.Helper()
	expectFail(t, r.Result)
	if strings.Contains(r.Stdout, "got:") {
		t.Errorf("the command ran without a password\nstdout:\n%s", r.Stdout)
	}
}

func expectUntouchedTerminal(t *testing.T, r detachedResult) {
	t.Helper()
	if r.terminal != "" {
		t.Errorf("kdbx-cli wrote to a terminal it does not own:\n%q", r.terminal)
	}
}

func expectZenityRun(t *testing.T, foreignTerminal bool) {
	t.Helper()
	sb := newSandbox(t)
	display := xDisplay(t)
	promptStore(sb, "")

	r := runDetached(sb, detachedRun{
		foreignTerminal: foreignTerminal,
		stdin:           "ping\n",
		env:             []string{"DISPLAY=" + display},
		whileRunning:    func() error { return answerZenity(display, sb.Password, "Return") },
	}, "--", "sh", "-c", echoStdinAndSecret)
	expectOK(t, r.Result)
	if !strings.Contains(r.Stdout, "got:ping:marker-value") {
		t.Errorf("child did not get both its stdin and the secret\nstdout:\n%s\nstderr:\n%s", r.Stdout, r.Stderr)
	}
	expectUntouchedTerminal(t, r)
}

func TestNoTTY1_ForeignTerminalAsksThroughZenity(t *testing.T) {
	expectZenityRun(t, true)
}

func TestNoTTY2_DetachedAsksThroughZenity(t *testing.T) {
	expectZenityRun(t, false)
}

func TestNoTTY3_CancelledZenityAbortsCommand(t *testing.T) {
	sb := newSandbox(t)
	display := xDisplay(t)
	promptStore(sb, "")

	r := runDetached(sb, detachedRun{
		stdin:        "ping\n",
		env:          []string{"DISPLAY=" + display},
		whileRunning: func() error { return answerZenity(display, "", "Escape") },
	}, "--", "sh", "-c", echoStdinAndSecret)
	expectNoStart(t, r)
	if !strings.Contains(r.Stderr, "cancelled") {
		t.Errorf("stderr does not report the cancellation:\n%s", r.Stderr)
	}
}

func TestNoTTY4_PromptOffFailsFastAndLeavesStdinAlone(t *testing.T) {
	sb := newSandbox(t)
	display := xDisplay(t)
	promptStore(sb, "off")

	r := runDetached(sb, detachedRun{
		foreignTerminal: true,
		stdin:           sb.Password + "\nping\n",
		env:             []string{"DISPLAY=" + display},
	}, "--", "sh", "-c", echoStdinAndSecret)
	expectNoStart(t, r)
	for _, want := range []string{"no terminal", "kdbx-cli unlock"} {
		if !strings.Contains(r.Stderr, want) {
			t.Errorf("stderr lacks %q:\n%s", want, r.Stderr)
		}
	}
	expectUntouchedTerminal(t, r)
}

func TestNoTTY5_NoDisplayFailsFast(t *testing.T) {
	sb := newSandbox(t)
	promptStore(sb, "")

	r := runDetached(sb, detachedRun{stdin: "ping\n"}, "--", "sh", "-c", echoStdinAndSecret)
	expectNoStart(t, r)
	for _, want := range []string{"no terminal", "zenity"} {
		if !strings.Contains(r.Stderr, want) {
			t.Errorf("stderr lacks %q:\n%s", want, r.Stderr)
		}
	}
}

func TestNoTTY6_NoZenityFailsFast(t *testing.T) {
	sb := newSandbox(t)
	display := xDisplay(t)
	promptStore(sb, "")
	path := pathWithOnly(t, filepath.Join(sb.Dir, "bin"), "sh", "keepassxc-cli")

	r := runDetached(sb, detachedRun{
		stdin: "ping\n",
		env:   []string{"DISPLAY=" + display, "PATH=" + path},
	}, "--", "sh", "-c", echoStdinAndSecret)
	expectNoStart(t, r)
	if !strings.Contains(r.Stderr, "install zenity") {
		t.Errorf("stderr does not suggest installing zenity:\n%s", r.Stderr)
	}
}

func TestNoTTY7_UnlockRequiresCache(t *testing.T) {
	sb := newSandbox(t)
	promptStore(sb, "off")

	r := sb.RunNoPassword("unlock")
	expectFail(t, r)
	if !strings.Contains(r.Stderr, "caching is disabled") {
		t.Errorf("stderr does not explain that caching is disabled:\n%s", r.Stderr)
	}
}
