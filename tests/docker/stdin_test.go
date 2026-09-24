//go:build e2e

package docker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dimkarp93/kdbx-cli-tests/internal/harness"
)

func TestStdinPG1_PasswordFromStdinWithoutTTY(t *testing.T) {
	requireTool(t, "psql")
	requireNoControllingTTY(t)
	sb := newSandbox(t)
	sb.storeWith(map[string]string{"pg-stdin": secret(t, "E2E_PG_STDIN_PW")}, nil)

	r := sb.Run("--stdin=pg-stdin", "--", "psql", "-h", "postgres", "-U", "app_stdin", "-d", "postgres", "-W", "-Atc", "select 1")
	expectOK(t, r)
	if got := strings.TrimSpace(r.Stdout); got != "1" {
		t.Errorf("stdout: got %q, want 1", got)
	}
}

func TestStdinPG2_PasswordAndSQLShareStdin(t *testing.T) {
	requireTool(t, "psql")
	requireNoControllingTTY(t)
	sb := newSandbox(t)
	sb.storeWith(map[string]string{"pg-stdin": secret(t, "E2E_PG_STDIN_PW")}, nil)

	r := sb.RunStdin("select current_user;\n", "--stdin=pg-stdin", "--stdin-keep-open",
		"--", "psql", "-h", "postgres", "-U", "app_stdin", "-d", "postgres", "-W", "-At")
	expectOK(t, r)
	if got := strings.TrimSpace(r.Stdout); got != "app_stdin" {
		t.Errorf("stdout: got %q, want app_stdin", got)
	}
}

func TestStdinReg1_SkopeoLoginPasswordStdin(t *testing.T) {
	requireTool(t, "skopeo")
	sb := newSandbox(t)
	sb.storeWith(map[string]string{"reg-pw": secret(t, "E2E_REG_PW")}, nil)

	r := sb.RunEnv([]string{"KDBX_CLI_PASSWORD=" + sb.Password, "REGISTRY_AUTH_FILE=" + filepath.Join(sb.Dir, "auth.json")}, "",
		"--stdin=reg-pw", "--", "skopeo", "login", "--tls-verify=false", "-u", "alice", "--password-stdin", "registry:5000")
	expectOK(t, r)
	if !strings.Contains(r.Stdout, "Login Succeeded") {
		t.Errorf("expected Login Succeeded:\n%s", r.Stdout)
	}
}

func TestStdinReg1_WrongPasswordRejected(t *testing.T) {
	requireTool(t, "skopeo")
	sb := newSandbox(t)
	sb.storeWith(map[string]string{"reg-pw": "not-the-password"}, nil)

	r := sb.RunEnv([]string{"KDBX_CLI_PASSWORD=" + sb.Password, "REGISTRY_AUTH_FILE=" + filepath.Join(sb.Dir, "auth.json")}, "",
		"--stdin=reg-pw", "--", "skopeo", "login", "--tls-verify=false", "-u", "alice", "--password-stdin", "registry:5000")
	expectFail(t, r)
}

func TestStdinKP1_DbCreateWithRepeatedPassword(t *testing.T) {
	sb := newSandbox(t)
	newPw := "new-" + secret(t, "E2E_REG_PW")
	sb.storeWith(map[string]string{"new-pw": newPw}, nil)
	db := filepath.Join(sb.Dir, "new.kdbx")

	expectOK(t, sb.Run("--stdin=new-pw,new-pw", "--", "keepassxc-cli", "db-create", "-p", db))
	expectOK(t, sb.Exec(sb.BaseEnv(), newPw+"\n", "keepassxc-cli", "ls", "-q", db))
	expectFail(t, sb.Exec(sb.BaseEnv(), "wrong\n", "keepassxc-cli", "ls", "-q", db))
}

func TestStdinSudo1_PasswordOverStdin(t *testing.T) {
	requireTool(t, "sudo")
	sb := newSandbox(t)
	sb.storeWith(map[string]string{"sudo-pw": secret(t, "E2E_SUDO_PW")}, nil)

	r := sb.Run("--stdin=sudo-pw", "--", "sudo", "-S", "-k", "id", "-u")
	expectOK(t, r)
	if got := strings.TrimSpace(r.Stdout); got != "0" {
		t.Errorf("id -u: got %q, want 0", got)
	}
}

func TestStdinSudo2_PasswordThenFileContent(t *testing.T) {
	requireTool(t, "sudo")
	sb := newSandbox(t)
	sb.storeWith(map[string]string{"sudo-pw": secret(t, "E2E_SUDO_PW")}, nil)
	target := "/etc/e2e-" + harness.SafeName(t.Name()) + ".conf"
	content := "key = value\nsecond = line\n"

	r := sb.RunStdin(content, "--stdin=sudo-pw", "--stdin-keep-open", "--", "sudo", "-S", "-k", "tee", target)
	expectOK(t, r)
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != content {
		t.Errorf("%s: got %q, want %q", target, got, content)
	}
}
