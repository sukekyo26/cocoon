package envfile_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/cocoon/internal/envfile"
)

func TestReadOr(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("FOO=bar\nBAZ=\"quoted value\"\nEMPTY=\n"), 0o600); err != nil {
		t.Fatalf("write env: %v", err)
	}
	if got := envfile.ReadOr(path, "FOO", "fallback"); got != "bar" {
		t.Fatalf("FOO: got %q", got)
	}
	if got := envfile.ReadOr(path, "BAZ", "fallback"); got != "quoted value" {
		t.Fatalf("BAZ: got %q", got)
	}
	if got := envfile.ReadOr(path, "MISSING", "fallback"); got != "fallback" {
		t.Fatalf("MISSING: got %q", got)
	}
	if got := envfile.ReadOr(filepath.Join(dir, "nonexistent"), "FOO", "fb"); got != "fb" {
		t.Fatalf("nonexistent: got %q", got)
	}
}

func TestConfirmYN(t *testing.T) {
	t.Parallel()
	cases := []struct {
		input string
		want  bool
	}{
		{"y\n", true},
		{"Y\n", true},
		{"yes\n", false},
		{"n\n", false},
		{"\n", false},
		{"", false},
	}
	for _, tc := range cases {
		out := &bytes.Buffer{}
		got, err := envfile.ConfirmYN(strings.NewReader(tc.input), out, "prompt> ")
		if err != nil {
			t.Errorf("input %q: unexpected error %v", tc.input, err)
			continue
		}
		if got != tc.want {
			t.Errorf("input %q: got %v want %v", tc.input, got, tc.want)
		}
		if !strings.Contains(out.String(), "prompt> ") {
			t.Errorf("input %q: prompt not written", tc.input)
		}
	}
}
