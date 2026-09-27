//go:build e2e

package basic

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dimkarp93/kdbx-cli-tests/internal/harness"
)

func TestStdinKP1_DbCreateWithRepeatedPassword(t *testing.T) {
	sb := newSandbox(t)
	newPw := "new-" + secret(t, "E2E_REG_PW")
	storeWith(sb, map[string]string{"new-pw": newPw}, nil)
	db := filepath.Join(sb.Dir, "new.kdbx")

	expectOK(t, sb.Run("--stdin=new-pw,new-pw", "--", "keepassxc-cli", "db-create", "-p", db))
	expectOK(t, sb.Exec(sb.BaseEnv(), newPw+"\n", "keepassxc-cli", "ls", "-q", db))
	expectFail(t, sb.Exec(sb.BaseEnv(), "wrong\n", "keepassxc-cli", "ls", "-q", db))
}

func TestStdinSudo1_PasswordOverStdin(t *testing.T) {
	requireTool(t, "sudo")
	sb := newSandbox(t)
	storeWith(sb, map[string]string{"sudo-pw": secret(t, "E2E_SUDO_PW")}, nil)

	r := sb.Run("--stdin=sudo-pw", "--", "sudo", "-S", "-k", "id", "-u")
	expectOK(t, r)
	if got := strings.TrimSpace(r.Stdout); got != "0" {
		t.Errorf("id -u: got %q, want 0", got)
	}
}

func TestStdinSudo2_PasswordThenFileContent(t *testing.T) {
	requireTool(t, "sudo")
	sb := newSandbox(t)
	storeWith(sb, map[string]string{"sudo-pw": secret(t, "E2E_SUDO_PW")}, nil)
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
