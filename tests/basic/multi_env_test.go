//go:build e2e

package basic

import (
	"strings"
	"testing"

	"github.com/dimkarp93/kdbx-cli-tests/internal/harness"
)

const multiEnvScript = `printf "%s|%s|%s" "$PGPASSWORD" "$DB_PASS" "$DB_SECRET"`

func TestMultiEnv1_OneSecretSeveralVariablesFromConfig(t *testing.T) {
	sb := newSandbox(t)
	value := secret(t, "E2E_REG_PW")
	storeWith(sb, map[string]string{"db/password": value}, map[string]harness.Section{
		"default": {Secrets: map[string]string{
			"PGPASSWORD": "db/password",
			"DB_PASS":    "db/password",
			"DB_SECRET":  "db/password",
		}},
	})

	r := sb.Run("--", "sh", "-c", multiEnvScript)
	expectOK(t, r)
	if want := strings.Join([]string{value, value, value}, "|"); r.Stdout != want {
		t.Errorf("stdout: got %q, want %q", r.Stdout, want)
	}
}

func TestMultiEnv2_OneSecretSeveralVariablesFromFlag(t *testing.T) {
	sb := newSandbox(t)
	value := secret(t, "E2E_REG_PW")
	store := sb.MakeStore("flag.kdbx", map[string]string{"db/password": value})

	r := sb.Run("--key-store", store, "--secrets=db/password:PGPASSWORD,db/password:DB_PASS", "--secrets=db/password:DB_SECRET", "--", "sh", "-c", multiEnvScript)
	expectOK(t, r)
	if want := strings.Join([]string{value, value, value}, "|"); r.Stdout != want {
		t.Errorf("stdout: got %q, want %q", r.Stdout, want)
	}
}

func TestMultiEnv3_ToolSectionAddsAndRepointsVariables(t *testing.T) {
	sb := newSandbox(t)
	storeWith(sb, map[string]string{"first": "value-first", "second": "value-second"}, map[string]harness.Section{
		"default": {Secrets: map[string]string{"A_ENV": "first", "B_ENV": "first"}},
		"sh":      {Secrets: map[string]string{"B_ENV": "second", "C_ENV": "first"}},
	})

	r := sb.Run("--", "sh", "-c", `printf "%s|%s|%s" "$A_ENV" "$B_ENV" "$C_ENV"`)
	expectOK(t, r)
	if want := "value-first|value-second|value-first"; r.Stdout != want {
		t.Errorf("stdout: got %q, want %q", r.Stdout, want)
	}
}

func TestMultiEnv4_SameSecretInEnvAndStdin(t *testing.T) {
	sb := newSandbox(t)
	value := secret(t, "E2E_REG_PW")
	storeWith(sb, map[string]string{"shared": value}, map[string]harness.Section{
		"default": {
			Secrets: map[string]string{"FROM_ENV": "shared", "FROM_ENV_TOO": "shared"},
			Stdin:   []string{"shared"},
		},
	})

	r := sb.Run("--", "sh", "-c", `read line; printf "%s|%s|%s" "$FROM_ENV" "$FROM_ENV_TOO" "$line"`)
	expectOK(t, r)
	if want := strings.Join([]string{value, value, value}, "|"); r.Stdout != want {
		t.Errorf("stdout: got %q, want %q", r.Stdout, want)
	}
}

func TestMultiEnv5_DryRunListsEveryVariableWithoutSecret(t *testing.T) {
	sb := newSandbox(t)
	value := secret(t, "E2E_REG_PW")
	storeWith(sb, map[string]string{"db/password": value}, map[string]harness.Section{
		"default": {Secrets: map[string]string{"PGPASSWORD": "db/password", "DB_PASS": "db/password"}},
	})

	r := sb.RunNoPassword("--dry-run", "--", "psql", "-c", "select 1")
	expectOK(t, r)
	for _, want := range []string{"PGPASSWORD", "DB_PASS", "<secret from db/password>"} {
		if !strings.Contains(r.Stdout, want) {
			t.Errorf("dry-run output misses %q:\n%s", want, r.Stdout)
		}
	}
	if strings.Contains(r.Stdout, value) || strings.Contains(r.Stderr, value) {
		t.Errorf("dry-run leaked the secret value")
	}
}

func TestMultiEnv6_CheckCreatesSharedEntryOnce(t *testing.T) {
	sb := newSandbox(t)
	store := sb.MakeStore("store.kdbx", map[string]string{"present": "x"})
	sb.WriteConfig(map[string]harness.Section{
		"default": {KeyStore: store, Secrets: map[string]string{
			"PRESENT_ENV": "present",
			"NEW_ENV":     "grp/new",
			"NEW_ENV_TOO": "grp/new",
		}},
	})

	r := sb.Run("check", "-y")
	expectOK(t, r)
	if got := strings.Count(r.Stdout, "- grp/new"); got != 1 {
		t.Errorf("grp/new reported %d times, want once:\n%s", got, r.Stdout)
	}
	second := sb.Run("check", "-y")
	expectOK(t, second)
	if !strings.Contains(second.Stdout, "All secrets are present") {
		t.Errorf("second check should find everything present:\n%s", second.Stdout)
	}
}

func TestMultiEnv7_InvalidVariableNameRejected(t *testing.T) {
	sb := newSandbox(t)
	storeWith(sb, map[string]string{"token": "x"}, map[string]harness.Section{
		"default": {Secrets: map[string]string{"BAD=NAME": "token", "1ST": "token", "GOOD": "token"}},
	})

	r := sb.Run("--", "sh", "-c", "true")
	expectFail(t, r)
	for _, want := range []string{`"BAD=NAME"`, `"1ST"`} {
		if !strings.Contains(r.Stderr, want) {
			t.Errorf("stderr misses %s:\n%s", want, r.Stderr)
		}
	}
	if strings.Contains(r.Stderr, `"GOOD"`) {
		t.Errorf("stderr mentions a valid name:\n%s", r.Stderr)
	}
	expectFail(t, sb.RunNoPassword("--dry-run", "--", "sh", "-c", "true"))
}
