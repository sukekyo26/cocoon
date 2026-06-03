//nolint:testpackage // exercises the cobra wiring of interactive `cocoon init` end-to-end.
package initcli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The interactive `cocoon init` E2E forces huh's accessible (line-based)
// mode with TERM=dumb, then drives it by swapping os.Stdin / os.Stdout.
// promptForMissing runs a sequence of independent single-field huh.Forms;
// in accessible mode each field is answered from stdin a line at a time.
// huh's built-in fields (Input/Select/MultiSelect/Confirm) wrap stdin in a
// bufio.Scanner that over-reads past their own line, so only the custom
// selectOrInputField (which reads with fmt.Fscanln, line-precise) is safe
// to drive across a multi-line script — every built-in-backed field is
// pre-filled by a flag instead.

// setTermDumb forces huh.NewForm down its accessible path so the prompts
// read stdin line-by-line instead of opening a tty.
func setTermDumb(t *testing.T) {
	t.Helper()
	t.Setenv("TERM", "dumb")
}

// withScriptedStdin points os.Stdin at a pipe preloaded with script and
// restores it via t.Cleanup. The write end is closed after the script so a
// read past the last line observes io.EOF. script must fit the OS pipe
// buffer (~64 KiB); the scripts here are a few bytes. os.Stdin is process-
// global, so callers must not run in parallel.
func withScriptedStdin(t *testing.T, script string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	if _, err := io.WriteString(w, script); err != nil {
		t.Fatalf("write scripted stdin: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close scripted stdin: %v", err)
	}
	orig := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = orig
		_ = r.Close()
	})
}

// captureStdout redirects os.Stdout to a pipe and returns a read func that
// restores os.Stdout and yields everything written to it — huh's accessible
// prompts land on os.Stdout. os.Stdout is process-global, so callers must
// not run in parallel.
func captureStdout(t *testing.T) func() string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	type result struct {
		text string
		err  error
	}
	done := make(chan result, 1)
	go func() {
		var buf bytes.Buffer
		_, copyErr := io.Copy(&buf, r)
		done <- result{text: buf.String(), err: copyErr}
	}()
	// Safety net: restore os.Stdout and unblock the copier even if the read
	// func is never reached (an earlier t.Fatal). A second Close is a no-op.
	t.Cleanup(func() {
		os.Stdout = orig
		_ = w.Close()
	})
	return func() string {
		os.Stdout = orig
		_ = w.Close()
		res := <-done
		_ = r.Close()
		if res.err != nil {
			t.Errorf("capture stdout: drain pipe: %v", res.err)
		}
		return res.text
	}
}

// baseInteractiveArgs flags every built-in-backed field so the only prompts
// that fire interactively are the two selectOrInputField pickers: the image
// version, then the `go` plugin version. --ports must be non-empty or
// PortsSet stays false and the ports huh.Input would prompt. `go` declares a
// single [install.methods] entry, so its method picker is auto-skipped.
var baseInteractiveArgs = []string{
	"--service-name", "e2eapp", "--username", "dev",
	"--image", "ubuntu", "--shell", "bash", "--mount-root", ".",
	"--dir", "workspace",
	"--no-devcontainer", "--no-certificates", "--sudo", "nopasswd",
	"--ports", "3000", "--apt-categories", "text-editors",
	"--alias-bundles", "git", "--plugins", "go",
}

