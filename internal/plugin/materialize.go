package plugin

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// Materialize copies every plugin listed in ids from src onto the host
// filesystem under dst. The destination becomes <dst>/<id>/ for each id,
// mirroring the on-disk layout the generated docker-compose.yml's
// `additional_contexts: plugins:` entry expects.
//
// dst is created if missing. Each <dst>/<id>/ subtree is removed before
// the fresh copy lands so a renamed install.sh on disk does not linger
// next to a fresh one. Other entries under dst (plugins removed from
// [plugins].enable, unrelated cache files) are left alone.
func Materialize(src fs.FS, ids []string, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return fmt.Errorf("materialize: mkdir %s: %w", dst, err)
	}
	for _, id := range ids {
		sub, err := fs.Sub(src, id)
		if err != nil {
			return fmt.Errorf("materialize: sub %s: %w", id, err)
		}
		idDst := filepath.Join(dst, id)
		if err := os.RemoveAll(idDst); err != nil {
			return fmt.Errorf("materialize: clean %s: %w", idDst, err)
		}
		if err := copyFromFS(sub, idDst); err != nil {
			return fmt.Errorf("materialize: copy %s: %w", id, err)
		}
	}
	return nil
}

func copyFromFS(src fs.FS, dst string) error {
	walkErr := fs.WalkDir(src, ".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		target := filepath.Join(dst, path)
		if d.IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("mkdir %s: %w", target, err)
			}
			return nil
		}
		in, err := src.Open(path)
		if err != nil {
			return fmt.Errorf("open %s: %w", path, err)
		}
		defer func() { _ = in.Close() }()
		out, err := os.Create(target) //nolint:gosec // target is composed from a controlled dst.
		if err != nil {
			return fmt.Errorf("create %s: %w", target, err)
		}
		defer func() { _ = out.Close() }()
		if _, err := io.Copy(out, in); err != nil {
			return fmt.Errorf("copy %s: %w", target, err)
		}
		// install.sh and similar shell scripts need an exec bit so the
		// Dockerfile's `bash /tmp/plugin/install.sh` can run them when
		// cocoon decides not to invoke them via `bash` explicitly.
		if filepath.Ext(path) == ".sh" {
			if err := os.Chmod(target, 0o755); err != nil {
				return fmt.Errorf("chmod %s: %w", target, err)
			}
		}
		return nil
	})
	if walkErr != nil {
		return fmt.Errorf("walk: %w", walkErr)
	}
	return nil
}
