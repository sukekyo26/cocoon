//nolint:testpackage
package setup

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/logx"
)

func discardLogger() *logx.Logger {
	return logx.New(io.Discard, io.Discard)
}

type stubTranslator struct{}

func (stubTranslator) Msg(key string, args ...any) string {
	if len(args) == 0 {
		return key
	}
	return fmt.Sprintf(key+":"+ /*args:*/ "%v", args)
}

func writeHomeFilesWS(files []string) *config.Workspace {
	if files == nil {
		return &config.Workspace{}
	}
	return &config.Workspace{HomeFiles: &config.HomeFilesSpec{Files: files}}
}

func withFakeHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	return dir
}

//nolint:paralleltest // mutates HOME via t.Setenv.
func TestEnsureHomeFiles_NoOpWhenNil(t *testing.T) {
	withFakeHome(t)
	if err := ensureHomeFiles(nil, discardLogger(), stubTranslator{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := ensureHomeFiles(&config.Workspace{}, discardLogger(), stubTranslator{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

//nolint:paralleltest // mutates HOME via t.Setenv.
func TestEnsureHomeFiles_TouchesMissingFile(t *testing.T) {
	home := withFakeHome(t)
	ws := writeHomeFilesWS([]string{".claude.json"})
	if err := ensureHomeFiles(ws, discardLogger(), stubTranslator{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	info, err := os.Stat(filepath.Join(home, ".claude.json"))
	if err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}
	if info.IsDir() {
		t.Fatal("expected file, got directory")
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("expected mode 0o600, got %o", info.Mode().Perm())
	}
}

//nolint:paralleltest // mutates HOME via t.Setenv.
func TestEnsureHomeFiles_CreatesParentDirs(t *testing.T) {
	home := withFakeHome(t)
	ws := writeHomeFilesWS([]string{".gemini/oauth_creds.json"})
	if err := ensureHomeFiles(ws, discardLogger(), stubTranslator{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	info, err := os.Stat(filepath.Join(home, ".gemini"))
	if err != nil {
		t.Fatalf("expected parent dir: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected directory")
	}
	if info.Mode().Perm() != 0o700 {
		t.Errorf("expected parent mode 0o700, got %o", info.Mode().Perm())
	}
	if _, err := os.Stat(filepath.Join(home, ".gemini/oauth_creds.json")); err != nil {
		t.Fatalf("expected file under parent: %v", err)
	}
}

//nolint:paralleltest // mutates HOME via t.Setenv.
func TestEnsureHomeFiles_Idempotent(t *testing.T) {
	home := withFakeHome(t)
	target := filepath.Join(home, ".claude.json")
	if err := os.WriteFile(target, []byte("existing"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	pre, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	ws := writeHomeFilesWS([]string{".claude.json"})
	if ensureErr := ensureHomeFiles(ws, discardLogger(), stubTranslator{}); ensureErr != nil {
		t.Fatalf("unexpected error: %v", ensureErr)
	}
	post, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if !pre.ModTime().Equal(post.ModTime()) {
		t.Error("ensureHomeFiles must not modify existing files")
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "existing" {
		t.Errorf("contents changed: %q", data)
	}
}

//nolint:paralleltest // mutates HOME via t.Setenv.
func TestEnsureHomeFiles_FailsIfExistingDir(t *testing.T) {
	home := withFakeHome(t)
	if err := os.MkdirAll(filepath.Join(home, ".claude.json"), 0o700); err != nil {
		t.Fatal(err)
	}
	ws := writeHomeFilesWS([]string{".claude.json"})
	err := ensureHomeFiles(ws, discardLogger(), stubTranslator{})
	if err == nil {
		t.Fatal("expected error when target is a directory")
	}
}

//nolint:paralleltest // mutates HOME via t.Setenv.
func TestEnsureHomeFiles_RejectsTraversal(t *testing.T) {
	withFakeHome(t)
	cases := []string{"/etc/passwd", "../escape", "~/.bad", ".cfg/../escape", "", "./foo", "foo:bar"}
	for _, c := range cases {
		ws := writeHomeFilesWS([]string{c})
		if err := ensureHomeFiles(ws, discardLogger(), stubTranslator{}); err == nil {
			t.Errorf("expected error for %q", c)
		}
	}
}

func inContainerYes() bool { return true }
func inContainerNo() bool  { return false }

func TestCheckHomeFilesHostOnly_BlocksInsideContainerWhenConfigured(t *testing.T) {
	t.Parallel()
	ws := writeHomeFilesWS([]string{".claude.json"})
	var buf bytes.Buffer
	log := logx.New(io.Discard, &buf)
	err := checkHomeFilesHostOnly(ws, log, stubTranslator{}, inContainerYes)
	if !errors.Is(err, ErrInsideContainer) {
		t.Fatalf("expected ErrInsideContainer, got %v", err)
	}
	if buf.Len() == 0 {
		t.Error("expected stderr message explaining the failure")
	}
}

func TestCheckHomeFilesHostOnly_AllowsOnHost(t *testing.T) {
	t.Parallel()
	ws := writeHomeFilesWS([]string{".claude.json"})
	if err := checkHomeFilesHostOnly(ws, discardLogger(), stubTranslator{}, inContainerNo); err != nil {
		t.Fatalf("unexpected error on host: %v", err)
	}
}

func TestCheckHomeFilesHostOnly_NoOpWhenUnset(t *testing.T) {
	t.Parallel()
	if err := checkHomeFilesHostOnly(
		&config.Workspace{}, discardLogger(), stubTranslator{}, inContainerYes,
	); err != nil {
		t.Errorf("unexpected error when [home_files] absent: %v", err)
	}
	ws := writeHomeFilesWS([]string{})
	if err := checkHomeFilesHostOnly(
		ws, discardLogger(), stubTranslator{}, inContainerYes,
	); err != nil {
		t.Errorf("unexpected error when files=[]: %v", err)
	}
}
