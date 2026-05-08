//nolint:testpackage // white-box tests for unexported promptValidated/promptPorts.
package setup

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/logx"
	"github.com/sukekyo26/cocoon/internal/tui"
)

type stubT struct{}

func (stubT) Msg(key string, args ...any) string {
	if len(args) == 0 {
		return key
	}
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = fmt.Sprint(a)
	}
	return key + ":" + strings.Join(parts, ",")
}

type stubSelector struct {
	singleIdx           int
	multiIdx            []int
	gotSingleDefaultIdx int
	singleErr           error
}

func (s *stubSelector) SelectSingle(_ string, _ []string, defaultIdx int) (int, error) {
	s.gotSingleDefaultIdx = defaultIdx
	if s.singleErr != nil {
		return 0, s.singleErr
	}
	return s.singleIdx, nil
}

func (s *stubSelector) SelectMulti(string, []string, []int) ([]int, error) {
	return s.multiIdx, nil
}

// recordingSelector returns successive entries from singleIndices on each
// SelectSingle call. Used when a flow drives multiple pickers in sequence
// (e.g. pickOs followed by pickOsVersion).
type recordingSelector struct {
	singleIndices []int
	multiIdx      []int
	calls         int
}

func (r *recordingSelector) SelectSingle(_ string, _ []string, _ int) (int, error) {
	if r.calls >= len(r.singleIndices) {
		return 0, fmt.Errorf("recordingSelector: SelectSingle call #%d exceeds %d configured indices",
			r.calls+1, len(r.singleIndices))
	}
	idx := r.singleIndices[r.calls]
	r.calls++
	return idx, nil
}

func (r *recordingSelector) SelectMulti(string, []string, []int) ([]int, error) {
	return r.multiIdx, nil
}

// slowReader streams a string one byte at a time. The setup helpers create a
// fresh bufio.Scanner per prompt, so buffering more than one byte per Read
// would let the first prompt's scanner pre-consume the next prompt's input.
type slowReader struct {
	s   string
	pos int
}

func (r *slowReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.s) {
		return 0, io.EOF
	}
	p[0] = r.s[r.pos]
	r.pos++
	return 1, nil
}

func TestPromptValidated_AcceptsFirstValid(t *testing.T) {
	t.Parallel()
	in := strings.NewReader("alpha\n")
	var out bytes.Buffer
	log := logx.New(&out, io.Discard)
	got := promptValidated(in, log, "name? ", func(s string) error {
		if s == "" {
			return fmt.Errorf("empty")
		}
		return nil
	})
	if got != "alpha" {
		t.Errorf("got %q, want \"alpha\"", got)
	}
}

func TestPromptValidated_RetriesOnError(t *testing.T) {
	t.Parallel()
	in := strings.NewReader("\n\nvalid\n")
	var out bytes.Buffer
	log := logx.New(&out, io.Discard)
	got := promptValidated(in, log, "name? ", func(s string) error {
		if s == "" {
			return fmt.Errorf("must not be empty")
		}
		return nil
	})
	if got != "valid" {
		t.Errorf("got %q, want \"valid\"", got)
	}
	if !strings.Contains(out.String(), "must not be empty") {
		t.Errorf("expected error message in output, got %q", out.String())
	}
}

func TestPromptValidated_EOFReturnsEmpty(t *testing.T) {
	t.Parallel()
	in := strings.NewReader("")
	log := logx.New(io.Discard, io.Discard)
	got := promptValidated(in, log, "name? ", func(_ string) error { return nil })
	if got != "" {
		t.Errorf("got %q, want empty on EOF", got)
	}
}

