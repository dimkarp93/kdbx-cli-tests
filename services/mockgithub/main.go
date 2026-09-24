package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	owner   = "acme"
	repo    = "private-tool"
	tag     = "v1.0.0"
	helloSh = "#!/bin/sh\ncase \"${1:-}\" in\n--version) echo 1.0.0 ;;\n*) echo hello-ok ;;\nesac\n"
)

type asset struct {
	URL                string `json:"url"`
	ID                 int    `json:"id"`
	Name               string `json:"name"`
	ContentType        string `json:"content_type"`
	Size               int    `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
	data               []byte
}

type release struct {
	ID      int      `json:"id"`
	TagName string   `json:"tag_name"`
	Name    string   `json:"name"`
	Draft   bool     `json:"draft"`
	Assets  []*asset `json:"assets"`
}

type requestRecord struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Query  string `json:"query"`
	Auth   string `json:"auth"`
}

type server struct {
	token   string
	release release
	mu      sync.Mutex
	log     []requestRecord
}

func archName() string {
	return runtime.GOARCH
}

func buildArchive(name string, body []byte) ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{Name: name, Mode: 0755, Size: int64(len(body)), ModTime: time.Unix(0, 0), Typeflag: tar.TypeReg}
	if err := tw.WriteHeader(hdr); err != nil {
		return nil, err
	}
	if _, err := tw.Write(body); err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func newServer(baseURL, token string) (*server, error) {
	archiveName := fmt.Sprintf("hello-linux-%s.tar.gz", archName())
	archive, err := buildArchive("hello", []byte(helloSh))
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(archive)
	sums := []byte(hex.EncodeToString(sum[:]) + "  " + archiveName + "\n")

	apiBase := fmt.Sprintf("%s/api/v3/repos/%s/%s/releases/assets/", baseURL, owner, repo)
	downloadBase := fmt.Sprintf("%s/%s/%s/releases/download/%s/", baseURL, owner, repo, tag)
	mk := func(id int, name, ctype string, data []byte) *asset {
		return &asset{
			URL:                fmt.Sprintf("%s%d", apiBase, id),
			ID:                 id,
			Name:               name,
			ContentType:        ctype,
			Size:               len(data),
			BrowserDownloadURL: downloadBase + name,
			data:               data,
		}
	}
	return &server{
		token: token,
		release: release{
			ID:      1,
			TagName: tag,
			Name:    tag,
			Assets: []*asset{
				mk(1, archiveName, "application/gzip", archive),
				mk(2, "SHA256SUMS", "text/plain", sums),
			},
		},
	}, nil
}

func (s *server) authStatus(r *http.Request) string {
	h := r.Header.Get("Authorization")
	switch {
	case h == "":
		return "none"
	case h == "token "+s.token || h == "Bearer "+s.token:
		return "valid"
	default:
		return "invalid"
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	data, _ := json.MarshalIndent(v, "", "  ")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(append(data, '\n'))
}

func notFound(w http.ResponseWriter) {
	writeJSON(w, http.StatusNotFound, map[string]string{"message": "Not Found"})
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/__health":
		w.Write([]byte("ok\n"))
		return
	case "/__requests":
		s.mu.Lock()
		records := append([]requestRecord{}, s.log...)
		s.mu.Unlock()
		writeJSON(w, http.StatusOK, records)
		return
	case "/__reset":
		s.mu.Lock()
		s.log = nil
		s.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
		return
	case "/__echo":
		keys := make([]string, 0, len(r.Header))
		for k := range r.Header {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(w, "%s: %s\n", k, strings.Join(r.Header[k], ", "))
		}
		return
	}

	auth := s.authStatus(r)
	s.mu.Lock()
	s.log = append(s.log, requestRecord{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Auth: auth})
	s.mu.Unlock()

	if auth != "valid" || r.Method != http.MethodGet {
		notFound(w)
		return
	}

	prefix := fmt.Sprintf("/api/v3/repos/%s/%s/releases", owner, repo)
	rest, ok := strings.CutPrefix(r.URL.Path, prefix)
	if !ok {
		notFound(w)
		return
	}
	switch {
	case rest == "" || rest == "/":
		writeJSON(w, http.StatusOK, []release{s.release})
	case rest == "/latest", rest == "/tags/"+tag:
		writeJSON(w, http.StatusOK, s.release)
	case strings.HasPrefix(rest, "/assets/"):
		id := strings.TrimPrefix(rest, "/assets/")
		for _, a := range s.release.Assets {
			if fmt.Sprint(a.ID) != id {
				continue
			}
			if r.Header.Get("Accept") == "application/octet-stream" {
				w.Header().Set("Content-Type", "application/octet-stream")
				w.Write(a.data)
			} else {
				writeJSON(w, http.StatusOK, a)
			}
			return
		}
		notFound(w)
	default:
		notFound(w)
	}
}

func main() {
	token := os.Getenv("E2E_GH_TOKEN")
	if token == "" {
		log.Fatal("E2E_GH_TOKEN is required")
	}
	base := os.Getenv("MOCK_BASE_URL")
	if base == "" {
		base = "http://mockgithub:8080"
	}
	srv, err := newServer(base, token)
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(http.ListenAndServe(":8080", srv))
}
