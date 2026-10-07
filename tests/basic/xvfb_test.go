//go:build e2e

package basic

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

const zenityTimeout = 10 * time.Second

var xvfb struct {
	once    sync.Once
	cmd     *exec.Cmd
	display string
	err     error
}

func xDisplay(t *testing.T) string {
	t.Helper()
	requireTool(t, "Xvfb", "xdotool", "zenity")
	xvfb.once.Do(startXvfb)
	if xvfb.err != nil {
		t.Fatal(xvfb.err)
	}
	return xvfb.display
}

func startXvfb() {
	r, w, err := os.Pipe()
	if err != nil {
		xvfb.err = err
		return
	}
	defer r.Close()
	cmd := exec.Command("Xvfb", "-displayfd", "3", "-screen", "0", "1280x800x24", "-nolisten", "tcp")
	cmd.ExtraFiles = []*os.File{w}
	if err := cmd.Start(); err != nil {
		w.Close()
		xvfb.err = fmt.Errorf("start Xvfb: %w", err)
		return
	}
	w.Close()
	xvfb.cmd = cmd

	number := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(r).ReadString('\n')
		number <- strings.TrimSpace(line)
	}()
	select {
	case n := <-number:
		if n == "" {
			xvfb.err = fmt.Errorf("Xvfb did not report its display")
			return
		}
		xvfb.display = ":" + n
	case <-time.After(10 * time.Second):
		xvfb.err = fmt.Errorf("Xvfb did not start within 10s")
	}
}

func stopXvfb() {
	if xvfb.cmd != nil && xvfb.cmd.Process != nil {
		_ = xvfb.cmd.Process.Kill()
		_ = xvfb.cmd.Wait()
	}
}

func xdotool(display string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), zenityTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "xdotool", args...)
	cmd.Env = append(os.Environ(), "DISPLAY="+display)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("xdotool %v: %w: %s", args, err, out)
	}
	return string(out), nil
}

func answerZenity(display, typed, finalKey string) error {
	out, err := xdotool(display, "search", "--sync", "--name", "^kdbx-cli$")
	if err != nil {
		return fmt.Errorf("no zenity window appeared: %w", err)
	}
	window := strings.Fields(out)[0]
	if _, err := xdotool(display, "windowfocus", "--sync", window); err != nil {
		return err
	}
	if typed != "" {
		if _, err := xdotool(display, "type", "--delay", "5", typed); err != nil {
			return err
		}
	}
	_, err = xdotool(display, "key", finalKey)
	return err
}
