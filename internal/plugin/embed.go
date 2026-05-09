package plugin

import (
	"embed"
	"fmt"
	"io/fs"
)

// Catalog is the curated set of plugins shipped inside the cocoon
// binary. Every plugin lives at `<id>/plugin.toml` plus optional
// install.sh / install_user.sh, identical to the on-disk layout under
// `plugins/` in the repository.
//
// Callers that want to combine the embedded catalog with user
// overrides (`~/.cocoon/plugins/`) or per-project overrides
// (`<project>/.cocoon/plugins/`) should use [CatalogFS] to get a
// rooted fs.FS view; see internal/plugin/loader.go for the loaders.
//
//go:embed all:catalog
var embeddedCatalog embed.FS

// CatalogFS returns the embedded catalog rooted at the catalog/
// directory, so paths look like "<id>/plugin.toml" rather than
// "catalog/<id>/plugin.toml".
//
// The error return only fires if the //go:embed directive failed to
// include the catalog directory, which is a build-time programming
// error; callers can `t.Fatal(err)` or wrap it for the user.
func CatalogFS() (fs.FS, error) {
	sub, err := fs.Sub(embeddedCatalog, "catalog")
	if err != nil {
		return nil, fmt.Errorf("plugin: embedded catalog missing: %w", err)
	}
	return sub, nil
}
