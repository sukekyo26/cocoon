package workspace_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/sukekyo26/cocoon/internal/workspace"
)

func TestAvailableDirsExpandsFolderOnly(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	// repo (regular dir with a file)
	mustMkdir(t, filepath.Join(parent, "repo"))
	mustWrite(t, filepath.Join(parent, "repo", "README.md"), "x")
	// group/{a,b} (folder-only, must expand)
	mustMkdir(t, filepath.Join(parent, "group", "a"))
	mustMkdir(t, filepath.Join(parent, "group", "b"))
	// .hidden must be ignored
	mustMkdir(t, filepath.Join(parent, ".hidden"))

	got, err := workspace.AvailableDirs(parent)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"group", "group/a", "group/b", "repo"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestWorkspaceFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "a.code-workspace"), "{}")
	mustWrite(t, filepath.Join(dir, "b.code-workspace"), "{}")
	mustWrite(t, filepath.Join(dir, "ignored.txt"), "x")

	got, err := workspace.ListFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a.code-workspace", "b.code-workspace"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestWorkspaceFilesMissingDir(t *testing.T) {
	t.Parallel()
	got, err := workspace.ListFiles(filepath.Join(t.TempDir(), "no-such"))
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestGenerateAndCurrentFolders(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	settings := filepath.Join(dir, "settings.json")
	mustWrite(t, settings, `{"editor.tabSize": 2}`)
	out := filepath.Join(dir, "ws.code-workspace")

	if err := workspace.Generate(out, settings, []string{"repo", "group/a"}); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Folders []struct {
			Name string `json:"name"`
			Path string `json:"path"`
		} `json:"folders"`
		Settings map[string]any `json:"settings"`
	}
	if uerr := json.Unmarshal(raw, &parsed); uerr != nil {
		t.Fatal(uerr)
	}
	if len(parsed.Folders) != 2 {
		t.Fatalf("expected 2 folders, got %d", len(parsed.Folders))
	}
	if parsed.Folders[0].Path != "../../repo" || parsed.Folders[0].Name != "repo" {
		t.Errorf("bad folder[0]: %+v", parsed.Folders[0])
	}
	if parsed.Folders[1].Path != "../../group/a" || parsed.Folders[1].Name != "a" {
		t.Errorf("bad folder[1]: %+v", parsed.Folders[1])
	}
	v, ok := parsed.Settings["editor.tabSize"].(float64)
	if !ok || v != 2 {
		t.Errorf("settings missing: %v", parsed.Settings)
	}

	current, err := workspace.CurrentFolders(out)
	if err != nil {
		t.Fatal(err)
	}
	wantCurrent := []string{"repo", "group/a"}
	if !reflect.DeepEqual(current, wantCurrent) {
		t.Errorf("got %v, want %v", current, wantCurrent)
	}
}

func TestGenerateValidation(t *testing.T) {
	t.Parallel()
	if err := workspace.Generate("", "x", []string{"a"}); err == nil {
		t.Error("expected error for empty outputFile")
	}
	if err := workspace.Generate("x", "", []string{"a"}); err == nil {
		t.Error("expected error for empty settingsFile")
	}
	if err := workspace.Generate("x", "y", nil); err == nil {
		t.Error("expected error for empty folders")
	}
}

func mustMkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", p, err)
	}
}

func mustWrite(t *testing.T, p, content string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
}
