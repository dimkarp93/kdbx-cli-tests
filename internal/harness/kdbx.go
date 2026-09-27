package harness

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type Template struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type Section struct {
	KeyStore      string            `json:"key-store,omitempty"`
	Secrets       map[string]string `json:"secrets,omitempty"`
	Stdin         []string          `json:"stdin,omitempty"`
	StdinKeepOpen bool              `json:"stdin-keep-open,omitempty"`
	Files         []string          `json:"files,omitempty"`
	Templates     []Template        `json:"templates,omitempty"`
	Askpass       string            `json:"askpass,omitempty"`
}

type Config struct {
	Sections map[string]Section `json:"sections"`
}

func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("invalid config %s: %w", path, err)
	}
	return c, nil
}

func KeepassRun(stdin string, args ...string) (string, error) {
	cmd := exec.Command("keepassxc-cli", args...)
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.Output()
	return string(out), err
}

type kpString struct {
	Key   string `xml:"Key"`
	Value string `xml:"Value"`
}

type kpEntry struct {
	Strings []kpString `xml:"String"`
}

type kpGroup struct {
	Entries []kpEntry `xml:"Entry"`
	Groups  []kpGroup `xml:"Group"`
}

type kpFile struct {
	Root struct {
		Group kpGroup `xml:"Group"`
	} `xml:"Root"`
}

func ExportTitles(xmlData string) (map[string]bool, error) {
	var f kpFile
	if err := xml.Unmarshal([]byte(xmlData), &f); err != nil {
		return nil, err
	}
	titles := map[string]bool{}
	var walk func(g kpGroup)
	walk = func(g kpGroup) {
		for _, e := range g.Entries {
			for _, s := range e.Strings {
				if s.Key == "Title" && s.Value != "" {
					titles[s.Value] = true
				}
			}
		}
		for _, child := range g.Groups {
			walk(child)
		}
	}
	walk(f.Root.Group)
	return titles, nil
}
