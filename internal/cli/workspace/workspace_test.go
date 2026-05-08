package workspacecli_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	workspacecli "github.com/sukekyo26/cocoon/internal/cli/workspace"
	"github.com/sukekyo26/cocoon/internal/tui"
)

type fakeSelector struct {
	singleResults []int
	singleIdx     int
	multiResult   []int
	singleErr     error
	multiErr      error
}

func (f *fakeSelector) SelectSingle(_ string, _ []string, _ int) (int, error) {
	if f.singleErr != nil {
		return 0, f.singleErr
	}
	if f.singleIdx >= len(f.singleResults) {
		return 0, nil
	}
	r := f.singleResults[f.singleIdx]
	f.singleIdx++
	return r, nil
}

func (f *fakeSelector) SelectMulti(_ string, _ []string, _ []int) ([]int, error) {
	if f.multiErr != nil {
		return nil, f.multiErr
	}
	return f.multiResult, nil
}

//nolint:unparam // test helper returns both buffers; not all callers use both
func runCmd(args ...string) (stdout, stderr bytes.Buffer, err error) {
	cmd := workspacecli.NewCommand(&stdout, &stderr)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return stdout, stderr, err
}

//nolint:unparam // test helper returns both buffers; not all callers use both
func runWithIO(stdin io.Reader, sel tui.Selector, args ...string) (stdout, stderr bytes.Buffer, err error) {
	cmd := workspacecli.NewCommandWithIO(stdin, &stdout, &stderr, sel)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return stdout, stderr, err
}

