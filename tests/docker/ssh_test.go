//go:build e2e

package docker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const authorizedKeys = "/seed/ssh/authorized_keys"

var sshBaseOpts = []string{
	"-o", "StrictHostKeyChecking=no",
	"-o", "UserKnownHostsFile=/dev/null",
	"-o", "LogLevel=ERROR",
	"-o", "ConnectTimeout=10",
}

func sshPasswordArgs() []string {
	args := append([]string{"ssh"}, sshBaseOpts...)
	return append(args,
		"-o", "PubkeyAuthentication=no",
		"-o", "PreferredAuthentications=password",
		"-o", "NumberOfPasswordPrompts=1",
		"alice@sshd", "echo", "ok")
}

func sshKeyArgs(key string) []string {
	args := append([]string{"ssh"}, sshBaseOpts...)
	return append(args,
		"-i", key,
		"-o", "IdentitiesOnly=yes",
		"-o", "PreferredAuthentications=publickey",
		"alice@sshd", "echo", "ok")
}

func (s *sandbox) authorizedKey(passphrase string) (string, string) {
	s.T.Helper()
	requireTool(s.T, "ssh-keygen")
	dir := filepath.Join(s.Home, ".ssh")
	if err := os.MkdirAll(dir, 0700); err != nil {
		s.T.Fatal(err)
	}
	key := filepath.Join(dir, "id_e2e")
	expectOK(s.T, s.Exec(s.BaseEnv(), "", "ssh-keygen", "-q", "-t", "ed25519", "-N", passphrase, "-C", s.T.Name(), "-f", key))
	pub, err := os.ReadFile(key + ".pub")
	if err != nil {
		s.T.Fatal(err)
	}
	f, err := os.OpenFile(authorizedKeys, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		s.T.Fatal(err)
	}
	defer f.Close()
	if _, err := f.Write(pub); err != nil {
		s.T.Fatal(err)
	}
	priv, err := os.ReadFile(key)
	if err != nil {
		s.T.Fatal(err)
	}
	return key, string(priv)
}

func expectSSHOK(t *testing.T, stdout string) {
	t.Helper()
	if got := strings.TrimSpace(stdout); got != "ok" {
		t.Errorf("remote output: got %q, want ok", got)
	}
}

func TestFileSSH1_IdentityFileIsClosedBySSH(t *testing.T) {
	requireTool(t, "ssh")
	sb := newSandbox(t)
	key, priv := sb.authorizedKey("")
	if err := os.Remove(key); err != nil {
		t.Fatal(err)
	}
	store := sb.MakeStore("store.kdbx", map[string]string{"ssh-key": priv})

	r := sb.Run(append([]string{"--key-store", store, "--secret-file=ssh-key", "--"}, sshKeyArgs("{{ssh-key}}")...)...)
	expectFail(t, r)
	if !strings.Contains(r.Stderr, "/dev/fd/3 not accessible") {
		t.Errorf("expected ssh to report the closed descriptor (it calls closefrom(3) at startup):\n%s", r.Stderr)
	}
}

func TestFileSSH2_PrivateKeyThroughSSHAdd(t *testing.T) {
	requireTool(t, "ssh", "ssh-agent", "ssh-add")
	sb := newSandbox(t)
	key, priv := sb.authorizedKey("")
	if err := os.Remove(key); err != nil {
		t.Fatal(err)
	}
	store := sb.MakeStore("store.kdbx", map[string]string{"ssh-key": priv})

	script := `eval "$(ssh-agent -s)" >/dev/null && trap 'kill $SSH_AGENT_PID' EXIT && ssh-add -q "$1" && shift && "$@"`
	args := append([]string{"--key-store", store, "--secret-file=ssh-key", "--", "sh", "-c", script, "sh", "{{ssh-key}}"},
		sshKeyArgs(key+".pub")...)
	r := sb.Run(args...)
	expectOK(t, r)
	expectSSHOK(t, r.Stdout)
}
