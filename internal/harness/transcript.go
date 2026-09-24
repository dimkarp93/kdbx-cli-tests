package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

const TranscriptDirEnv = "KDBX_CLI_E2E_TRANSCRIPT_DIR"

const minMaskedLength = 8

type Transcript struct {
	t       *testing.T
	dir     string
	base    string
	start   time.Time
	mu      sync.Mutex
	f       *os.File
	seq     int
	casts   int
	secrets map[string]string
}

func OpenTranscript(t *testing.T) *Transcript {
	t.Helper()
	dir := os.Getenv(TranscriptDirEnv)
	if dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	base := SafeName(t.Name())
	f, err := os.Create(filepath.Join(dir, base+".log"))
	if err != nil {
		t.Fatal(err)
	}
	tr := &Transcript{t: t, dir: dir, base: base, start: time.Now(), f: f, secrets: map[string]string{}}
	fmt.Fprintf(f, "=== %s\n=== started %s\n", t.Name(), tr.start.Format(time.RFC3339))
	t.Cleanup(tr.finish)
	return tr
}

func (tr *Transcript) finish() {
	result := "PASS"
	switch {
	case tr.t.Failed():
		result = "FAIL"
	case tr.t.Skipped():
		result = "SKIP"
	}
	elapsed := time.Since(tr.start).Round(time.Millisecond)
	tr.mu.Lock()
	fmt.Fprintf(tr.f, "\n=== RESULT: %s (%s)\n", result, elapsed)
	tr.f.Close()
	tr.mu.Unlock()

	summary, err := os.OpenFile(filepath.Join(tr.dir, "SUMMARY.txt"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer summary.Close()
	fmt.Fprintf(summary, "%-4s  %-8s  %s  (%s.log)\n", result, elapsed, tr.t.Name(), tr.base)
}

func (tr *Transcript) AddSecret(value, label string) {
	if tr == nil || len(value) < minMaskedLength {
		return
	}
	tr.mu.Lock()
	defer tr.mu.Unlock()
	if _, ok := tr.secrets[value]; !ok {
		tr.secrets[value] = label
	}
}

func (tr *Transcript) Mask(s string) string {
	if tr == nil {
		return s
	}
	tr.mu.Lock()
	values := make([]string, 0, len(tr.secrets))
	for v := range tr.secrets {
		values = append(values, v)
	}
	tr.mu.Unlock()
	sort.Slice(values, func(i, j int) bool { return len(values[i]) > len(values[j]) })
	for _, v := range values {
		s = strings.ReplaceAll(s, v, "<"+tr.secrets[v]+">")
	}
	return s
}

func (tr *Transcript) write(s string) {
	masked := tr.Mask(s)
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.f.WriteString(masked)
}

func (tr *Transcript) stamp() string {
	return fmt.Sprintf("+%6.3fs", time.Since(tr.start).Seconds())
}

func (tr *Transcript) Notef(format string, args ...any) {
	if tr == nil {
		return
	}
	tr.write(fmt.Sprintf("\n%s note: %s\n", tr.stamp(), fmt.Sprintf(format, args...)))
}

func indentBlock(label, body string) string {
	if body == "" {
		return "    " + label + ": (empty)\n"
	}
	var b strings.Builder
	b.WriteString("    " + label + ":\n")
	for _, line := range strings.SplitAfter(body, "\n") {
		if line == "" {
			continue
		}
		b.WriteString("      | " + strings.TrimSuffix(line, "\n") + "\n")
	}
	if !strings.HasSuffix(body, "\n") {
		b.WriteString("      (no trailing newline)\n")
	}
	return b.String()
}

func shellQuote(s string) string {
	if s != "" && !strings.ContainsAny(s, " \t\n'\"\\$`&|;<>()*?[]{}#~!") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func commandLine(argv []string) string {
	quoted := make([]string, len(argv))
	for i, a := range argv {
		quoted[i] = shellQuote(a)
	}
	return strings.Join(quoted, " ")
}

func envDiff(env []string) []string {
	base := map[string]bool{}
	for _, kv := range os.Environ() {
		base[kv] = true
	}
	last := map[string]string{}
	var order []string
	for _, kv := range env {
		k, _, _ := strings.Cut(kv, "=")
		if _, seen := last[k]; !seen {
			order = append(order, k)
		}
		last[k] = kv
	}
	var out []string
	for _, k := range order {
		if !base[last[k]] {
			out = append(out, last[k])
		}
	}
	return out
}

func (tr *Transcript) Command(argv, env []string, stdin string, r Result, elapsed time.Duration, note string) {
	if tr == nil {
		return
	}
	tr.mu.Lock()
	tr.seq++
	seq := tr.seq
	tr.mu.Unlock()

	var b strings.Builder
	startedAt := time.Since(tr.start) - elapsed
	fmt.Fprintf(&b, "\n+%6.3fs [#%d] $ %s\n", startedAt.Seconds(), seq, commandLine(argv))
	if extra := envDiff(env); len(extra) > 0 {
		b.WriteString("    env:\n")
		for _, kv := range extra {
			b.WriteString("      " + kv + "\n")
		}
	}
	if stdin != "" {
		b.WriteString(indentBlock("stdin", stdin))
	}
	fmt.Fprintf(&b, "    exit: %d (%s)%s\n", r.ExitCode, elapsed.Round(time.Millisecond), note)
	b.WriteString(indentBlock("stdout", r.Stdout))
	b.WriteString(indentBlock("stderr", r.Stderr))
	tr.write(b.String())
}

var ansiSequence = regexp.MustCompile(`\x1b(\[[0-9;?]*[ -/]*[@-~]|\][^\x07\x1b]*(\x07|\x1b\\)|[@-Z\\-_])`)

func StripANSI(s string) string {
	return strings.ReplaceAll(ansiSequence.ReplaceAllString(s, ""), "\r", "")
}

type Cast struct {
	tr    *Transcript
	path  string
	argv  []string
	start time.Time
	mu    sync.Mutex
	f     *os.File
	enc   *json.Encoder
	out   strings.Builder
	input []string
}

func (tr *Transcript) NewCast(argv []string, width, height int) *Cast {
	if tr == nil {
		return nil
	}
	tr.mu.Lock()
	tr.casts++
	name := tr.base + ".cast"
	if tr.casts > 1 {
		name = fmt.Sprintf("%s-%d.cast", tr.base, tr.casts)
	}
	tr.mu.Unlock()
	path := filepath.Join(tr.dir, name)
	f, err := os.Create(path)
	if err != nil {
		tr.t.Fatal(err)
	}
	c := &Cast{tr: tr, path: path, argv: argv, start: time.Now(), f: f, enc: json.NewEncoder(f)}
	c.enc.Encode(map[string]any{
		"version":   2,
		"width":     width,
		"height":    height,
		"timestamp": c.start.Unix(),
		"title":     tr.t.Name() + ": " + tr.Mask(commandLine(argv)),
		"env":       map[string]string{"TERM": "xterm-256color", "SHELL": "/bin/sh"},
	})
	tr.write(fmt.Sprintf("\n%s PTY session $ %s\n    recording: %s\n", tr.stamp(), commandLine(argv), name))
	return c
}

func (c *Cast) event(kind, data string) {
	c.enc.Encode([]any{time.Since(c.start).Seconds(), kind, c.tr.Mask(data)})
}

func (c *Cast) Output(data []byte) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.out.Write(data)
	c.event("o", string(data))
}

func (c *Cast) Input(data string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.input = append(c.input, fmt.Sprintf("+%.3fs %q", time.Since(c.start).Seconds(), c.tr.Mask(data)))
	c.event("i", data)
}

func (c *Cast) Close(exitCode int, finished bool) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.f == nil {
		return
	}
	c.f.Close()
	c.f = nil
	var b strings.Builder
	if len(c.input) > 0 {
		b.WriteString("    typed into the terminal:\n")
		for _, in := range c.input {
			b.WriteString("      " + in + "\n")
		}
	}
	if finished {
		fmt.Fprintf(&b, "    exit: %d after %s\n", exitCode, time.Since(c.start).Round(time.Millisecond))
	} else {
		fmt.Fprintf(&b, "    killed by the test after %s\n", time.Since(c.start).Round(time.Millisecond))
	}
	b.WriteString(indentBlock("screen (escape sequences stripped)", StripANSI(c.out.String())))
	c.tr.write(b.String())
}
