// Package workspace implements .code-workspace file generation: scanning
// candidate folders under the parent directory, reading existing workspace
// files, and emitting the JSON structure consumed by VS Code.
package workspace

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Sentinel errors for callers / exit-code mapping.
var (
	ErrConfig    = errors.New("workspace: invalid options")
	ErrNoFolders = errors.New("workspace: no candidate folders found")
	ErrFailure   = errors.New("workspace: generation failed")
)

// AvailableDirs returns relative paths to candidate folders under parent.
//
// A direct subdirectory of parent that contains only further
// subdirectories (no regular files) is *expanded*: the function returns
// each child instead of the wrapper. This mirrors the historical
// `is_folder_only_dir` heuristic from lib/workspace.sh.
func AvailableDirs(parent string) ([]string, error) {
	tops, err := readSortedDirs(parent)
	if err != nil {
		return nil, fmt.Errorf("read parent dir: %w", err)
	}
	out := make([]string, 0, len(tops))
	for _, name := range tops {
		full := filepath.Join(parent, name)
		out = append(out, name)
		onlyDirs, oerr := isFolderOnly(full)
		if oerr != nil || !onlyDirs {
			continue
		}
		subs, serr := readSortedDirs(full)
		if serr != nil {
			continue
		}
		for _, s := range subs {
			out = append(out, name+"/"+s)
		}
	}
	return out, nil
}

// ListFiles returns the basenames of `*.code-workspace` files
// directly under workspacesDir, sorted.
func ListFiles(workspacesDir string) ([]string, error) {
	entries, err := os.ReadDir(workspacesDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read workspaces dir: %w", err)
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".code-workspace") {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

// CurrentFolders extracts the folder paths from an existing
// `.code-workspace` file. The leading `../../` prefix added by Generate
// is stripped so that the returned paths are comparable to AvailableDirs
// output.
func CurrentFolders(file string) ([]string, error) {
	raw, err := os.ReadFile(file) //nolint:gosec // path supplied by caller (CLI).
	if err != nil {
		return nil, fmt.Errorf("read workspace file: %w", err)
	}
	var ws struct {
		Folders []struct {
			Path string `json:"path"`
		} `json:"folders"`
	}
	if err := json.Unmarshal(raw, &ws); err != nil {
		return nil, fmt.Errorf("parse workspace JSON: %w", err)
	}
	out := make([]string, 0, len(ws.Folders))
	for _, f := range ws.Folders {
		out = append(out, strings.TrimPrefix(f.Path, "../../"))
	}
	return out, nil
}

// Generate writes a `.code-workspace` JSON file at outputFile.
//
// The "folders" array contains one entry per element of folders, with
// `path = "../../" + folder` and `name = base(folder)`. The "settings"
// object is read verbatim from settingsFile.
func Generate(outputFile, settingsFile string, folders []string) error {
	if outputFile == "" {
		return fmt.Errorf("%w: outputFile required", ErrConfig)
	}
	if settingsFile == "" {
		return fmt.Errorf("%w: settingsFile required", ErrConfig)
	}
	if len(folders) == 0 {
		return fmt.Errorf("%w: folders is empty", ErrConfig)
	}

	settingsRaw, err := os.ReadFile(settingsFile) //nolint:gosec // path supplied by CLI.
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	var settings any
	if uerr := json.Unmarshal(settingsRaw, &settings); uerr != nil {
		return fmt.Errorf("parse settings JSON: %w", uerr)
	}

	type folder struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	out := struct {
		Folders  []folder `json:"folders"`
		Settings any      `json:"settings"`
	}{
		Folders:  make([]folder, 0, len(folders)),
		Settings: settings,
	}
	for _, f := range folders {
		out.Folders = append(out.Folders, folder{
			Name: filepath.Base(f),
			Path: "../../" + f,
		})
	}

	buf, err := json.MarshalIndent(out, "", "\t")
	if err != nil {
		return fmt.Errorf("encode workspace JSON: %w", err)
	}
	if err := os.WriteFile(outputFile, buf, 0o600); err != nil {
		return fmt.Errorf("write workspace file: %w", err)
	}
	return nil
}

func readSortedDirs(parent string) ([]string, error) {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return nil, fmt.Errorf("readdir %s: %w", parent, err)
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out, nil
}

func isFolderOnly(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, fmt.Errorf("readdir %s: %w", dir, err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if !e.IsDir() {
			return false, nil
		}
	}
	return true, nil
}
