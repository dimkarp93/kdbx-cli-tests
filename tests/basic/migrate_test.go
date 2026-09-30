//go:build e2e

package basic

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/dimkarp93/kdbx-cli-tests/internal/harness"
)

func writeLegacyConfig(t *testing.T, path, store string, secrets map[string]string) {
	t.Helper()
	var pairs []string
	for entry, env := range secrets {
		pairs = append(pairs, fmt.Sprintf("%q:%q", entry, env))
	}
	writeRawConfig(t, path, fmt.Sprintf(`{"sections":{"default":{"key-store":%q,"secrets":{%s}}}}`, store, strings.Join(pairs, ",")))
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestMigrate1_InvertsSecretsAndSetsVersion(t *testing.T) {
	sb := newSandbox(t)
	value := secret(t, "E2E_REG_PW")
	store := sb.MakeStore("store.kdbx", map[string]string{"GITHUB_TOKEN": value})
	writeLegacyConfig(t, sb.ConfigPath(), store, map[string]string{"GITHUB_TOKEN": "GH_TOKEN"})

	expectOK(t, sb.RunNoPassword("migrate"))

	cfg, err := harness.LoadConfig(sb.ConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Version != 1 {
		t.Errorf("version: got %d, want 1", cfg.Version)
	}
	if got := cfg.Sections["default"].Secrets["GH_TOKEN"]; got != "GITHUB_TOKEN" {
		t.Errorf("secrets after migration: %+v", cfg.Sections["default"].Secrets)
	}
}

func TestMigrate2_MigratedConfigInjectsSecret(t *testing.T) {
	sb := newSandbox(t)
	value := secret(t, "E2E_REG_PW")
	store := sb.MakeStore("store.kdbx", map[string]string{"GITHUB_TOKEN": value})
	writeLegacyConfig(t, sb.ConfigPath(), store, map[string]string{"GITHUB_TOKEN": "GH_TOKEN"})

	expectOK(t, sb.RunNoPassword("migrate"))

	r := sb.Run("--", "sh", "-c", `printf %s "$GH_TOKEN"`)
	expectOK(t, r)
	if r.Stdout != value {
		t.Errorf("stdout: got %q, want %q", r.Stdout, value)
	}
}

func TestMigrate3_LegacyConfigFailsBeforeMigration(t *testing.T) {
	sb := newSandbox(t)
	store := sb.MakeStore("store.kdbx", map[string]string{"GITHUB_TOKEN": "x"})
	writeLegacyConfig(t, sb.ConfigPath(), store, map[string]string{"GITHUB_TOKEN": "GH_TOKEN"})

	expectFail(t, sb.Run("--", "sh", "-c", "true"))
	expectOK(t, sb.RunNoPassword("migrate"))
	expectOK(t, sb.Run("--", "sh", "-c", "true"))
}

func TestMigrate4_AlreadyMigratedIsNoop(t *testing.T) {
	sb := newSandbox(t)
	store := storeWith(sb, map[string]string{"GITHUB_TOKEN": "x"}, map[string]harness.Section{
		"default": {Secrets: map[string]string{"GH_TOKEN": "GITHUB_TOKEN"}},
	})
	before := readFile(t, sb.ConfigPath())

	r := sb.RunNoPassword("migrate")
	expectOK(t, r)
	if !strings.Contains(r.Stdout, "already at version 1") {
		t.Errorf("expected the already-migrated message:\n%s", r.Stdout)
	}
	if after := readFile(t, sb.ConfigPath()); after != before {
		t.Errorf("config changed for %s:\n%s", store, after)
	}
}

func TestMigrate5_EnvCollisionLeavesFileUntouched(t *testing.T) {
	sb := newSandbox(t)
	store := sb.MakeStore("store.kdbx", map[string]string{"A": "x", "B": "y"})
	writeLegacyConfig(t, sb.ConfigPath(), store, map[string]string{"A": "SAME_ENV", "B": "SAME_ENV"})
	before := readFile(t, sb.ConfigPath())

	r := sb.RunNoPassword("migrate")
	expectFail(t, r)
	if !strings.Contains(r.Stderr, "SAME_ENV") {
		t.Errorf("expected the colliding variable in stderr:\n%s", r.Stderr)
	}
	if after := readFile(t, sb.ConfigPath()); after != before {
		t.Errorf("config was modified:\n%s", after)
	}
}

func TestMigrate6_FromMismatchIsError(t *testing.T) {
	sb := newSandbox(t)
	store := sb.MakeStore("store.kdbx", map[string]string{"GITHUB_TOKEN": "x"})
	writeLegacyConfig(t, sb.ConfigPath(), store, map[string]string{"GITHUB_TOKEN": "GH_TOKEN"})
	before := readFile(t, sb.ConfigPath())

	r := sb.RunNoPassword("migrate", "--from", "1")
	expectFail(t, r)
	if after := readFile(t, sb.ConfigPath()); after != before {
		t.Errorf("config was modified:\n%s", after)
	}
}

func TestMigrate7_ExplicitConfigPath(t *testing.T) {
	sb := newSandbox(t)
	store := sb.MakeStore("store.kdbx", map[string]string{"GITHUB_TOKEN": "x"})
	custom := sb.Dir + "/custom_config"
	writeLegacyConfig(t, custom, store, map[string]string{"GITHUB_TOKEN": "GH_TOKEN"})

	expectOK(t, sb.RunNoPassword("migrate", "--config", custom, "--from", "0", "--to", "1"))

	cfg, err := harness.LoadConfig(custom)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Version != 1 || cfg.Sections["default"].Secrets["GH_TOKEN"] != "GITHUB_TOKEN" {
		t.Errorf("migrated config: %+v", cfg)
	}
}