// makeRepoLayout creates: <root>/scripts/{workspaces,config} with a
// workspace-settings.json.example fixture, plus sibling directories
// <root>/{proj1,proj2} so AvailableDirs(<root>) returns both. Returns the
// scripts directory; the parent (root) is reachable via filepath.Dir.
func makeRepoLayout(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	scripts := filepath.Join(root, "scripts")
	configDir := filepath.Join(scripts, "config")
	for _, d := range []string{
		scripts,
		configDir,
		filepath.Join(root, "proj1"),
		filepath.Join(root, "proj2"),
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	settings := filepath.Join(configDir, "workspace-settings.json.example")
	if err := os.WriteFile(settings, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{
		filepath.Join(root, "proj1", "README"),
		filepath.Join(root, "proj2", "README"),
	} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return scripts
}

func TestRun_Help(t *testing.T) {
	t.Parallel()
	for _, flag := range []string{"help", "--help", "-h"} {
		stdout, _, err := runCmd(flag)
		if err != nil {
			t.Fatalf("flag=%q err = %v", flag, err)
		}
		if !strings.Contains(stdout.String(), "wsd workspace") {
			t.Errorf("flag=%q stdout missing banner: %q", flag, stdout.String())
		}
	}
}

func TestRun_MissingScriptsDir(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd()
	if !errors.Is(err, workspacecli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

func TestRun_FlagValueMissing(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("--scripts-dir")
	if !errors.Is(err, workspacecli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

func TestRun_UnknownArg(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("--bogus")
	if !errors.Is(err, workspacecli.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

func TestRunWith_NewWorkspaceFile(t *testing.T) {
	t.Parallel()
	scripts := makeRepoLayout(t)
	sel := &fakeSelector{multiResult: []int{0, 1}}
	stdin := strings.NewReader("my-ws\n")

	_, _, err := runWithIO(stdin, sel, "--scripts-dir", scripts)
	if err != nil {
		t.Fatalf("err = %v", err)
	}

	out := filepath.Join(scripts, "workspaces", "my-ws.code-workspace")
	if _, err := os.Stat(out); err != nil {
		t.Errorf("expected output file %s: %v", out, err)
	}
}

func TestRunWith_NoSelection(t *testing.T) {
	t.Parallel()
	scripts := makeRepoLayout(t)
	sel := &fakeSelector{multiResult: []int{}}
	stdin := strings.NewReader("noop\n")

	_, _, err := runWithIO(stdin, sel, "--scripts-dir", scripts)
	if !errors.Is(err, workspacecli.ErrFailure) {
		t.Fatalf("err = %v, want ErrFailure", err)
	}
}

func TestRunWith_SelectorCanceled(t *testing.T) {
	t.Parallel()
	scripts := makeRepoLayout(t)
	sel := &fakeSelector{multiErr: tui.ErrCanceled}
	stdin := strings.NewReader("\n")

	_, _, err := runWithIO(stdin, sel, "--scripts-dir", scripts)
	if !errors.Is(err, workspacecli.ErrCanceled) {
		t.Fatalf("err = %v, want ErrCanceled", err)
	}
}

func TestRunWith_NoSiblingFolders(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	scripts := filepath.Join(root, "scripts")
	if err := os.MkdirAll(scripts, 0o755); err != nil {
		t.Fatal(err)
	}
	sel := &fakeSelector{}
	stdin := strings.NewReader("\n")

	_, _, err := runWithIO(stdin, sel, "--scripts-dir", scripts)
	if !errors.Is(err, workspacecli.ErrFailure) {
		t.Fatalf("err = %v, want ErrFailure", err)
	}
}

// makeRepoWithExisting wraps makeRepoLayout and pre-creates a
// .code-workspace file so the existing-target prompt path runs.
func makeRepoWithExisting(t *testing.T, name string) string {
	t.Helper()
	scripts := makeRepoLayout(t)
	workspacesDir := filepath.Join(scripts, "workspaces")
	if err := os.MkdirAll(workspacesDir, 0o750); err != nil {
		t.Fatal(err)
	}
	wsFile := filepath.Join(workspacesDir, name+".code-workspace")
	body := `{"folders":[{"path":"../proj1"}]}`
	if err := os.WriteFile(wsFile, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return scripts
}

func TestRunWith_UpdateExistingFile(t *testing.T) {
	t.Parallel()
	scripts := makeRepoWithExisting(t, "existing")
	// First SelectSingle picks "Update existing" (idx 0); second picks file
	// index 0; SelectMulti picks both folders.
	sel := &fakeSelector{singleResults: []int{0, 0}, multiResult: []int{0, 1}}
	stdin := strings.NewReader("\n")

	_, _, err := runWithIO(stdin, sel, "--scripts-dir", scripts)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	out := filepath.Join(scripts, "workspaces", "existing.code-workspace")
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "proj2") {
		t.Errorf("expected new selection to include proj2:\n%s", body)
	}
}

func TestRunWith_ExistingFile_CreateNew(t *testing.T) {
	t.Parallel()
	scripts := makeRepoWithExisting(t, "old")
	// First SelectSingle picks "Create new" (idx 1); SelectMulti picks both.
	sel := &fakeSelector{singleResults: []int{1}, multiResult: []int{0, 1}}
	stdin := strings.NewReader("brand-new\n")

	_, _, err := runWithIO(stdin, sel, "--scripts-dir", scripts)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(scripts, "workspaces", "brand-new.code-workspace")); err != nil {
		t.Errorf("expected new file, got %v", err)
	}
}

func TestRunWith_ExistingFile_SelectorCanceledOnAction(t *testing.T) {
	t.Parallel()
	scripts := makeRepoWithExisting(t, "x")
	sel := &fakeSelector{singleErr: tui.ErrCanceled}
	stdin := strings.NewReader("\n")

	_, _, err := runWithIO(stdin, sel, "--scripts-dir", scripts)
	if !errors.Is(err, workspacecli.ErrCanceled) {
		t.Fatalf("err = %v, want ErrCanceled", err)
	}
}

func TestRunWith_BlankFilenameIsRejected(t *testing.T) {
	t.Parallel()
	scripts := makeRepoLayout(t)
	sel := &fakeSelector{multiResult: []int{0}}
	// First answer: empty -> reprompt; second: usable name.
	stdin := strings.NewReader("\nfinal\n")

	_, _, err := runWithIO(stdin, sel, "--scripts-dir", scripts)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(scripts, "workspaces", "final.code-workspace")); err != nil {
		t.Errorf("expected final.code-workspace, got %v", err)
	}
}

func TestRunWith_NewFilenameOverwriteConfirmed(t *testing.T) {
	t.Parallel()
	scripts := makeRepoLayout(t)
	// Pre-create a file the user will be asked to overwrite.
	wsDir := filepath.Join(scripts, "workspaces")
	if err := os.MkdirAll(wsDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wsDir, "exists.code-workspace"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	sel := &fakeSelector{multiResult: []int{0, 1}}
	// "Create new" path picks idx 1; user names the file "exists" then
	// confirms overwrite with "y".
	stdin := strings.NewReader("exists\ny\n")
	_, _, err := runWithIO(stdin, sel, "--scripts-dir", scripts)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestRunWith_NewFilenameOverwriteDeclined(t *testing.T) {
	t.Parallel()
	scripts := makeRepoLayout(t)
	wsDir := filepath.Join(scripts, "workspaces")
	if err := os.MkdirAll(wsDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wsDir, "x.code-workspace"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	// First SelectSingle: idx 1 (Create new); user enters "x" then "n".
	sel := &fakeSelector{singleResults: []int{1}, multiResult: []int{0}}
	stdin := strings.NewReader("x\nn\n")
	_, _, err := runWithIO(stdin, sel, "--scripts-dir", scripts)
	if !errors.Is(err, workspacecli.ErrCanceled) {
		t.Fatalf("err = %v, want ErrCanceled", err)
	}
}
