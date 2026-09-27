//go:build e2e

package basic

import (
	"strings"
	"testing"
)

func TestAskSudo1_SudoAskpass(t *testing.T) {
	requireTool(t, "sudo")
	sb := newSandbox(t)
	store := sb.MakeStore("store.kdbx", map[string]string{"sudo-pw": secret(t, "E2E_SUDO_PW")})

	r := sb.Run("--key-store", store, "--askpass=sudo-pw", "--", "sudo", "-A", "-k", "id", "-u")
	expectOK(t, r)
	if got := strings.TrimSpace(r.Stdout); got != "0" {
		t.Errorf("id -u: got %q, want 0", got)
	}
}

func TestAskRestic1_PasswordCommand(t *testing.T) {
	requireTool(t, "restic")
	sb := newSandbox(t)
	pw := "restic-" + secret(t, "E2E_REG_PW")
	repo := resticRepo(sb, pw)
	store := sb.MakeStore("store.kdbx", map[string]string{"restic-pw": pw})

	expectOK(t, sb.Run("--key-store", store, "--askpass=restic-pw", "--", "restic", "-r", repo, "snapshots"))
}
