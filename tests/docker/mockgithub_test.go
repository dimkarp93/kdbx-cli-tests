//go:build e2e

package docker

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

const (
	mockGitHubURL = "http://mockgithub:8080"
	ghInstaller   = "/opt/install/github_install.sh"
)

type mockRequest struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Query  string `json:"query"`
	Auth   string `json:"auth"`
}

func resetMockGitHub(t *testing.T) {
	t.Helper()
	resp, err := http.Post(mockGitHubURL+"/__reset", "text/plain", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}

func mockGitHubRequests(t *testing.T) []mockRequest {
	t.Helper()
	resp, err := http.Get(mockGitHubURL + "/__requests")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var records []mockRequest
	if err := json.NewDecoder(resp.Body).Decode(&records); err != nil {
		t.Fatal(err)
	}
	return records
}

func (s *sandbox) recordMockRequests() {
	t := s.T
	t.Cleanup(func() {
		reqs := mockGitHubRequests(t)
		var b strings.Builder
		for _, r := range reqs {
			fmt.Fprintf(&b, "\n      %s %s", r.Method, r.Path)
			if r.Query != "" {
				b.WriteString("?" + r.Query)
			}
			b.WriteString("  auth=" + r.Auth)
		}
		if len(reqs) == 0 {
			b.WriteString(" none")
		}
		s.Transcript.Notef("requests received by mockgithub:%s", b.String())
		if t.Failed() {
			t.Logf("mockgithub requests:%s", b.String())
		}
	})
}
