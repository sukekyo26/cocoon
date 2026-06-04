package cli_test

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/cocoon/internal/cli"
)

//nolint:gochecknoglobals // test flag for golden regeneration.
var updateHelpGolden = flag.Bool("update-golden", false, "rewrite testdata/help/*.txt from current --help output")

// helpGoldenCases enumerate every `--help` surface we want pinned. Use
// underscore-delimited file basenames for nested subcommands so the
// testdata/ tree stays a flat alphabetical list.
//
//nolint:gochecknoglobals // table-driven test fixture, read-only.
var helpGoldenCases = []struct {
	name string
	args []string
}{
	{"root", []string{"--help"}},
	{"init", []string{"init", "--help"}},
	{"gen", []string{"gen", "--help"}},
	{"gen_workspace", []string{"gen", "workspace", "--help"}},
	{"lock", []string{"lock", "--help"}},
	{"plugin", []string{"plugin", "--help"}},
	{"plugin_list", []string{"plugin", "list", "--help"}},
	{"plugin_show", []string{"plugin", "show", "--help"}},
	{"plugin_pin", []string{"plugin", "pin", "--help"}},
	{"plugin_scaffold", []string{"plugin", "scaffold", "--help"}},
	{"self_update", []string{"self-update", "--help"}},
	{"version", []string{"version", "--help"}},
	{"completion", []string{"completion", "--help"}},
	{"completion_bash", []string{"completion", "bash", "--help"}},
	{"help", []string{"help", "--help"}},
}

//nolint:paralleltest // t.Setenv pins locale; goroutine-safe parallelism is not the goal.
func TestHelpGolden(t *testing.T) {
	for _, lang := range []string{"en", "ja"} {
		t.Run(lang, func(t *testing.T) {
			for _, tc := range helpGoldenCases {
				t.Run(tc.name, func(t *testing.T) {
					pinLocale(t, lang)
					var stdout, stderr bytes.Buffer
					app := cli.New("0.0.0-test", &stdout, &stderr)
					if err := app.Execute(tc.args); err != nil {
						t.Fatalf("Execute(%v) returned err: %v", tc.args, err)
					}
					goldenPath := filepath.Join("testdata", "help", tc.name+"."+lang+".txt")
					compareOrUpdate(t, goldenPath, stdout.Bytes())
				})
			}
		})
	}
}

// pinLocale clears every locale env var that internal/i18n.Detect()
// inspects and re-sets only WORKSPACE_LANG, so the test stays
// deterministic regardless of host locale.
func pinLocale(t *testing.T, lang string) {
	t.Helper()
	for _, k := range []string{"WORKSPACE_LANG", "LC_ALL", "LC_MESSAGES", "LANG"} {
		t.Setenv(k, "")
	}
	t.Setenv("WORKSPACE_LANG", lang)
}

func compareOrUpdate(t *testing.T, goldenPath string, got []byte) {
	t.Helper()
	if *updateHelpGolden {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(goldenPath), err)
		}
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil { //nolint:gosec // testdata fixture, user-readable.
			t.Fatalf("write %s: %v", goldenPath, err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath) //nolint:gosec // goldenPath is under testdata/.
	if err != nil {
		t.Fatalf("read %s: %v (run `go test -run TestHelpGolden -update-golden` to create)", goldenPath, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("golden mismatch for %s\nwant:\n%s\ngot:\n%s\n(run `go test -run TestHelpGolden -update-golden` to refresh)",
			goldenPath, indent(string(want)), indent(string(got)))
	}
}

// indent prefixes every line with two spaces so the diff is readable
// inside `t.Fatalf` output.
func indent(s string) string {
	if s == "" {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		lines[i] = "  " + ln
	}
	return strings.Join(lines, "\n")
}
