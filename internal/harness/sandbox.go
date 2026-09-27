package harness

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const TestPassword = "test-pass-123"

const CommandTimeout = 30 * time.Second

type Sandbox struct {
	T        *testing.T
	Dir      string
	Home     string
	Bin      string
	Password string
	Env      []string
}

type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

func NewSandbox(t *testing.T, bin string) *Sandbox {
	t.Helper()
	root := os.Getenv("KDBX_CLI_E2E_ROOT")
	if root == "" {
		t.Fatal("KDBX_CLI_E2E_ROOT is required")
	}
	keep := os.Getenv("KDBX_CLI_E2E_KEEP") == "1"

	dir := filepath.Join(root, SafeName(t.Name()))
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(dir, "home")
	if err := os.MkdirAll(filepath.Join(home, ".config", "kdbx-cli"), 0755); err != nil {
		t.Fatal(err)
	}
	sb := &Sandbox{T: t, Dir: dir, Home: home, Bin: bin, Password: TestPassword}
	if !keep {
		t.Cleanup(func() { _ = os.RemoveAll(dir) })
	}
	return sb
}

func SafeName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

func xmlEsc(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func buildTestXML(entries map[string]string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	b.WriteString("<KeePassFile><Root><Group><Name>Root</Name>\n")
	for title, pass := range entries {
		b.WriteString("<Entry>\n")
		b.WriteString("  <String><Key>Title</Key><Value>" + xmlEsc(title) + "</Value></String>\n")
		b.WriteString("  <String><Key>Password</Key><Value>" + xmlEsc(pass) + "</Value></String>\n")
		b.WriteString("</Entry>\n")
	}
	b.WriteString("</Group></Root></KeePassFile>\n")
	return b.String()
}

func (s *Sandbox) MakeStore(rel string, entries map[string]string) string {
	s.T.Helper()
	dbPath := filepath.Join(s.Home, rel)
	if err := os.MkdirAll(filepath.Dir(dbPath), 0700); err != nil {
		s.T.Fatal(err)
	}
	xmlPath := filepath.Join(s.Dir, strings.ReplaceAll(rel, "/", "_")+".import.xml")
	if err := os.WriteFile(xmlPath, []byte(buildTestXML(entries)), 0600); err != nil {
		s.T.Fatal(err)
	}
	defer os.Remove(xmlPath)
	out, err := KeepassRun(s.Password+"\n"+s.Password+"\n", "import", "-q", "-p", xmlPath, dbPath)
	if err != nil {
		s.T.Fatalf("keepassxc-cli import failed: %v\n%s", err, out)
	}
	return dbPath
}

func (s *Sandbox) ConfigPath() string {
	return filepath.Join(s.Home, ".config", "kdbx-cli", "default")
}

func (s *Sandbox) WriteConfig(sections map[string]Section) {
	s.T.Helper()
	data, _ := json.MarshalIndent(Config{Sections: sections}, "", "  ")
	if err := os.WriteFile(s.ConfigPath(), data, 0600); err != nil {
		s.T.Fatal(err)
	}
}

func (s *Sandbox) StoreTitles(dbPath string) map[string]bool {
	s.T.Helper()
	out, err := KeepassRun(s.Password+"\n", "export", "-q", "-f", "xml", dbPath)
	if err != nil {
		s.T.Fatalf("export %s failed: %v\n%s", dbPath, err, out)
	}
	titles, err := ExportTitles(out)
	if err != nil {
		s.T.Fatal(err)
	}
	return titles
}

func (s *Sandbox) BaseEnv() []string {
	env := append(os.Environ(), "HOME="+s.Home)
	return append(env, s.Env...)
}

func (s *Sandbox) Exec(env []string, stdin string, name string, args ...string) Result {
	s.T.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), CommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = s.Dir
	cmd.Env = env
	cmd.WaitDelay = 2 * time.Second
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		s.T.Fatalf("%s %v timed out after %s\nstdout:\n%s\nstderr:\n%s", name, args, CommandTimeout, stdout.String(), stderr.String())
	}
	code := 0
	if err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			s.T.Fatalf("run %s %v: %v", name, args, err)
		}
		code = ee.ExitCode()
	}
	return Result{Stdout: stdout.String(), Stderr: stderr.String(), ExitCode: code}
}

func (s *Sandbox) RunEnv(extraEnv []string, stdin string, args ...string) Result {
	s.T.Helper()
	return s.Exec(append(s.BaseEnv(), extraEnv...), stdin, s.Bin, args...)
}

func (s *Sandbox) Run(args ...string) Result {
	s.T.Helper()
	return s.RunEnv([]string{"KDBX_CLI_PASSWORD=" + s.Password}, "", args...)
}

func (s *Sandbox) RunStdin(stdin string, args ...string) Result {
	s.T.Helper()
	return s.RunEnv([]string{"KDBX_CLI_PASSWORD=" + s.Password}, stdin, args...)
}

func (s *Sandbox) RunWithPassword(password string, args ...string) Result {
	s.T.Helper()
	return s.RunEnv([]string{"KDBX_CLI_PASSWORD=" + password}, "", args...)
}

func (s *Sandbox) RunNoPassword(args ...string) Result {
	s.T.Helper()
	return s.RunEnv(nil, "", args...)
}
