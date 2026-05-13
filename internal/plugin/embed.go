package plugin

import (
	"embed"
	"fmt"
	"io/fs"
)

// embeddedCatalog mirrors the on-disk layout under `plugins/` so
// `<id>/plugin.toml` paths work identically across embedded and on-disk
// loaders.
//
//go:embed all:catalog
var embeddedCatalog embed.FS

// CatalogFS rebases the embedded catalog so paths look like
// "<id>/plugin.toml" rather than "catalog/<id>/plugin.toml". The error
// only fires when the //go:embed directive failed (build-time bug).
func CatalogFS() (fs.FS, error) {
	sub, err := fs.Sub(embeddedCatalog, "catalog")
	if err != nil {
		return nil, fmt.Errorf("plugin: embedded catalog missing: %w", err)
	}
	return sub, nil
}
