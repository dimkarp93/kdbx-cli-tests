//go:build e2e

package basic

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func writeRawConfig(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestConfigVersion1_MissingVersionRejected(t *testing.T) {
	sb := newSandbox(t)
	store := sb.MakeStore("store.kdbx", map[string]string{"TOKEN": "x"})
	writeRawConfig(t, sb.ConfigPath(), fmt.Sprintf(`{"sections":{"default":{"key-store":%q,"secrets":{"TOKEN":"T"}}}}`, store))

	r := sb.Run("--", "sh", "-c", "true")
	expectFail(t, r)
	if !strings.Contains(r.Stderr, "kdbx-cli migrate") {
		t.Errorf("expected the migrate hint in stderr:\n%s", r.Stderr)
	}
}

func TestConfigVersion2_UnexpectedVersionRejected(t *testing.T) {
	sb := newSandbox(t)
	store := sb.MakeStore("store.kdbx", map[string]string{"TOKEN": "x"})
	writeRawConfig(t, sb.ConfigPath(), fmt.Sprintf(`{"version":99,"sections":{"default":{"key-store":%q,"secrets":{"T":"TOKEN"}}}}`, store))

	r := sb.Run("--", "sh", "-c", "true")
	expectFail(t, r)
	if !strings.Contains(r.Stderr, "version 99") {
		t.Errorf("expected the found version in stderr:\n%s", r.Stderr)
	}
}

func TestConfigVersion3_CurrentVersionAccepted(t *testing.T) {
	sb := newSandbox(t)
	value := secret(t, "E2E_REG_PW")
	store := sb.MakeStore("store.kdbx", map[string]string{"TOKEN": value})
	writeRawConfig(t, sb.ConfigPath(), fmt.Sprintf(`{"version":1,"sections":{"default":{"key-store":%q,"secrets":{"T":"TOKEN"}}}}`, store))

	r := sb.Run("--", "sh", "-c", `printf %s "$T"`)
	expectOK(t, r)
	if r.Stdout != value {
		t.Errorf("stdout: got %q, want %q", r.Stdout, value)
	}
}
