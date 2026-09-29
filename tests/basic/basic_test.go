//go:build e2e

package basic

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dimkarp93/kdbx-cli-tests/internal/harness"
)

var binaryPath string

func TestMain(m *testing.M) {
	bin, _, err := harness.CheckBinary()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	binaryPath = bin
	if err := os.MkdirAll(os.Getenv("KDBX_CLI_E2E_ROOT"), 0755); err != nil {
		fmt.Fprintln(os.Stderr, "cannot create KDBX_CLI_E2E_ROOT:", err)
		os.Exit(2)
	}
	os.Exit(m.Run())
}

func newSandbox(t *testing.T) *harness.Sandbox {
	t.Helper()
	return harness.NewSandbox(t, binaryPath)
}

func TestE2E_InjectSecret(t *testing.T) {
	sb := newSandbox(t)
	store := sb.MakeStore("store.kdbx", map[string]string{"GITHUB_TOKEN": "ghp_secret"})
	sb.WriteConfig(map[string]harness.Section{
		"default": {KeyStore: store, Secrets: map[string]string{"GH_TOKEN": "GITHUB_TOKEN"}},
	})

	r := sb.Run("--", "sh", "-c", `printf %s "$GH_TOKEN"`)
	if r.ExitCode != 0 {
		t.Fatalf("exit %d\nstderr:\n%s", r.ExitCode, r.Stderr)
	}
	if r.Stdout != "ghp_secret" {
		t.Errorf("stdout: got %q, want %q", r.Stdout, "ghp_secret")
	}
}

func TestE2E_MergeDefaultAndToolSection(t *testing.T) {
	sb := newSandbox(t)
	store := sb.MakeStore("store.kdbx", map[string]string{
		"GITHUB_TOKEN": "ghp_secret",
		"API_KEY":      "api_secret",
	})
	sb.WriteConfig(map[string]harness.Section{
		"default": {KeyStore: store, Secrets: map[string]string{"GH_TOKEN": "GITHUB_TOKEN"}},
		"sh":      {Secrets: map[string]string{"API_KEY": "API_KEY"}},
	})

	r := sb.Run("--", "sh", "-c", `printf "%s|%s" "$GH_TOKEN" "$API_KEY"`)
	if r.ExitCode != 0 {
		t.Fatalf("exit %d\nstderr:\n%s", r.ExitCode, r.Stderr)
	}
	if r.Stdout != "ghp_secret|api_secret" {
		t.Errorf("stdout: got %q, want %q", r.Stdout, "ghp_secret|api_secret")
	}
}

func TestE2E_FlagsOverrideConfig(t *testing.T) {
	sb := newSandbox(t)
	store := sb.MakeStore("flag.kdbx", map[string]string{"TOKEN": "flag_secret"})

	r := sb.Run("--key-store", store, "--secrets=TOKEN:MY_TOKEN", "--", "sh", "-c", `printf %s "$MY_TOKEN"`)
	if r.ExitCode != 0 {
		t.Fatalf("exit %d\nstderr:\n%s", r.ExitCode, r.Stderr)
	}
	if r.Stdout != "flag_secret" {
		t.Errorf("stdout: got %q, want %q", r.Stdout, "flag_secret")
	}
}

func TestE2E_MissingSecret(t *testing.T) {
	sb := newSandbox(t)
	store := sb.MakeStore("store.kdbx", map[string]string{"GITHUB_TOKEN": "ghp_secret"})
	sb.WriteConfig(map[string]harness.Section{
		"default": {KeyStore: store, Secrets: map[string]string{"NOPE_ENV": "NOPE"}},
	})

	r := sb.Run("--", "sh", "-c", "echo should-not-run")
	if r.ExitCode == 0 {
		t.Fatalf("expected non-zero exit\nstdout:\n%s", r.Stdout)
	}
	if !strings.Contains(r.Stderr, "not found") {
		t.Errorf("expected 'not found' in stderr:\n%s", r.Stderr)
	}
	if strings.Contains(r.Stdout, "should-not-run") {
		t.Errorf("child command ran despite missing secret:\n%s", r.Stdout)
	}
}

func TestE2E_ExitCodePropagation(t *testing.T) {
	sb := newSandbox(t)
	store := sb.MakeStore("store.kdbx", map[string]string{"TOKEN": "secret"})
	sb.WriteConfig(map[string]harness.Section{
		"default": {KeyStore: store, Secrets: map[string]string{"TOKEN": "TOKEN"}},
	})

	r := sb.Run("--", "sh", "-c", "exit 7")
	if r.ExitCode != 7 {
		t.Errorf("exit code: got %d, want 7\nstderr:\n%s", r.ExitCode, r.Stderr)
	}
}

