package doctor

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileExists(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	file := filepath.Join(dir, "f")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !fileExists(file) {
		t.Errorf("fileExists(file) = false, want true")
	}
	if fileExists(dir) {
		t.Errorf("fileExists(dir) = true, want false (must reject directories)")
	}
	if fileExists(filepath.Join(dir, "missing")) {
		t.Errorf("fileExists(missing) = true, want false")
	}
}

func TestDirExists(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if !dirExists(dir) {
		t.Errorf("dirExists(dir) = false, want true")
	}
	file := filepath.Join(dir, "f")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if dirExists(file) {
		t.Errorf("dirExists(file) = true, want false")
	}
	if dirExists(filepath.Join(dir, "missing")) {
		t.Errorf("dirExists(missing) = true, want false")
	}
}

func TestFirstLine(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"hello", "hello"},
		{"hello\nworld", "hello"},
		{"\nfirst", ""},
		{"only\n", "only"},
	}
	for _, c := range cases {
		if got := firstLine(c.in); got != c.want {
			t.Errorf("firstLine(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestIsReadWritable(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	rw := filepath.Join(dir, "rw")
	if err := os.WriteFile(rw, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !isReadWritable(rw) {
		t.Errorf("isReadWritable(rw 0o600) = false, want true")
	}
	if isReadWritable(filepath.Join(dir, "missing")) {
		t.Errorf("isReadWritable(missing) = true, want false")
	}
	// Read-only file: open with O_WRONLY should fail.
	if os.Getuid() != 0 { // root bypasses mode bits
		ro := filepath.Join(dir, "ro")
		if err := os.WriteFile(ro, []byte("x"), 0o400); err != nil {
			t.Fatal(err)
		}
		if isReadWritable(ro) {
			t.Errorf("isReadWritable(ro 0o400) = true, want false")
		}
	}
}

func TestReportSidecar_HealthyAndUnhealthy(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		line   string
		expect string
	}{
		{"empty_line_warns", "", "[⚠]"},
		{"healthy", "id\trunning\thealthy", "[✓]"},
		{"starting", "id\tstarting\tstarting", "[⚠]"},
		{"unhealthy", "id\trunning\tunhealthy", "[✗]"},
		{"running_no_health", "id\trunning\t<no value>", "[✓]"},
		{"stopped", "id\texited\t<no value>", "[⚠]"},
		{"missing_health_field", "id\trunning", "[✓]"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			r := NewReporter(&buf)
			reportSidecar(r, "svc", tc.line)
			if !strings.Contains(buf.String(), tc.expect) {
				t.Errorf("missing %q in:\n%s", tc.expect, buf.String())
			}
		})
	}
}

func TestCheckPluginTOMLs_Variants(t *testing.T) {
	t.Parallel()
	t.Run("missing_dir_warns", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		r := NewReporter(&buf)
		checkPluginTOMLs(r, filepath.Join(t.TempDir(), "no-such"))
		if !strings.Contains(buf.String(), "[⚠]") {
			t.Errorf("expected warning for missing dir:\n%s", buf.String())
		}
	})
	t.Run("invalid_toml_fails", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		bad := filepath.Join(dir, "bogus")
		if err := os.MkdirAll(bad, 0o755); err != nil {
			t.Fatal(err)
		}
		// Plugin TOML missing required [metadata] / [install].
		if err := os.WriteFile(filepath.Join(bad, "plugin.toml"), []byte("# empty\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		r := NewReporter(&buf)
		checkPluginTOMLs(r, dir)
		if !strings.Contains(buf.String(), "[✗]") {
			t.Errorf("expected failure for invalid TOML:\n%s", buf.String())
		}
	})
	t.Run("non_dir_entries_skipped", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		// Stray file (not a plugin dir).
		if err := os.WriteFile(filepath.Join(dir, "stray.txt"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		r := NewReporter(&buf)
		checkPluginTOMLs(r, dir)
		// All non-dir entries skipped, but no plugin.toml triggers continue;
		// final r.Pass fires.
		if !strings.Contains(buf.String(), "[✓]") {
			t.Errorf("expected pass with no plugins:\n%s", buf.String())
		}
	})
}
