// Package repositories implements the [repositories].clone semantics from
// workspace.toml. It mirrors the behaviour of lib/repositories.sh so the
// `wsd repositories` subcommand and `wsd doctor` share a single source of
// truth.
package repositories

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sukekyo26/cocoon/internal/config"
	wsdexec "github.com/sukekyo26/cocoon/internal/exec"
	"github.com/sukekyo26/cocoon/internal/exec/gitx"
)

// Status enumerates the result of a per-repository health probe.
type Status string

// Status constants mirror the strings emitted by lib/repositories.sh.
const (
	StatusOK      Status = "OK"
	StatusMissing Status = "MISSING"
	StatusNotGit  Status = "NOT_GIT"
	StatusBadPath Status = "BAD_PATH"
)

// Entry is a normalized repositories.clone item with the resolved target path.
type Entry struct {
	URL               string
	Path              string
	Branch            string
	Depth             int
	RecurseSubmodules bool
}

// StatusReport is a single line emitted by Status (one per declared repo).
type StatusReport struct {
	Status Status
	Path   string
	URL    string
}

// CloneSummary aggregates the outcome of CloneAll.
type CloneSummary struct {
	Cloned  int
	Skipped int
	Failed  int
}

// ErrUnsafePath is returned by ValidatePath when a target escapes its parent
// or collides with the workspace-docker checkout itself.
var ErrUnsafePath = errors.New("unsafe path")

// ValidatePath resolves rel under parent without allowing escape via "/" or
// "..". The returned path is lexically resolved (no symlink walk) to match
// the bash implementation.
func ValidatePath(parent, rel string) (string, error) {
	if rel == "" || strings.HasPrefix(rel, "/") {
		return "", ErrUnsafePath
	}
	for _, seg := range strings.Split(rel, "/") {
		if seg == ".." {
			return "", ErrUnsafePath
		}
	}
	parentAbs, err := filepath.Abs(parent)
	if err != nil {
		return "", fmt.Errorf("abs parent: %w", err)
	}
	if info, err := os.Stat(parentAbs); err != nil || !info.IsDir() {
		return "", ErrUnsafePath
	}
	target := filepath.Clean(parentAbs + "/" + rel)
	prefix := parentAbs + string(os.PathSeparator)
	if !strings.HasPrefix(target+string(os.PathSeparator), prefix) {
		return "", ErrUnsafePath
	}
	if target == filepath.Join(parentAbs, "workspace-docker") {
		return "", ErrUnsafePath
	}
	return target, nil
}

// LoadEntries parses scriptDir/workspace.toml and returns one Entry per
// [repositories].clone item with the resolved target path. Returns (nil,nil)
// when the file or section is missing.
func LoadEntries(scriptDir string) ([]Entry, error) {
	path := filepath.Join(scriptDir, "workspace.toml")
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	ws, err := config.LoadWorkspace(path)
	if err != nil {
		return nil, fmt.Errorf("load workspace: %w", err)
	}
	if ws.Repositories == nil {
		return nil, nil
	}
	out := make([]Entry, 0, len(ws.Repositories.Clone))
	for _, c := range ws.Repositories.Clone {
		e := Entry{URL: c.URL, Path: "", Branch: "", Depth: 0, RecurseSubmodules: false}
		pathField := ""
		if c.Path != nil {
			pathField = *c.Path
		}
		e.Path = config.ResolveRepoPath(pathField, c.URL)
		if c.Branch != nil {
			e.Branch = *c.Branch
		}
		if c.Depth != nil {
			e.Depth = *c.Depth
		}
		if c.RecurseSubmodules != nil {
			e.RecurseSubmodules = *c.RecurseSubmodules
		}
		out = append(out, e)
	}
	return out, nil
}

// CheckStatus returns one StatusReport per declared repository, mirroring
// repositories_status in lib/repositories.sh.
func CheckStatus(scriptDir string) ([]StatusReport, error) {
	entries, err := LoadEntries(scriptDir)
	if err != nil || len(entries) == 0 {
		return nil, err
	}
	parent := filepath.Dir(filepath.Clean(scriptDir))
	out := make([]StatusReport, 0, len(entries))
	for _, e := range entries {
		target, err := ValidatePath(parent, e.Path)
		if err != nil {
			out = append(out, StatusReport{Status: StatusBadPath, Path: e.Path, URL: e.URL})
			continue
		}
		switch {
		case !pathExists(target):
			out = append(out, StatusReport{Status: StatusMissing, Path: e.Path, URL: e.URL})
		case !isDir(filepath.Join(target, ".git")):
			out = append(out, StatusReport{Status: StatusNotGit, Path: e.Path, URL: e.URL})
		default:
			out = append(out, StatusReport{Status: StatusOK, Path: e.Path, URL: e.URL})
		}
	}
	return out, nil
}

// CloneAll clones every entry that does not exist yet. Existing targets are
// skipped (never auto-pulled) so local changes are preserved. Failures are
// non-fatal — each entry is attempted independently.
func CloneAll(runner wsdexec.Runner, scriptDir string, log func(level, msg string)) (CloneSummary, error) {
	if log == nil {
		log = func(string, string) {}
	}
	var summary CloneSummary
	if _, err := exec.LookPath("git"); err != nil {
		log("warn", "git not found; skipping [repositories] clone")
		return summary, nil //nolint:nilerr // git absence is a soft skip, not an error
	}
	gc := gitx.New(runner)
	entries, err := LoadEntries(scriptDir)
	if err != nil {
		return summary, err
	}
	if len(entries) == 0 {
		return summary, nil
	}
	parent, err := filepath.Abs(filepath.Dir(filepath.Clean(scriptDir)))
	if err != nil {
		return summary, fmt.Errorf("resolve parent: %w", err)
	}
	log("info", fmt.Sprintf("Cloning [repositories] into %s (entries: %d)", parent, len(entries)))

	for _, e := range entries {
		target, err := ValidatePath(parent, e.Path)
		if err != nil {
			log("warn", fmt.Sprintf("  - %s -> %s: rejected (unsafe path)", e.URL, e.Path))
			summary.Failed++
			continue
		}
		if pathExists(target) {
			log("info", fmt.Sprintf("  - %s: exists, skipping (run `git pull` manually if needed)", e.Path))
			summary.Skipped++
			continue
		}
		log("info", fmt.Sprintf("  - %s -> %s", e.URL, e.Path))
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		out, cloneErr := gc.Clone(ctx, gitx.CloneOptions{
			URL:               e.URL,
			Target:            target,
			Branch:            e.Branch,
			Depth:             e.Depth,
			RecurseSubmodules: e.RecurseSubmodules,
		})
		cancel()
		if cloneErr != nil {
			log("warn", fmt.Sprintf("    clone failed: %s", strings.TrimSpace(string(out))))
			summary.Failed++
			continue
		}
		summary.Cloned++
	}
	log("info", fmt.Sprintf("[repositories] summary: cloned=%d skipped=%d failed=%d",
		summary.Cloned, summary.Skipped, summary.Failed))
	return summary, nil
}

func pathExists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}
