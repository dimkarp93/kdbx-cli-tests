package harness

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	BinaryEnv  = "KDBX_CLI_BIN"
	VersionEnv = "KDBX_CLI_VERSION"
)

func CheckBinary() (string, string, error) {
	bin := os.Getenv(BinaryEnv)
	if bin == "" {
		return "", "", fmt.Errorf("%s is not set", BinaryEnv)
	}
	out, err := exec.Command(bin, "--version").Output()
	if err != nil {
		return "", "", fmt.Errorf("%s --version: %w", bin, err)
	}
	version := strings.TrimSpace(string(out))
	if want := os.Getenv(VersionEnv); want != "" && version != want {
		return "", "", fmt.Errorf("%s reports version %s, expected %s from versions.txt", bin, version, want)
	}
	if _, err := exec.LookPath("keepassxc-cli"); err != nil {
		return "", "", fmt.Errorf("keepassxc-cli is required")
	}
	return bin, version, nil
}

func WriteSummaryHeader(suite, version string) {
	dir := os.Getenv(TranscriptDirEnv)
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(dir, "SUMMARY.txt"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "=== %s: kdbx-cli %s\n", suite, version)
}
