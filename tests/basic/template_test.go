//go:build e2e

package basic

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTemplate(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTemplate1_JSONFileRendered(t *testing.T) {
	requireTool(t, "cat")
	sb := newSandbox(t)
	pw := "admin-" + secret(t, "E2E_REG_PW")
	store := sb.MakeStore("store.kdbx", map[string]string{"AdminPw": pw})
	tmplPath := writeTemplate(t, sb.Dir, "settings.json.tmpl", `{"user":"Bob","password":"{{AdminPw}}"}`)

	r := sb.Run("--key-store", store, "--template=cfg:"+tmplPath, "--", "cat", "{{cfg}}")
	expectOK(t, r)

	var decoded map[string]string
	if err := json.Unmarshal([]byte(r.Stdout), &decoded); err != nil {
		t.Fatalf("rendered output is not valid JSON: %v\n%s", err, r.Stdout)
	}
	if decoded["user"] != "Bob" || decoded["password"] != pw {
		t.Errorf("decoded: %+v", decoded)
	}
}

func TestTemplate2_JSONEscaping(t *testing.T) {
	requireTool(t, "cat")
	sb := newSandbox(t)
	pw := `has "quotes" and` + "\nnewline-" + secret(t, "E2E_REG_PW")
	store := sb.MakeStore("store.kdbx", map[string]string{"AdminPw": pw})
	tmplPath := writeTemplate(t, sb.Dir, "settings.json", `{"password":"{{AdminPw}}"}`)

	r := sb.Run("--key-store", store, "--template=cfg:"+tmplPath, "--", "cat", "{{cfg}}")
	expectOK(t, r)

	var decoded map[string]string
	if err := json.Unmarshal([]byte(r.Stdout), &decoded); err != nil {
		t.Fatalf("rendered output is not valid JSON: %v\n%s", err, r.Stdout)
	}
	if decoded["password"] != pw {
		t.Errorf("decoded password: got %q, want %q", decoded["password"], pw)
	}
}

func TestTemplate3_MissingPlaceholderInArgv(t *testing.T) {
	requireTool(t, "cat")
	sb := newSandbox(t)
	store := sb.MakeStore("store.kdbx", map[string]string{"AdminPw": "pw"})
	tmplPath := writeTemplate(t, sb.Dir, "settings.json", `{"password":"{{AdminPw}}"}`)

	r := sb.Run("--key-store", store, "--template=cfg:"+tmplPath, "--", "cat", tmplPath)
	expectFail(t, r)
	if !strings.Contains(r.Stderr, "{{cfg}}") {
		t.Errorf("expected the placeholder in stderr:\n%s", r.Stderr)
	}
}

func TestTemplate4_MissingSecretInStore(t *testing.T) {
	requireTool(t, "cat")
	sb := newSandbox(t)
	store := sb.MakeStore("store.kdbx", map[string]string{"Other": "pw"})
	tmplPath := writeTemplate(t, sb.Dir, "settings.json", `{"password":"{{AdminPw}}"}`)

	r := sb.Run("--key-store", store, "--template=cfg:"+tmplPath, "--", "cat", "{{cfg}}")
	expectFail(t, r)
	if !strings.Contains(r.Stderr, "AdminPw") {
		t.Errorf("expected the missing title in stderr:\n%s", r.Stderr)
	}
}