func TestRunInteractive_OsAndVersionSelected(t *testing.T) {
	t.Parallel()
	work := t.TempDir()
	wsPath := filepath.Join(work, "workspace.toml")
	pluginsDir := filepath.Join(work, "plugins") // unused; LoadDir error is swallowed

	// Stdin feeds service name, username and an empty port line (skip).
	// slowReader avoids letting the first prompt's bufio.Scanner pre-consume
	// the input meant for later prompts.
	in := &slowReader{s: "dev\nshogo\n\n"}
	// recordingSelector returns 0 for the OS prompt (ubuntu) and 2 for the
	// version prompt; ubuntu's version list is ["26.04","24.04","22.04"], so
	// index 2 selects 22.04.
	sel := &recordingSelector{singleIndices: []int{0, 2}}
	opts := Options{
		Stdin:    in,
		Stdout:   io.Discard,
		Stderr:   io.Discard,
		Logger:   logx.New(io.Discard, io.Discard),
		Catalog:  stubT{},
		Selector: sel,
	}

	ws, err := runInteractive(opts, wsPath, pluginsDir)
	if err != nil {
		t.Fatalf("runInteractive: %v", err)
	}
	if ws.Container.Os != "ubuntu" {
		t.Errorf("Os = %q, want ubuntu", ws.Container.Os)
	}
	if ws.Container.OsVersion != "22.04" {
		t.Errorf("OsVersion = %q, want 22.04", ws.Container.OsVersion)
	}

	body, err := os.ReadFile(wsPath)
	if err != nil {
		t.Fatalf("read workspace.toml: %v", err)
	}
	for _, want := range []string{`os = "ubuntu"`, `os_version = "22.04"`} {
		if !strings.Contains(string(body), want) {
			t.Errorf("workspace.toml missing %q entry:\n%s", want, body)
		}
	}
}

func TestPickOs_AutoYesPreservesExisting(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	opts := Options{
		AutoYes: true,
		Logger:  logx.New(&buf, io.Discard),
		Catalog: stubT{},
	}
	existing := &partialWS{Container: &partialContainer{Os: "debian", OsVersion: "12"}}
	got, err := pickOs(opts, existing)
	if err != nil {
		t.Fatalf("pickOs: %v", err)
	}
	if got != "debian" {
		t.Errorf("got %q, want debian (preserved from existing)", got)
	}
	if !strings.Contains(buf.String(), "setup_os_preserved") {
		t.Errorf("expected preserved log key, got: %s", buf.String())
	}
}

func TestPickOs_AutoYesUsesDefaultWhenNoExisting(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	opts := Options{
		AutoYes: true,
		Logger:  logx.New(&buf, io.Discard),
		Catalog: stubT{},
	}
	got, err := pickOs(opts, nil)
	if err != nil {
		t.Fatalf("pickOs: %v", err)
	}
	if got != defaultOs {
		t.Errorf("got %q, want %q", got, defaultOs)
	}
	if !strings.Contains(buf.String(), "setup_os_default") {
		t.Errorf("expected default log key, got: %s", buf.String())
	}
}

func TestPickOsVersion_AutoYesPreservesWhenSameOs(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	opts := Options{
		AutoYes: true,
		Logger:  logx.New(&buf, io.Discard),
		Catalog: stubT{},
	}
	existing := &partialWS{Container: &partialContainer{Os: "ubuntu", OsVersion: "22.04"}}
	got, err := pickOsVersion(opts, existing, "ubuntu")
	if err != nil {
		t.Fatalf("pickOsVersion: %v", err)
	}
	if got != "22.04" {
		t.Errorf("got %q, want 22.04 (preserved)", got)
	}
	if !strings.Contains(buf.String(), "setup_os_version_preserved") {
		t.Errorf("expected preserved log key, got: %s", buf.String())
	}
}