// TestRunInit_InteractiveSelectOrInput drives `cocoon init` through its
// interactive path end-to-end: runInit -> collectAnswers -> promptForMissing
// -> runSingleFieldForm -> huh accessible -> selectOrInputField.RunAccessible
// -> renderWorkspaceToml -> os.WriteFile. The image-version prompt comes
// first, the `go` plugin-version prompt second.
//
//nolint:paralleltest // os.Stdin/os.Stdout/TERM and t.Chdir are process-global.
func TestRunInit_InteractiveSelectOrInput(t *testing.T) {
	// When `go` is unpinned (the user kept LATEST, or stdin EOF'd),
	// writePluginVersions emits the commented [plugins.versions] template —
	// which itself carries an example `# go = "=1.22.5"` line. So the
	// live-pin check looks for a pin line with no leading "#", i.e.
	// `\ngo = "=`, not the bare substring.
	cases := []struct {
		name            string
		script          string
		wantContains    []string
		wantNotContains []string
		wantStdout      []string
	}{
		{
			// "2" -> SupportedImageVersions["ubuntu"][1] = "24.04";
			// "1" -> go-version suggestion 1 = LATEST -> go left unpinned.
			name:   "pick_by_number",
			script: "2\n1\n",
			wantContains: []string{
				`image_version = "24.04"`,
				"[plugins]\nenable = [\n    \"go\",\n]",
				"version constraints for version_capable plugins",
			},
			wantNotContains: []string{"\ngo = \"="},
			wantStdout: []string{
				"Image version",
				"go version",
				"https://github.com/golang/go",
				"Choose by number or type a tag:",
			},
		},
		{
			// Both answers are >2 chars, so tryIndex rejects them as
			// indices and they take the free-text path. Keep them >2
			// chars: a bare "1" would resolve to a suggestion, not a pin.
			name:   "type_free_text",
			script: "24.04-patched\n1.23.4\n",
			wantContains: []string{
				`image_version = "24.04-patched"`,
				"[plugins.versions]\ngo = \"=1.23.4\"\n",
			},
			wantNotContains: []string{"version constraints for version_capable plugins"},
			wantStdout:      nil,
		},
		{
			// "bad/tag" has a slash -> rxImageVersionInput rejects it,
			// RunAccessible prints the format error and re-prompts;
			// "1" then keeps go on LATEST (unpinned).
			name:   "invalid_then_valid",
			script: "bad/tag\n24.04\n1\n",
			wantContains: []string{
				`image_version = "24.04"`,
				"version constraints for version_capable plugins",
			},
			wantNotContains: []string{`image_version = "bad/tag"`, "\ngo = \"="},
			wantStdout:      []string{"must be a plain Docker tag"},
		},
		{
			// A blank first line -> fmt.Fscanln returns n=0 with a
			// non-EOF error -> "empty input not accepted" + re-prompt.
			name:   "empty_then_valid",
			script: "\n26.04\n1\n",
			wantContains: []string{
				`image_version = "26.04"`,
				"version constraints for version_capable plugins",
			},
			wantNotContains: []string{"\ngo = \"="},
			wantStdout:      []string{"empty input not accepted"},
		},
		{
			// Only the image-version answer is scripted; stdin EOFs
			// before the go-version field reads. selectOrInputField
			// returns a wrapped io.EOF, but huh.Form.runAccessible
			// discards every field error and returns nil — so Execute()
			// still succeeds and `go` simply stays unpinned. This pins
			// the observable degradation; the EOF error contract itself
			// is asserted in field_select_or_input_test.go.
			name:   "graceful_eof",
			script: "24.04\n",
			wantContains: []string{
				`image_version = "24.04"`,
				"version constraints for version_capable plugins",
			},
			wantNotContains: []string{"\ngo = \"="},
			wantStdout:      nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pinEnglish(t)
			setTermDumb(t)
			work := t.TempDir()
			t.Chdir(work)
			withScriptedStdin(t, tc.script)
			readStdout := captureStdout(t)

			cmd := NewCommand(io.Discard, io.Discard)
			cmd.SetArgs(baseInteractiveArgs)
			err := cmd.Execute()
			out := readStdout()
			if err != nil {
				t.Fatalf("cocoon init (interactive): %v\n--- prompts ---\n%s", err, out)
			}

			target := filepath.Join(work, "workspace.toml")
			info, statErr := os.Stat(target)
			if statErr != nil {
				t.Fatalf("stat workspace.toml: %v", statErr)
			}
			if perm := info.Mode().Perm(); perm != 0o644 {
				t.Errorf("workspace.toml mode = %#o, want 0644", perm)
			}
			body, readErr := os.ReadFile(target)
			if readErr != nil {
				t.Fatalf("read workspace.toml: %v", readErr)
			}
			got := string(body)
			for _, want := range tc.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("workspace.toml missing %q\n--- got ---\n%s", want, got)
				}
			}
			for _, notWant := range tc.wantNotContains {
				if strings.Contains(got, notWant) {
					t.Errorf("workspace.toml unexpectedly contains %q\n--- got ---\n%s", notWant, got)
				}
			}
			for _, want := range tc.wantStdout {
				if !strings.Contains(out, want) {
					t.Errorf("accessible prompts missing %q\n--- prompts ---\n%s", want, out)
				}
			}
		})
	}
}
