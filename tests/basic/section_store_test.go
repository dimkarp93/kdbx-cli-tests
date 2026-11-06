//go:build e2e

package basic

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimkarp93/kdbx-cli-tests/internal/harness"
)

const sectionStoreScript = `printf "%s|%s" "$OWN_TOKEN" "$COMMON_TOKEN"`

type twoStores struct {
	common string
	own    string
}

func makeTwoStores(sb *harness.Sandbox) twoStores {
	sb.T.Helper()

	return twoStores{
		common: sb.MakeStore("common.kdbx", map[string]string{
			"common_token": "common-value",
			"own_token":    "own-value-in-common",
		}),
		own: sb.MakeStore("own.kdbx", map[string]string{
			"own_token": "own-value",
		}),
	}
}

func defaultSection(keyStore string) harness.Section {
	return harness.Section{
		KeyStore: keyStore,
		Secrets:  map[string]string{"COMMON_TOKEN": "common_token"},
	}
}

func defaultSectionWithChannels(keyStore string) harness.Section {
	section := defaultSection(keyStore)
	section.Stdin = []string{"common_stdin"}
	section.Files = []string{"common_file"}
	section.Askpass = "common_askpass"

	return section
}

func ownSection(keyStore string) harness.Section {
	return harness.Section{
		KeyStore: keyStore,
		Secrets:  map[string]string{"OWN_TOKEN": "own_token"},
	}
}

func TestSectionStore1_OtherStoreGetsOnlyOwnSecrets(t *testing.T) {
	sb := newSandbox(t)
	stores := makeTwoStores(sb)
	sb.WriteConfig(map[string]harness.Section{
		"default": defaultSection(stores.common),
		"sh":      ownSection(stores.own),
	})

	r := sb.Run("--", "sh", "-c", sectionStoreScript)

	t.Run("exits-zero", func(t *testing.T) {
		assert.Equal(t, 0, r.ExitCode, r.Stderr)
	})
	t.Run("delivers-only-own-secret", func(t *testing.T) {
		assert.Equal(t, "own-value|", r.Stdout)
	})
}

func TestSectionStore2_SameStoreKeepsDefault(t *testing.T) {
	sb := newSandbox(t)
	stores := makeTwoStores(sb)
	sb.WriteConfig(map[string]harness.Section{
		"default": defaultSection(stores.common),
		"sh":      ownSection(stores.common),
	})

	r := sb.Run("--", "sh", "-c", sectionStoreScript)

	t.Run("exits-zero", func(t *testing.T) {
		assert.Equal(t, 0, r.ExitCode, r.Stderr)
	})
	t.Run("delivers-both-secrets", func(t *testing.T) {
		assert.Equal(t, "own-value-in-common|common-value", r.Stdout)
	})
}

func TestSectionStore3_DryRunShowsDefaultNotApplied(t *testing.T) {
	sb := newSandbox(t)
	stores := makeTwoStores(sb)
	sb.WriteConfig(map[string]harness.Section{
		"default": defaultSectionWithChannels(stores.common),
		"sh":      ownSection(stores.own),
	})

	r := sb.RunNoPassword("--dry-run", "--", "sh", "-c", "true")

	t.Run("exits-zero", func(t *testing.T) {
		assert.Equal(t, 0, r.ExitCode, r.Stderr)
	})
	t.Run("names-own-store", func(t *testing.T) {
		assert.Contains(t, r.Stdout, stores.own)
	})
	t.Run("explains-section", func(t *testing.T) {
		assert.Contains(t, r.Stdout, `"default" not applied`)
	})
	t.Run("omits-default-titles", func(t *testing.T) {
		assert.NotContains(t, r.Stdout, "common_")
	})
}

func TestSectionStore4_CheckKeepsDefaultTitlesOutOfOtherStore(t *testing.T) {
	sb := newSandbox(t)
	stores := makeTwoStores(sb)
	sb.WriteConfig(map[string]harness.Section{
		"default": defaultSection(stores.common),
		"sh":      ownSection(stores.own),
	})

	r := sb.Run("check", "-y")

	t.Run("exits-zero", func(t *testing.T) {
		assert.Equal(t, 0, r.ExitCode, r.Stderr)
	})
	t.Run("reports-nothing-missing", func(t *testing.T) {
		assert.Contains(t, r.Stdout, "All secrets are present")
	})
	t.Run("adds-no-default-title", func(t *testing.T) {
		assert.NotContains(t, sb.StoreTitles(stores.own), "common_token")
	})
}
