// Package codeworkspace generates a VS Code .code-workspace JSON file from
// the [code_workspace] section of workspace.toml.
//
// The output is written by callers at the project root as
// <name>.code-workspace, not under .devcontainer/, because .code-workspace
// files are the user-facing entry point that VS Code opens (`code
// foo.code-workspace`) rather than container infrastructure.
//
// Path resolution: every folders[].path is "~"-expanded against opts.HomeDir
// and then relativized against ctx.ProjectDir via filepath.Rel. The result
// is what VS Code interprets when opening the workspace, so writing
// "~/.claude" in workspace.toml from /home/u/proj produces "../.claude" in
// the output and lets VS Code traverse upward to $HOME-adjacent
// directories.
package codeworkspace

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/generate"
)

// Sentinel errors. All exported so CLI callers can classify failures with
// errors.Is and surface the right exit code / message.
var (
	// ErrInvalidFolderPath signals a structurally bad folders[].path entry:
	// empty, a "~user" form (home expansion only supports the current
	// user), or anything else that cannot be resolved relative to the
	// project directory.
	ErrInvalidFolderPath = errors.New("code_workspace: invalid folder path")

	// ErrNoFolders signals that neither workspace.toml nor the caller
	// (CLI flag) provided any folders. Generating an empty folders[] would
	// produce a .code-workspace that VS Code rejects, so we fail fast.
	ErrNoFolders = errors.New("code_workspace: no folders configured")

	// ErrMissingHomeDir signals that opts.HomeDir was empty while at least
	// one folder used "~" expansion. Caller must inject os.UserHomeDir()
	// (or a test stub) before invoking Generate when "~" paths are in play.
	ErrMissingHomeDir = errors.New("code_workspace: home directory required for ~ expansion")
)

// Options controls Generate. ExtraFolders is appended after the
// workspace.toml folders so CLI --folder flags supplement the declarative
// config. HomeDir is injected for testability — production callers pass
// os.UserHomeDir().
type Options struct {
	ExtraFolders []config.CodeWorkspaceFolder
	HomeDir      string
}

// Generate produces the JSON body for a .code-workspace file from ctx and
// opts. The output uses 2-space indent, ends with a single trailing
// newline, and elides "settings" / "extensions" when both inputs are
// empty. HTML escaping is disabled so URL and regex values pass through
// verbatim.
//
// Failure modes (all classifiable via errors.Is):
//   - ErrNoFolders     — no folders in WS.CodeWorkspace and no opts.ExtraFolders.
//   - ErrInvalidFolderPath — empty path, "~user" form, or rel computation failure.
//   - ErrMissingHomeDir — a folder uses "~" but opts.HomeDir is empty.
func Generate(ctx *generate.WorkspaceContext, opts Options) (string, error) {
	if ctx == nil || ctx.WS == nil {
		return "", fmt.Errorf("%w: nil workspace context", ErrNoFolders)
	}
	folders := collectFolders(ctx.WS.CodeWorkspace, opts.ExtraFolders)
	if len(folders) == 0 {
		return "", ErrNoFolders
	}
	rendered := make([]renderedFolder, 0, len(folders))
	for i, f := range folders {
		rel, name, err := resolveFolder(f, ctx.ProjectDir, opts.HomeDir)
		if err != nil {
			return "", fmt.Errorf("folders[%d]: %w", i, err)
		}
		rendered = append(rendered, renderedFolder{Name: name, Path: rel})
	}

	var settings map[string]any
	if ctx.WS.CodeWorkspace != nil && len(ctx.WS.CodeWorkspace.Settings) > 0 {
		settings = ctx.WS.CodeWorkspace.Settings
	}
	var ext *renderedExt
	if ctx.WS.CodeWorkspace != nil &&
		ctx.WS.CodeWorkspace.Extensions != nil &&
		len(ctx.WS.CodeWorkspace.Extensions.Recommendations) > 0 {
		ext = &renderedExt{Recommendations: ctx.WS.CodeWorkspace.Extensions.Recommendations}
	}
	out := renderedWorkspace{
		Folders:    rendered,
		Settings:   settings,
		Extensions: ext,
	}

	var raw bytes.Buffer
	enc := json.NewEncoder(&raw)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(out); err != nil {
		return "", fmt.Errorf("encode: %w", err)
	}
	var indented bytes.Buffer
	if err := json.Indent(&indented, bytes.TrimRight(raw.Bytes(), "\n"), "", "  "); err != nil {
		return "", fmt.Errorf("indent: %w", err)
	}
	return indented.String() + "\n", nil
}

