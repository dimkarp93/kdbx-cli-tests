//go:build e2e

package basic

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/dimkarp93/kdbx-cli-tests/internal/harness"
	"golang.org/x/sys/unix"
)

type ptySession struct {
	t          *testing.T
	cmd        *exec.Cmd
	tty        *os.File
	mu         sync.Mutex
	buf        bytes.Buffer
	exited     chan struct{}
	readerDone chan struct{}
	exitCode   int
}

func startPTY(sb *harness.Sandbox, env []string, name string, args ...string) *ptySession {
	sb.T.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = sb.Dir
	cmd.Env = env
	const rows, cols = 40, 200
	tty, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: rows, Cols: cols})
	if err != nil {
		sb.T.Fatal(err)
	}
	p := &ptySession{t: sb.T, cmd: cmd, tty: tty, exited: make(chan struct{}), readerDone: make(chan struct{})}
	go func() {
		defer close(p.readerDone)
		chunk := make([]byte, 4096)
		for {
			n, err := tty.Read(chunk)
			if n > 0 {
				p.mu.Lock()
				p.buf.Write(chunk[:n])
				p.mu.Unlock()
				answerTerminalQueries(tty, chunk[:n])
			}
			if err != nil {
				return
			}
		}
	}()
	go func() {
		err := cmd.Wait()
		var ee *exec.ExitError
		switch {
		case err == nil:
			p.exitCode = 0
		case errors.As(err, &ee):
			p.exitCode = ee.ExitCode()
		default:
			p.exitCode = -1
		}
		close(p.exited)
	}()
	sb.T.Cleanup(p.kill)
	return p
}

func answerTerminalQueries(tty *os.File, data []byte) {
	if bytes.Contains(data, []byte("\x1b]11;?")) {
		io.WriteString(tty, "\x1b]11;rgb:0000/0000/0000\x1b\\")
	}
	if bytes.Contains(data, []byte("\x1b[6n")) {
		io.WriteString(tty, "\x1b[1;1R")
	}
}

func (p *ptySession) output() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.buf.String()
}

func (p *ptySession) expect(substr string, timeout time.Duration) {
	p.t.Helper()
	deadline := time.Now().Add(timeout)
	for !strings.Contains(p.output(), substr) {
		if time.Now().After(deadline) {
			p.t.Fatalf("PTY did not print %q within %s; output:\n%s", substr, timeout, p.output())
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func (p *ptySession) send(s string) {
	p.t.Helper()
	if _, err := io.WriteString(p.tty, s); err != nil {
		p.t.Fatal(err)
	}
}

func (p *ptySession) wait(timeout time.Duration) (int, bool) {
	select {
	case <-p.exited:
	case <-time.After(timeout):
		return 0, false
	}
	select {
	case <-p.readerDone:
	case <-time.After(2 * time.Second):
	}
	return p.exitCode, true
}

func (p *ptySession) mustExit(timeout time.Duration) int {
	p.t.Helper()
	code, ok := p.wait(timeout)
	if !ok {
		p.t.Fatalf("process still running after %s; output:\n%s", timeout, p.output())
	}
	return code
}

func (p *ptySession) kill() {
	select {
	case <-p.exited:
	default:
		if p.cmd.Process != nil {
			_ = p.cmd.Process.Kill()
		}
	}
	p.wait(5 * time.Second)
	p.tty.Close()
}

const ptyTimeout = 30 * time.Second

func markerStore(sb *harness.Sandbox) string {
	sb.T.Helper()
	return sb.MakeStore("store.kdbx", map[string]string{"MARK": "marker-value"})
}

func TestTTY1_MasterPasswordFromTerminal(t *testing.T) {
	sb := newSandbox(t)
	store := markerStore(sb)

	p := startPTY(sb, sb.BaseEnv(), binaryPath, "--key-store", store, "--secrets=MARK:MARK", "--", "sh", "-c", `echo "ran:$MARK"`)
	p.expect("Enter password for", ptyTimeout)
	p.send(sb.Password + "\r")
	code := p.mustExit(ptyTimeout)
	out := p.output()
	if code != 0 {
		t.Fatalf("exit %d; output:\n%s", code, out)
	}
	if !strings.Contains(out, "ran:marker-value") {
		t.Errorf("child did not run with the secret; output:\n%s", out)
	}
	if strings.Contains(out, sb.Password) {
		t.Errorf("the master password was echoed to the terminal; output:\n%s", out)
	}
}

func TestTTY2_WrongMasterPassword(t *testing.T) {
	sb := newSandbox(t)
	store := markerStore(sb)

	p := startPTY(sb, sb.BaseEnv(), binaryPath, "--key-store", store, "--secrets=MARK:MARK", "--", "sh", "-c", `echo "ran:$MARK"`)
	p.expect("Enter password for", ptyTimeout)
	p.send("definitely-wrong\r")
	code := p.mustExit(ptyTimeout)
	if code == 0 {
		t.Errorf("expected a non-zero exit; output:\n%s", p.output())
	}
	if strings.Contains(p.output(), "ran:") {
		t.Errorf("child ran with a wrong password; output:\n%s", p.output())
	}
}

func TestTTY3_CtrlDIgnoredCtrlCAborts(t *testing.T) {
	sb := newSandbox(t)
	store := markerStore(sb)

	p := startPTY(sb, sb.BaseEnv(), binaryPath, "--key-store", store, "--secrets=MARK:MARK", "--", "sh", "-c", `echo "ran:$MARK"`)
	p.expect("Enter password for", ptyTimeout)
	p.send("\x04")
	if _, done := p.wait(time.Second); done {
		t.Fatalf("Ctrl+D is expected to be ignored by x/term.ReadPassword, but kdbx-cli exited; output:\n%s", p.output())
	}
	p.send("\x03")
	code := p.mustExit(ptyTimeout)
	if code == 0 {
		t.Errorf("expected a non-zero exit after Ctrl+C; output:\n%s", p.output())
	}
	if strings.Contains(p.output(), "ran:") {
		t.Errorf("child ran after Ctrl+C; output:\n%s", p.output())
	}
	termios, err := unix.IoctlGetTermios(int(p.tty.Fd()), unix.TCGETS)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("terminal echo after Ctrl+C: %v", termios.Lflag&unix.ECHO != 0)
}
