// Package workspacecli implements the `wsd workspace` subcommand: an
// interactive .code-workspace generator that ports generate-workspace.sh.
package workspacecli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sukekyo26/cocoon/internal/envfile"
	"github.com/sukekyo26/cocoon/internal/i18n"
	"github.com/sukekyo26/cocoon/internal/logx"
	"github.com/sukekyo26/cocoon/internal/tui"
	"github.com/sukekyo26/cocoon/internal/workspace"
)

// ErrUsage signals a bad invocation; mapped to exit 2 at the binary boundary.
var ErrUsage = errors.New("usage error")

// ErrCanceled signals user cancellation; mapped to exit 130.
var ErrCanceled = errors.New("canceled")

// ErrFailure signals a runtime failure; mapped to exit 1.
var ErrFailure = errors.New("failure")

// runWorkspace executes the post-parse flow invoked by the cobra command in
// cmd.go after ` + "`--scripts-dir`" + ` has been resolved.
func runWorkspace(scriptsDir string, stdin io.Reader, stderr io.Writer, sel tui.Selector) error {
	log := logx.New(io.Discard, stderr)
	cat := i18n.New(i18n.Detect())
	parentDir := filepath.Dir(scriptsDir)
	workspacesDir := filepath.Join(scriptsDir, "workspaces")
	settingsFile := pickSettingsFile(scriptsDir)

	if err := os.MkdirAll(workspacesDir, 0o750); err != nil {
		return fmt.Errorf("%w: mkdir workspaces: %w", ErrFailure, err)
	}

	printHeader(log, cat, parentDir)

	files, lerr := workspace.ListFiles(workspacesDir)
	if lerr != nil {
		return fmt.Errorf("%w: %w", ErrFailure, lerr)
	}

	target, terr := selectTarget(cat, sel, log, files, workspacesDir)
	if terr != nil {
		return terr
	}

	dirs, derr := workspace.AvailableDirs(parentDir)
	if derr != nil {
		return fmt.Errorf("%w: %w", ErrFailure, derr)
	}
	if len(dirs) == 0 {
		log.Error("ERROR: " + cat.Msg("gen_ws_no_folders"))
		return ErrFailure
	}

	selected, serr := pickFolders(cat, sel, log, dirs, target)
	if serr != nil {
		return serr
	}

	output := target
	if output == "" {
		var perr error
		output, perr = promptNewFilename(stdin, stderr, workspacesDir, cat)
		if perr != nil {
			return perr
		}
	}

	if gerr := workspace.Generate(output, settingsFile, selected); gerr != nil {
		return fmt.Errorf("%w: %w", ErrFailure, gerr)
	}

	printResult(log, cat, output, selected)
	return nil
}

func selectTarget(
	cat *i18n.Catalog, sel tui.Selector, log *logx.Logger, files []string, workspacesDir string,
) (string, error) {
	if len(files) == 0 {
		log.Error(cat.Msg("gen_ws_no_files"))
		return "", nil
	}
	printList(log, cat.Msg("gen_ws_existing_files"), files)
	log.Error("")
	idx, err := sel.SelectSingle(cat.Msg("gen_ws_select_action"),
		[]string{cat.Msg("gen_ws_update_existing"), cat.Msg("gen_ws_create_new")}, 0)
	if err != nil {
		return "", mapSelErr(cat, log, err)
	}
	if idx != 0 {
		return "", nil
	}
	fileIdx, ferr := sel.SelectSingle(cat.Msg("gen_ws_select_file"), files, 0)
	if ferr != nil {
		return "", mapSelErr(cat, log, ferr)
	}
	return filepath.Join(workspacesDir, files[fileIdx]), nil
}

