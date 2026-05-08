package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/logx"
)

// checkHomeFilesHostOnly enforces the host-only invariant: when
// [home_files] is configured, `wsd setup` must run on the host because
// the touch step plus the bind-mount source resolution both depend on
// the host's filesystem namespace. Running inside a container would
// silently touch container paths and leave the host side empty, so we
// fail fast with a self-explanatory message instead.
//
// inContainer is injected so tests can simulate either side of the
// boundary without poking at hostguard's package-private test seam.
// Production callers pass hostguard.InsideContainer.
func checkHomeFilesHostOnly(
	ws *config.Workspace, log *logx.Logger, t Translator, inContainer func() bool,
) error {
	if ws == nil || ws.HomeFiles == nil || len(ws.HomeFiles.Files) == 0 {
		return nil
	}
	if !inContainer() {
		return nil
	}
	log.Error(t.Msg("setup_home_files_inside_container"))
	return ErrInsideContainer
}

// ensureHomeFiles materializes each [home_files].files entry on the host so
// that the docker-compose bind mount finds a regular file (not a missing
// path that Docker would auto-create as a directory). It is idempotent:
// existing files are left untouched. The created file is 0o600 and any
// missing parent directory is created with 0o700, which fits the typical
// "private credential file under $HOME" use case.
func ensureHomeFiles(ws *config.Workspace, log *logx.Logger, t Translator) error {
	if ws == nil || ws.HomeFiles == nil || len(ws.HomeFiles.Files) == 0 {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("home_files: resolve home: %w", err)
	}
	log.Info(t.Msg("setup_home_files"))
	homeClean := filepath.Clean(home)
	for _, rel := range ws.HomeFiles.Files {
		if err := assertSafeHomeRel(rel); err != nil {
			return err
		}
		abs := filepath.Join(home, rel)
		if !strings.HasPrefix(filepath.Clean(abs)+string(os.PathSeparator),
			homeClean+string(os.PathSeparator)) {
			return fmt.Errorf("%w: %q escapes home", ErrHomeFiles, rel)
		}
		info, statErr := os.Lstat(abs)
		switch {
		case statErr == nil:
			if info.IsDir() {
				return fmt.Errorf(
					"%w: %s exists as a directory (likely auto-created by a previous "+
						"container start before [home_files] was set); remove it and re-run setup",
					ErrHomeFiles, abs,
				)
			}
			continue
		case !os.IsNotExist(statErr):
			return fmt.Errorf("%w: stat %s: %w", ErrHomeFiles, abs, statErr)
		}
		if err := os.MkdirAll(filepath.Dir(abs), 0o700); err != nil {
			return fmt.Errorf("%w: mkdir parent for %s: %w", ErrHomeFiles, abs, err)
		}
		//nolint:gosec // path validated by assertSafeHomeRel above.
		f, err := os.OpenFile(abs, os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return fmt.Errorf("%w: touch %s: %w", ErrHomeFiles, abs, err)
		}
		_ = f.Close()
		log.Info(t.Msg("setup_home_files_touched", abs))
	}
	return nil
}

// assertSafeHomeRel mirrors the validator in internal/config so the setup
// step is robust even if it is called with a Workspace that bypassed
// validation (defense in depth).
func assertSafeHomeRel(rel string) error {
	switch {
	case rel == "":
		return fmt.Errorf("%w: entry must not be empty", ErrHomeFiles)
	case strings.HasPrefix(rel, "/"):
		return fmt.Errorf("%w: %q must be relative to ~/", ErrHomeFiles, rel)
	case strings.HasPrefix(rel, "~"):
		return fmt.Errorf("%w: %q must not start with ~", ErrHomeFiles, rel)
	case strings.HasPrefix(rel, "./") || strings.HasPrefix(rel, "../"):
		return fmt.Errorf("%w: %q must not start with ./ or ../", ErrHomeFiles, rel)
	case strings.Contains(rel, ":"):
		return fmt.Errorf("%w: %q must not contain `:`", ErrHomeFiles, rel)
	case strings.HasSuffix(rel, "/"):
		return fmt.Errorf("%w: %q must not end with /", ErrHomeFiles, rel)
	}
	for _, seg := range strings.Split(rel, "/") {
		if seg == ".." || seg == "" {
			return fmt.Errorf("%w: %q contains unsafe path segment", ErrHomeFiles, rel)
		}
	}
	return nil
}