func TestPickOsVersion_AutoYesUsesPerOsDefault(t *testing.T) {
	t.Parallel()
	cases := []struct {
		osID string
		want string
	}{
		{"ubuntu", "24.04"},
		{"debian", "12"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.osID, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			opts := Options{
				AutoYes: true,
				Logger:  logx.New(&buf, io.Discard),
				Catalog: stubT{},
			}
			got, err := pickOsVersion(opts, nil, tc.osID)
			if err != nil {
				t.Fatalf("pickOsVersion: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
			if !strings.Contains(buf.String(), "setup_os_version_default") {
				t.Errorf("expected default log key, got: %s", buf.String())
			}
		})
	}
}

func TestPickOsVersion_InteractivePreselectsExistingForSameOs(t *testing.T) {
	t.Parallel()
	sel := &stubSelector{singleIdx: 2}
	opts := Options{
		Logger:   logx.New(io.Discard, io.Discard),
		Catalog:  stubT{},
		Selector: sel,
	}
	// "22.04" is index 2 in SupportedOsVersions["ubuntu"] ["26.04","24.04","22.04"].
	existing := &partialWS{Container: &partialContainer{Os: "ubuntu", OsVersion: "22.04"}}
	if _, err := pickOsVersion(opts, existing, "ubuntu"); err != nil {
		t.Fatalf("pickOsVersion: %v", err)
	}
	if sel.gotSingleDefaultIdx != 2 {
		t.Errorf("defaultIdx passed to selector = %d, want 2 (existing 22.04)", sel.gotSingleDefaultIdx)
	}
}

func TestPickOsVersion_InteractiveFallsBackToPerOsDefault(t *testing.T) {
	t.Parallel()
	for _, osID := range config.SupportedOSes {
		osID := osID
		want := defaultOsVersion[osID]
		defaultIdx := -1
		for i, v := range config.SupportedOsVersions[osID] {
			if v == want {
				defaultIdx = i
				break
			}
		}
		if defaultIdx < 0 {
			t.Fatalf("defaultOsVersion[%s] %q not in SupportedOsVersions", osID, want)
		}

		cases := []struct {
			name     string
			existing *partialWS
		}{
			{"no_existing", nil},
			{"existing_without_container", &partialWS{}},
			{
				"existing_other_os",
				&partialWS{Container: &partialContainer{Os: "ubuntu", OsVersion: "24.04"}},
			},
		}
		for _, tc := range cases {
			tc := tc
			t.Run(osID+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				sel := &stubSelector{singleIdx: 0}
				opts := Options{
					Logger:   logx.New(io.Discard, io.Discard),
					Catalog:  stubT{},
					Selector: sel,
				}
				if _, err := pickOsVersion(opts, tc.existing, osID); err != nil {
					t.Fatalf("pickOsVersion: %v", err)
				}
				// existing_other_os: when osID == ubuntu and existing is also
				// ubuntu, the existing OsVersion is preserved instead of falling
				// back to the per-OS default. Skip that branch.
				if osID == "ubuntu" && tc.name == "existing_other_os" {
					return
				}
				if sel.gotSingleDefaultIdx != defaultIdx {
					t.Errorf("defaultIdx = %d, want %d (defaultOsVersion[%s] %q)",
						sel.gotSingleDefaultIdx, defaultIdx, osID, want)
				}
			})
		}
	}
}

func TestPickOs_TuiCancelMapsToErrCanceled(t *testing.T) {
	t.Parallel()
	sel := &stubSelector{singleErr: tui.ErrCanceled}
	opts := Options{
		Logger:   logx.New(io.Discard, io.Discard),
		Catalog:  stubT{},
		Selector: sel,
	}
	_, err := pickOs(opts, nil)
	if !errors.Is(err, ErrCanceled) {
		t.Errorf("err = %v, want errors.Is(err, ErrCanceled)", err)
	}
}

func TestPickOs_NonCancelErrorIsNotMaskedAsCancel(t *testing.T) {
	t.Parallel()
	boom := errors.New("render failed")
	sel := &stubSelector{singleErr: boom}
	opts := Options{
		Logger:   logx.New(io.Discard, io.Discard),
		Catalog:  stubT{},
		Selector: sel,
	}
	_, err := pickOs(opts, nil)
	if !errors.Is(err, boom) {
		t.Errorf("underlying error not propagated: %v", err)
	}
	if errors.Is(err, ErrCanceled) {
		t.Errorf("non-cancel TUI error must not be reported as ErrCanceled (would map to exit 130)")
	}
}

func TestPromptPorts(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		input   string
		want    []any
		wantLen int
	}{
		{"empty_returns_empty", "\n", []any{}, 0},
		{"single_port", "8080\n", []any{"8080:8080"}, 1},
		{"multiple_ports", "80,443,8080\n", []any{"80:80", "443:443", "8080:8080"}, 3},
		{"with_spaces", "80 ,  443 ,  8080\n", []any{"80:80", "443:443", "8080:8080"}, 3},
		{"retry_on_invalid", "abc\n8080\n", []any{"8080:8080"}, 1},
		{"eof_returns_empty", "", []any{}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var out bytes.Buffer
			log := logx.New(&out, io.Discard)
			got := promptPorts(strings.NewReader(tc.input), log, stubT{})
			if len(got) != tc.wantLen {
				t.Errorf("len = %d, want %d (got %v)", len(got), tc.wantLen, got)
				return
			}
			for i, p := range tc.want {
				if got[i] != p {
					t.Errorf("got[%d] = %v, want %v", i, got[i], p)
				}
			}
		})
	}
}
