//go:build e2e

package basic

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileRestic1_PasswordFile(t *testing.T) {
	requireTool(t, "restic")
	sb := newSandbox(t)
	pw := "restic-" + secret(t, "E2E_REG_PW")
	repo := resticRepo(sb, pw)
	storeWith(sb, map[string]string{"restic-pw": pw, "wrong-pw": "nope"}, nil)

	expectOK(t, sb.Run("--secret-file=restic-pw", "--", "restic", "-r", repo, "--password-file", "{{restic-pw}}", "snapshots"))
	expectFail(t, sb.Run("--secret-file=wrong-pw", "--", "restic", "-r", repo, "--password-file", "{{wrong-pw}}", "snapshots"))
}

func TestFileReread1_SameFileOpenedTwice(t *testing.T) {
	requireTool(t, "restic")
	sb := newSandbox(t)
	pw := "restic-" + secret(t, "E2E_REG_PW")
	repo := resticRepo(sb, pw)
	storeWith(sb, map[string]string{"restic-pw": pw}, nil)
	data := filepath.Join(sb.Dir, "data.txt")
	if err := os.WriteFile(data, []byte("payload\n"), 0600); err != nil {
		t.Fatal(err)
	}

	r := sb.Run("--secret-file=restic-pw", "--", "sh", "-c",
		`restic -r "$1" --password-file "$2" backup -q "$3" && restic -r "$1" --password-file "$2" snapshots --json`,
		"sh", repo, "{{restic-pw}}", data)
	expectOK(t, r)
	if !strings.Contains(r.Stdout, data) {
		t.Errorf("second read did not see the snapshot:\n%s", r.Stdout)
	}
}

func TestFileGPG1_PassphraseFile(t *testing.T) {
	requireTool(t, "gpg", "gpgconf")
	sb := newSandbox(t)
	gnupg := filepath.Join(sb.Dir, "gnupg")
	if err := os.MkdirAll(gnupg, 0700); err != nil {
		t.Fatal(err)
	}
	sb.Env = append(sb.Env, "GNUPGHOME="+gnupg)
	t.Cleanup(func() { sb.Exec(sb.BaseEnv(), "", "gpgconf", "--kill", "gpg-agent") })

	pass := "gpg-" + secret(t, "E2E_REG_PW")
	plain := filepath.Join(sb.Dir, "f.txt")
	if err := os.WriteFile(plain, []byte("top secret text\n"), 0600); err != nil {
		t.Fatal(err)
	}
	enc := plain + ".gpg"
	expectOK(t, sb.Exec(sb.BaseEnv(), "", "gpg", "--batch", "--pinentry-mode", "loopback",
		"--passphrase", pass, "--symmetric", "-o", enc, plain))
	sb.Exec(sb.BaseEnv(), "", "gpgconf", "--kill", "gpg-agent")
	storeWith(sb, map[string]string{"gpg-pass": pass}, nil)

	r := sb.Run("--secret-file=gpg-pass", "--", "gpg", "--batch", "--no-symkey-cache", "--pinentry-mode", "loopback",
		"--passphrase-file", "{{gpg-pass}}", "-d", enc)
	expectOK(t, r)
	if r.Stdout != "top secret text\n" {
		t.Errorf("decrypted: got %q", r.Stdout)
	}
}