func pickFolders(
	cat *i18n.Catalog, sel tui.Selector, log *logx.Logger, dirs []string, target string,
) ([]string, error) {
	preselected := preselectedIdx(target, dirs)
	picks, err := sel.SelectMulti(cat.Msg("gen_ws_select_folders"), dirs, preselected)
	if err != nil {
		return nil, mapSelErr(cat, log, err)
	}
	if len(picks) == 0 {
		log.Error("ERROR: " + cat.Msg("gen_ws_no_selection"))
		return nil, ErrFailure
	}
	out := make([]string, len(picks))
	for i, p := range picks {
		out[i] = dirs[p]
	}
	return out, nil
}

func printResult(log *logx.Logger, cat *i18n.Catalog, output string, selected []string) {
	log.Error("")
	log.Error(cat.Msg("gen_ws_file_generated"))
	log.Error("   workspaces/" + filepath.Base(output))
	log.Error("")
	log.Error(cat.Msg("gen_ws_included_projects"))
	for _, f := range selected {
		log.Error("  - " + f)
	}
}

func pickSettingsFile(scriptsDir string) string {
	primary := filepath.Join(scriptsDir, "config", "workspace-settings.json")
	if _, err := os.Stat(primary); err == nil {
		return primary
	}
	return primary + ".example"
}

func printHeader(log *logx.Logger, cat *i18n.Catalog, parentDir string) {
	log.Error("")
	log.Error("========================================")
	log.Error(" " + cat.Msg("gen_ws_header"))
	log.Error("========================================")
	log.Error("")
	log.Errorf("%s %s", cat.Msg("gen_ws_scan_target"), parentDir)
	log.Errorf("%s %s", cat.Msg("gen_ws_output_dir"), "workspaces/")
	log.Error("")
}

func printList(log *logx.Logger, title string, items []string) {
	log.Error(title)
	for _, it := range items {
		log.Error("  - " + it)
	}
}

func mapSelErr(cat *i18n.Catalog, log *logx.Logger, err error) error {
	if errors.Is(err, tui.ErrCanceled) {
		log.Error(cat.Msg("gen_ws_cancelled"))
		return ErrCanceled
	}
	return fmt.Errorf("%w: %w", ErrFailure, err)
}

func preselectedIdx(targetFile string, dirs []string) []int {
	if targetFile == "" {
		return nil
	}
	current, err := workspace.CurrentFolders(targetFile)
	if err != nil || len(current) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(current))
	for _, c := range current {
		set[c] = struct{}{}
	}
	var out []int
	for i, d := range dirs {
		if _, ok := set[d]; ok {
			out = append(out, i)
		}
	}
	return out
}

func promptNewFilename(stdin io.Reader, stderr io.Writer, workspacesDir string, cat *i18n.Catalog) (string, error) {
	log := logx.New(io.Discard, stderr)
	scanner := bufio.NewScanner(stdin)
	for {
		log.Error("")
		// Prompt the user inline (no trailing newline) so the cursor stays on
		// the same line as the question. logx has no stderr-no-newline helper;
		// fmt.Fprint is not in the forbidigo Print* family so it stays clean.
		_, _ = fmt.Fprint(log.Stderr(), cat.Msg("gen_ws_prompt_filename"))
		if !scanner.Scan() {
			return "", fmt.Errorf("%w: read filename: %w", ErrFailure, scanner.Err())
		}
		name := strings.TrimSpace(scanner.Text())
		if name == "" {
			log.Error(cat.Msg("gen_ws_empty_filename"))
			continue
		}
		name = strings.TrimSuffix(name, ".code-workspace")
		out := filepath.Join(workspacesDir, name+".code-workspace")
		if _, err := os.Stat(out); err == nil {
			log.Error(cat.Msg("gen_ws_overwrite", name))
			ok, cerr := envfile.ConfirmYN(stdin, stderr, cat.Msg("gen_ws_confirm_yn"))
			if cerr != nil {
				return "", fmt.Errorf("%w: read confirm: %w", ErrFailure, cerr)
			}
			if !ok {
				log.Error(cat.Msg("gen_ws_cancelled"))
				return "", ErrCanceled
			}
		}
		return out, nil
	}
}