func TestE2E_DryRun(t *testing.T) {
	sb := newSandbox(t)
	sb.WriteConfig(map[string]harness.Section{
		"default": {KeyStore: "~/missing-store.kdbx", Secrets: map[string]string{"GH_TOKEN": "GITHUB_TOKEN"}},
		"sh":      {Secrets: map[string]string{"API_KEY": "API_KEY"}},
	})

	r := sb.RunNoPassword("--dry-run", "--", "sh", "-c", "echo MARKER_EXECUTED")
	if r.ExitCode != 0 {
		t.Fatalf("dry-run exit %d\nstderr:\n%s", r.ExitCode, r.Stderr)
	}
	if n := strings.Count(r.Stdout, "MARKER_EXECUTED"); n != 1 {
		t.Errorf("marker appears %d times (want 1 — only in the plan); command was executed:\n%s", n, r.Stdout)
	}
	for _, want := range []string{
		"Dry run",
		`"sh" (merged over "default")`,
		"← GITHUB_TOKEN",
		"← API_KEY",
		"GH_TOKEN=<secret from GITHUB_TOKEN>",
		"API_KEY=<secret from API_KEY>",
	} {
		if !strings.Contains(r.Stdout, want) {
			t.Errorf("dry-run output missing %q:\n%s", want, r.Stdout)
		}
	}
}

func TestE2E_CheckAddsMissingSecrets(t *testing.T) {
	sb := newSandbox(t)
	store := sb.MakeStore("store.kdbx", map[string]string{"GITHUB_TOKEN": "ghp_secret"})
	sb.WriteConfig(map[string]harness.Section{
		"default": {KeyStore: store, Secrets: map[string]string{
			"GH_TOKEN":  "GITHUB_TOKEN",
			"NPM_TOKEN": "NPM_TOKEN",
		}},
	})

	r := sb.Run("check", "-y")
	if r.ExitCode != 0 {
		t.Fatalf("check exit %d\nstderr:\n%s", r.ExitCode, r.Stderr)
	}
	if !strings.Contains(r.Stdout, "NPM_TOKEN") {
		t.Errorf("check did not report missing NPM_TOKEN:\n%s", r.Stdout)
	}

	titles := sb.StoreTitles(store)
	if !titles["NPM_TOKEN"] || !titles["GITHUB_TOKEN"] {
		t.Errorf("NPM_TOKEN not added to store, titles=%v", titles)
	}
}

func TestE2E_CheckAllPresent(t *testing.T) {
	sb := newSandbox(t)
	store := sb.MakeStore("store.kdbx", map[string]string{"TOKEN": "v"})
	sb.WriteConfig(map[string]harness.Section{
		"default": {KeyStore: store, Secrets: map[string]string{"TOKEN": "TOKEN"}},
	})

	r := sb.Run("check", "-y")
	if r.ExitCode != 0 {
		t.Fatalf("check exit %d\nstderr:\n%s", r.ExitCode, r.Stderr)
	}
	if !strings.Contains(r.Stdout, "All secrets are present") {
		t.Errorf("expected all-present message:\n%s", r.Stdout)
	}
}

func TestE2E_ConfigCreatesStoreAndSecrets(t *testing.T) {
	sb := newSandbox(t)
	store := filepath.Join(sb.Home, "new", "store.kdbx")
	cfgPath := filepath.Join(sb.Home, ".config", "kdbx-cli", "default")

	stdin := store + "\n" + "GITHUB_TOKEN:GH_TOKEN,API_KEY:API_KEY\n"
	r := sb.RunStdin(stdin, "config", "-y", "--config", cfgPath)
	if r.ExitCode != 0 {
		t.Fatalf("config exit %d\nstdout:\n%s\nstderr:\n%s", r.ExitCode, r.Stdout, r.Stderr)
	}

	if _, err := os.Stat(store); err != nil {
		t.Fatalf("store not created: %v", err)
	}
	titles := sb.StoreTitles(store)
	if !titles["GITHUB_TOKEN"] || !titles["API_KEY"] {
		t.Errorf("created store missing secrets, titles=%v", titles)
	}

	cfg, err := harness.LoadConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Sections["default"].KeyStore != store {
		t.Errorf("config key-store: got %q, want %q", cfg.Sections["default"].KeyStore, store)
	}
}

func TestE2E_WrongPassword(t *testing.T) {
	sb := newSandbox(t)
	store := sb.MakeStore("store.kdbx", map[string]string{"TOKEN": "secret"})
	sb.WriteConfig(map[string]harness.Section{
		"default": {KeyStore: store, Secrets: map[string]string{"TOKEN": "TOKEN"}},
	})

	r := sb.RunWithPassword("wrong-password", "--", "sh", "-c", "echo nope")
	if r.Stdout == "nope\n" {
		t.Errorf("child ran with wrong password")
	}
	if !strings.Contains(r.Stderr, "keepassxc-cli") {
		t.Errorf("expected keepassxc-cli error in stderr:\n%s", r.Stderr)
	}
}

func TestE2E_StdinDelivery(t *testing.T) {
	sb := newSandbox(t)
	store := sb.MakeStore("store.kdbx", map[string]string{"REG_TOKEN": "tok_secret"})
	sb.WriteConfig(map[string]harness.Section{
		"default": {KeyStore: store, Stdin: []string{"REG_TOKEN", "REG_TOKEN"}},
	})

	r := sb.Run("--", "sh", "-c", `read a; read b; printf "%s|%s" "$a" "$b"; cat`)
	if r.ExitCode != 0 {
		t.Fatalf("exit %d\nstderr:\n%s", r.ExitCode, r.Stderr)
	}
	if r.Stdout != "tok_secret|tok_secret" {
		t.Errorf("stdout: got %q", r.Stdout)
	}
}

