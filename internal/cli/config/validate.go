package configcli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/logx"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

func cmdValidateWorkspace(args []string, stdout, stderr io.Writer) error {
	log := logx.New(stdout, stderr)
	if err := requireArgs(args, 1, "validate-workspace", stderr); err != nil {
		return err
	}
	file := args[0]
	var pluginsDir string
	if len(args) >= 2 {
		pluginsDir = args[1]
	}

	ws, err := config.LoadWorkspace(file)
	if err != nil {
		var ve *config.ValidationError
		if errors.As(err, &ve) {
			printValidationErrors(log, file, ve)
			return ErrFailure
		}
		log.Errorf("ERROR: %s: %s", file, err)
		return ErrFailure
	}

	if pluginsDir != "" {
		var problems []string
		for _, id := range ws.Plugins.Enable {
			pluginToml := filepath.Join(pluginsDir, id, "plugin.toml")
			installSh := filepath.Join(pluginsDir, id, "install.sh")
			if !isFile(pluginToml) {
				problems = append(problems, fmt.Sprintf(
					"Plugin '%s' is enabled but plugins/%s/plugin.toml does not exist", id, id,
				))
			} else if !isFile(installSh) {
				problems = append(problems, fmt.Sprintf(
					"Plugin '%s' is missing plugins/%s/install.sh", id, id,
				))
			}
		}
		if len(problems) > 0 {
			for _, p := range problems {
				log.Errorf("ERROR: %s: %s", file, p)
			}
			return ErrFailure
		}
	}

	log.Infof("OK: %s is valid", file)
	return nil
}

func cmdValidatePlugins(args []string, stdout, stderr io.Writer) error {
	log := logx.New(stdout, stderr)
	if err := requireArgs(args, 1, "validate-plugins", stderr); err != nil {
		return err
	}
	dir := args[0]
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		log.Errorf("ERROR: Directory not found: %s", dir)
		return ErrFailure
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Errorf("ERROR: read dir %s: %s", dir, err)
		return ErrFailure
	}

	hasErrors := false
	validated := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pluginToml := filepath.Join(dir, e.Name(), "plugin.toml")
		installSh := filepath.Join(dir, e.Name(), "install.sh")
		st, statErr := os.Stat(pluginToml)
		if statErr != nil || st.IsDir() {
			continue
		}
		_, loadErr := plugin.Load(pluginToml)
		if loadErr != nil {
			var ve *config.ValidationError
			if errors.As(loadErr, &ve) {
				// Re-emit using `<id>/plugin.toml` as the path prefix (Python parity).
				printValidationErrors(log, e.Name()+"/plugin.toml", ve)
			} else {
				log.Errorf(
					"ERROR: %s/plugin.toml: Failed to parse: %s", e.Name(), loadErr)
			}
			hasErrors = true
		}
		if !isFile(installSh) {
			log.Errorf(
				"ERROR: %s: install.sh is required (plugins/%s/install.sh missing)",
				e.Name(), e.Name())
			hasErrors = true
		}
		validated++
	}

	if hasErrors {
		return ErrFailure
	}
	log.Infof("OK: %d plugins validated", validated)
	return nil
}

func isFile(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

// printValidationErrors writes every field error in v to log.Stderr using the
// "ERROR: <path>: <loc>: <msg>" line format that lib/*.sh consumers expect.
// Errors are sorted by location for deterministic output.
func printValidationErrors(log *logx.Logger, path string, v *config.ValidationError) {
	sorted := v.Sort()
	for _, fe := range sorted.Errors {
		log.Errorf("ERROR: %s: %s: %s", path, fe.LocString(), fe.Message)
	}
}
