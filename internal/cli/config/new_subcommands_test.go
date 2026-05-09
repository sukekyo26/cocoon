//nolint:testpackage // exercises unexported Run helpers together with golden_test.go.
package configcli

import (
	"path/filepath"
	"testing"
)

func TestCmdGet(t *testing.T) {
	t.Parallel()
	fixture := filepath.Join(fixtureRoot, "snapshot.workspace.toml")
	cases := []struct {
		field string
		want  string
	}{
		{"service-name", "snapshot-test\n"},
		{"username", "testuser\n"},
		{"os", "ubuntu\n"},
		{"os-version", "24.04\n"},
	}
	for _, tc := range cases {
		t.Run(tc.field, func(t *testing.T) {
			t.Parallel()
			got, stderr, err := runCmd(t, "get", fixture, tc.field)
			if err != nil {
				t.Fatalf("err=%v stderr=%s", err, stderr)
			}
			if got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestCmdGetUnknownField(t *testing.T) {
	t.Parallel()
	fixture := filepath.Join(fixtureRoot, "snapshot.workspace.toml")
	_, _, err := runCmd(t, "get", fixture, "bogus")
	if err == nil {
		t.Fatal("expected error for unknown field")
	}
}

func TestCmdList(t *testing.T) {
	t.Parallel()
	fixture := filepath.Join(fixtureRoot, "snapshot.workspace.toml")
	cases := []struct {
		field string
		want  string
	}{
		{
			"plugins",
			"custom-ps1\nproto\ndocker-cli\naws-cli\naws-sam-cli\ngithub-cli\n" +
				"copilot-cli\nclaude-code\nzig\nuv\nrust\ngo\nlazygit\n",
		},
		{"forward-ports", "3000:3000\n"},
		{"apt-extra", ""},
	}
	for _, tc := range cases {
		t.Run(tc.field, func(t *testing.T) {
			t.Parallel()
			got, stderr, err := runCmd(t, "list", fixture, tc.field)
			if err != nil {
				t.Fatalf("err=%v stderr=%s", err, stderr)
			}
			if got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestCmdVolumes(t *testing.T) {
	t.Parallel()
	fixture := filepath.Join(fixtureRoot, "snapshot.workspace.toml")
	got, stderr, err := runCmd(t, "volumes", fixture)
	if err != nil {
		t.Fatalf("err=%v stderr=%s", err, stderr)
	}
	want := "deno\t/home/${USERNAME}/.deno\n"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestCmdPluginGet(t *testing.T) {
	t.Parallel()
	dir := "../../../internal/plugin/catalog/go"
	cases := []struct {
		field string
		want  string
	}{
		{"id", "go\n"},
		{"name", "Go\n"},
		{"default", "false\n"},
		{"version-capable", "true\n"},
	}
	for _, tc := range cases {
		t.Run(tc.field, func(t *testing.T) {
			t.Parallel()
			got, stderr, err := runCmd(t, "plugin-get", dir, tc.field)
			if err != nil {
				t.Fatalf("err=%v stderr=%s", err, stderr)
			}
			if got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestCmdPluginListAndVolumes(t *testing.T) {
	t.Parallel()
	dir := "../../../internal/plugin/catalog/go"
	gotDirs, _, err := runCmd(t, "plugin-list", dir, "user-dirs")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	// don't assert exact content (might evolve). Just sanity: ends with newline if non-empty.
	if gotDirs != "" && gotDirs[len(gotDirs)-1] != '\n' {
		t.Errorf("plugin-list output missing trailing newline: %q", gotDirs)
	}
	gotVols, _, err := runCmd(t, "plugin-volumes", dir)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if gotVols != "" && gotVols[len(gotVols)-1] != '\n' {
		t.Errorf("plugin-volumes output missing trailing newline: %q", gotVols)
	}
}

func TestCmdPluginsTable(t *testing.T) {
	t.Parallel()
	got, stderr, err := runCmd(t, "plugins-table", "../../../internal/plugin/catalog")
	if err != nil {
		t.Fatalf("err=%v stderr=%s", err, stderr)
	}
	if got == "" {
		t.Fatal("expected non-empty plugins-table output")
	}
	// Each line must have exactly 3 tabs (4 columns) and end in a newline.
	for i, line := range splitLines(got) {
		tabs := 0
		for _, c := range line {
			if c == '\t' {
				tabs++
			}
		}
		if tabs != 3 {
			t.Errorf("line %d: want 3 tabs got %d in %q", i, tabs, line)
		}
	}
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}
