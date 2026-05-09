package configcli

import (
	"io"
	"os"
	"path/filepath"

	"github.com/sukekyo26/cocoon/internal/logx"
)

// cmdPluginsTable emits one TSV row per plugin under <plugins-dir>:
//
//	id<TAB>name<TAB>default<TAB>description
//
// Rows are sorted by directory name (matches list-plugins ordering).
func cmdPluginsTable(args []string, stdout, stderr io.Writer) error {
	log := logx.New(stdout, stderr)
	if err := requireArgs(args, 1, "plugins-table", stderr); err != nil {
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
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pluginToml := filepath.Join(dir, e.Name(), "plugin.toml")
		st, statErr := os.Stat(pluginToml)
		if statErr != nil || st.IsDir() {
			continue
		}
		data, parseErr := decodeRaw(pluginToml)
		if parseErr != nil {
			log.Errorf("WARNING: Failed to parse %s: %s", pluginToml, parseErr)
			continue
		}
		md := asMap(data["metadata"])
		log.Infof("%s\t%s\t%s\t%s",
			e.Name(),
			asString(md["name"], e.Name()),
			boolString(asBool(md["default"], false)),
			asString(md["description"], ""),
		)
	}
	return nil
}