func TestE2E_StdinKeepOpen(t *testing.T) {
	sb := newSandbox(t)
	store := sb.MakeStore("store.kdbx", map[string]string{"PW": "pw_secret"})

	r := sb.RunStdin("rest-of-stdin\n",
		"--key-store", store, "--stdin=PW", "--stdin-keep-open",
		"--", "sh", "-c", `read a; read b; printf "%s|%s" "$a" "$b"`)
	if r.ExitCode != 0 {
		t.Fatalf("exit %d\nstderr:\n%s", r.ExitCode, r.Stderr)
	}
	if r.Stdout != "pw_secret|rest-of-stdin" {
		t.Errorf("stdout: got %q", r.Stdout)
	}
}

func TestE2E_SecretFileDelivery(t *testing.T) {
	sb := newSandbox(t)
	store := sb.MakeStore("store.kdbx", map[string]string{"repo-pw": "file_secret"})

	r := sb.Run("--key-store", store, "--secret-file=repo-pw",
		"--", "sh", "-c", `cat "$1"; cat "$1"`, "sh", "{{repo-pw}}")
	if r.ExitCode != 0 {
		t.Fatalf("exit %d\nstderr:\n%s", r.ExitCode, r.Stderr)
	}
	if r.Stdout != "file_secretfile_secret" {
		t.Errorf("stdout: got %q", r.Stdout)
	}
}

func TestE2E_SecretFileMissingPlaceholder(t *testing.T) {
	sb := newSandbox(t)
	store := sb.MakeStore("store.kdbx", map[string]string{"repo-pw": "file_secret"})

	r := sb.Run("--key-store", store, "--secret-file=repo-pw", "--", "sh", "-c", "echo should-not-run")
	if r.ExitCode == 0 {
		t.Fatalf("expected non-zero exit\nstdout:\n%s", r.Stdout)
	}
	if strings.Contains(r.Stdout, "should-not-run") {
		t.Errorf("child ran despite the missing placeholder:\n%s", r.Stdout)
	}
	if !strings.Contains(r.Stderr, "{{repo-pw}}") {
		t.Errorf("expected the placeholder in stderr:\n%s", r.Stderr)
	}
}

func TestE2E_AskpassDelivery(t *testing.T) {
	sb := newSandbox(t)
	store := sb.MakeStore("store.kdbx", map[string]string{"ssh-pw": "askpass_secret"})

	r := sb.Run("--key-store", store, "--askpass=ssh-pw",
		"--", "sh", "-c", `"$SSH_ASKPASS" "Password:"; "$GIT_ASKPASS" "Password:"; printf %s "$SSH_ASKPASS_REQUIRE"`)
	if r.ExitCode != 0 {
		t.Fatalf("exit %d\nstderr:\n%s", r.ExitCode, r.Stderr)
	}
	if r.Stdout != "askpass_secret\naskpass_secret\nforce" {
		t.Errorf("stdout: got %q", r.Stdout)
	}
}

func TestE2E_DryRunDeliveryModes(t *testing.T) {
	sb := newSandbox(t)
	sb.WriteConfig(map[string]harness.Section{
		"default": {KeyStore: "~/missing-store.kdbx"},
		"restic":  {Files: []string{"repo-pw"}, Askpass: "ssh-pw", Stdin: []string{"tok"}},
	})

	r := sb.RunNoPassword("--dry-run", "--", "restic", "--password-file", "{{repo-pw}}")
	if r.ExitCode != 0 {
		t.Fatalf("dry-run exit %d\nstderr:\n%s", r.ExitCode, r.Stderr)
	}
	for _, want := range []string{
		"<secret from tok>",
		"{{repo-pw}} ← repo-pw",
		"Askpass:      ssh-pw",
		"restic --password-file '<file with repo-pw>'",
	} {
		if !strings.Contains(r.Stdout, want) {
			t.Errorf("dry-run output missing %q:\n%s", want, r.Stdout)
		}
	}
}

func TestE2E_CheckCoversAllChannels(t *testing.T) {
	sb := newSandbox(t)
	store := sb.MakeStore("store.kdbx", map[string]string{"PRESENT": "v"})
	sb.WriteConfig(map[string]harness.Section{
		"default": {KeyStore: store, Secrets: map[string]string{"PRESENT": "PRESENT"}},
		"docker":  {Stdin: []string{"REG_TOKEN"}},
		"restic":  {Files: []string{"REPO_PW"}},
		"ssh":     {Askpass: "SSH_PW"},
	})

	r := sb.Run("check", "-y")
	if r.ExitCode != 0 {
		t.Fatalf("check exit %d\nstderr:\n%s", r.ExitCode, r.Stderr)
	}
	titles := sb.StoreTitles(store)
	for _, want := range []string{"REG_TOKEN", "REPO_PW", "SSH_PW"} {
		if !titles[want] {
			t.Errorf("%s was not added to the store, titles=%v", want, titles)
		}
	}
}