// collectFolders concatenates the workspace.toml folders with any
// ExtraFolders supplied by the caller, preserving declaration order. A nil
// spec is treated as "no folders configured".
func collectFolders(spec *config.CodeWorkspaceSpec, extra []config.CodeWorkspaceFolder) []config.CodeWorkspaceFolder {
	var base []config.CodeWorkspaceFolder
	if spec != nil {
		base = spec.Folders
	}
	if len(base) == 0 && len(extra) == 0 {
		return nil
	}
	out := make([]config.CodeWorkspaceFolder, 0, len(base)+len(extra))
	out = append(out, base...)
	out = append(out, extra...)
	return out
}

// resolveFolder expands ~, joins relative paths against projectDir, then
// relativizes against projectDir. The returned (rel, name) is the final
// JSON form. name follows the precedence: explicit f.Name > basename of
// resolved abs path > projectDir basename when the folder *is* projectDir
// itself > "workspace" as the last resort.
func resolveFolder(f config.CodeWorkspaceFolder, projectDir, home string) (rel, name string, err error) {
	if f.Path == "" {
		return "", "", fmt.Errorf("%w: empty path", ErrInvalidFolderPath)
	}
	abs, expanded, err := expandPath(f.Path, projectDir, home)
	if err != nil {
		return "", "", err
	}
	rel, err = filepath.Rel(projectDir, abs)
	if err != nil {
		return "", "", fmt.Errorf("%w: rel %s: %w", ErrInvalidFolderPath, expanded, err)
	}
	// filepath.Rel can return platform-specific separators on Windows but
	// cocoon targets Linux containers; the host runner is typically Linux
	// too. Keep the conversion explicit so a future Windows host produces a
	// VS Code-compatible forward-slash path.
	rel = filepath.ToSlash(rel)
	switch {
	case f.Name != "":
		name = f.Name
	case rel == "." || rel == "":
		name = filepath.Base(projectDir)
	default:
		name = filepath.Base(abs)
	}
	if name == "" || name == "." || name == "/" {
		name = "workspace"
	}
	return rel, name, nil
}

// expandPath resolves "~" / "~/<rest>" against home and joins anything else
// with projectDir when relative. Returns (absoluteCleanPath, displayPath,
// err). The displayPath is the post-expansion string used in error context
// so users can see exactly which path failed.
func expandPath(p, projectDir, home string) (abs, display string, err error) {
	display = p
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home == "" {
			return "", display, fmt.Errorf("%w: %s", ErrMissingHomeDir, p)
		}
		switch p {
		case "~":
			display = home
			return filepath.Clean(home), display, nil
		default:
			rest := strings.TrimPrefix(p, "~/")
			abs = filepath.Join(home, rest)
			display = abs
			return filepath.Clean(abs), display, nil
		}
	}
	// Reject "~user" form (home expansion for other users). VS Code does
	// not interpret it, and supporting it would require shelling out to
	// getent passwd. Fail loudly instead of silently emitting a literal
	// "~bob/..." path that would not work.
	if strings.HasPrefix(p, "~") {
		return "", display, fmt.Errorf(`%w: "~user" form not supported (got %q)`, ErrInvalidFolderPath, p)
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p), display, nil
	}
	abs = filepath.Join(projectDir, p)
	return filepath.Clean(abs), abs, nil
}

// renderedWorkspace is the JSON shape emitted into <name>.code-workspace.
// The field order matches VS Code's own convention: folders first, then
// settings, then extensions.
type renderedWorkspace struct {
	Folders    []renderedFolder `json:"folders"`
	Settings   map[string]any   `json:"settings,omitempty"`
	Extensions *renderedExt     `json:"extensions,omitempty"`
}

type renderedFolder struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type renderedExt struct {
	Recommendations []string `json:"recommendations,omitempty"`
}
