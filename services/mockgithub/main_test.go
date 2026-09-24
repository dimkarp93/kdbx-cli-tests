//go:build e2e

package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
)

const installScript = "/opt/install/github_install.sh"

func scriptFunction(t *testing.T, name string) string {
	t.Helper()
	out, err := exec.Command("sed", "-n", "/^"+name+"() {/,/^}/p", installScript).Output()
	if err != nil || len(out) == 0 {
		t.Fatalf("cannot extract %s from %s: %v", name, installScript, err)
	}
	return string(out)
}

func TestReleaseJSONMatchesInstallerParsing(t *testing.T) {
	srv, err := newServer("http://mock.test", "tok")
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/api/v3/repos/acme/private-tool/releases/tags/v1.0.0", nil)
	req.Header.Set("Authorization", "token tok")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d: %s", resp.StatusCode, body)
	}

	script := "OS=linux\nARCH=" + archName() + "\n" +
		scriptFunction(t, "discover_bin") + scriptFunction(t, "asset_api_url") +
		"RELEASE_JSON=$(cat)\ndiscover_bin \"$RELEASE_JSON\"\nasset_api_url \"$1\"\nasset_api_url SHA256SUMS\n"
	cmd := exec.Command("sh", "-c", script, "sh", "hello-linux-"+archName()+".tar.gz")
	cmd.Stdin = strings.NewReader(string(body))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("installer functions failed: %v\n%s", err, out)
	}
	want := "hello\n" +
		"http://mock.test/api/v3/repos/acme/private-tool/releases/assets/1\n" +
		"http://mock.test/api/v3/repos/acme/private-tool/releases/assets/2\n"
	if string(out) != want {
		t.Errorf("installer parsing:\ngot:\n%s\nwant:\n%s", out, want)
	}
}

func TestUnauthorizedRequestsGet404(t *testing.T) {
	srv, _ := newServer("http://mock.test", "tok")
	ts := httptest.NewServer(srv)
	defer ts.Close()
	for _, auth := range []string{"", "token wrong"} {
		req, _ := http.NewRequest("GET", ts.URL+"/api/v3/repos/acme/private-tool/releases/latest", nil)
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 404 {
			t.Errorf("auth %q: status %d, want 404", auth, resp.StatusCode)
		}
	}
}
