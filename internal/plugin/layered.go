package plugin

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sort"
	"time"
)

// errIsDirectory is the sentinel returned by [LayeredFS]'s synthetic root
// directory file when a caller mistakes it for a regular file.
var errIsDirectory = errors.New("is a directory")

// Plugin source layer names. Returned by [LayeredFS.Source].
const (
	SourceEmbedded = "embedded"
	SourceUser     = "user"
	SourceProject  = "project"
)

// LayeredFS overlays plugin catalogs in priority order:
//
//	project (<project>/.cocoon/plugins) > user (~/.cocoon/plugins) > embedded
//
// Same-id directories are not merged: the highest-priority layer wins
// completely. The empty/absent layers are skipped silently. The receiver
// implements [fs.FS] and [fs.ReadDirFS] so it can be passed directly to
// [fs.WalkDir].
type LayeredFS struct {
	// layers is in priority order (highest first). Always ends with the
	// embedded catalog.
	layers []layeredEntry
	// sources maps a top-level plugin id to the layer name that owns it.
	sources map[string]string
}

type layeredEntry struct {
	name string
	fsys fs.FS
}

// NewLayeredFS returns a LayeredFS overlaying project (highest) > user >
// embedded. userDir / projectDir may be empty or refer to non-existent
// directories; those layers are skipped.
func NewLayeredFS(embedded fs.FS, userDir, projectDir string) *LayeredFS {
	var layers []layeredEntry
	if dir, ok := dirFSIfPresent(projectDir); ok {
		layers = append(layers, layeredEntry{name: SourceProject, fsys: dir})
	}
	if dir, ok := dirFSIfPresent(userDir); ok {
		layers = append(layers, layeredEntry{name: SourceUser, fsys: dir})
	}
	layers = append(layers, layeredEntry{name: SourceEmbedded, fsys: embedded})

	sources := make(map[string]string)
	for _, l := range layers {
		ents, err := fs.ReadDir(l.fsys, ".")
		if err != nil {
			continue
		}
		for _, e := range ents {
			if !e.IsDir() {
				continue
			}
			if _, claimed := sources[e.Name()]; claimed {
				continue
			}
			sources[e.Name()] = l.name
		}
	}
	return &LayeredFS{layers: layers, sources: sources}
}

// Source returns the layer name that owns the top-level plugin id, or "" if
// the id is not present in any layer. Layer names are [SourceProject],
// [SourceUser], or [SourceEmbedded].
func (l *LayeredFS) Source(id string) string { return l.sources[id] }

// Sources returns a copy of the id-to-source map covering every plugin id
// visible through the layered view.
func (l *LayeredFS) Sources() map[string]string {
	out := make(map[string]string, len(l.sources))
	for k, v := range l.sources {
		out[k] = v
	}
	return out
}

// LogOverrides emits one "INFO: plugin <id> overridden by <source>" line to w
// for every id whose winning layer is not the embedded catalog. w may be nil
// (no-op). Lines are emitted in stable id order.
func (l *LayeredFS) LogOverrides(w io.Writer) {
	if w == nil {
		return
	}
	overridden := make([]string, 0, len(l.sources))
	for id, src := range l.sources {
		if src == SourceEmbedded {
			continue
		}
		overridden = append(overridden, id)
	}
	sort.Strings(overridden)
	for _, id := range overridden {
		fmt.Fprintf(w, "INFO: plugin %s overridden by %s\n", id, l.sources[id])
	}
}

// Open implements [fs.FS]. For "." it returns a synthetic root directory
// that lists the union of top-level plugin ids (winner per id). For any
// nested path "<id>/..." it forwards to the layer that owns <id>.
func (l *LayeredFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	if name == "." {
		return l.openRoot(), nil
	}
	id := firstSegment(name)
	target := l.layerFor(id)
	if target == nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	f, err := target.fsys.Open(name)
	if err != nil {
		return nil, fmt.Errorf("layered open %s: %w", name, err)
	}
	return f, nil
}

// ReadDir implements [fs.ReadDirFS]. For "." it returns the union of
// top-level ids; nested names route to the layer that owns the id.
func (l *LayeredFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrInvalid}
	}
	if name == "." {
		return l.readRoot()
	}
	id := firstSegment(name)
	target := l.layerFor(id)
	if target == nil {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
	}
	ents, err := fs.ReadDir(target.fsys, name)
	if err != nil {
		return nil, fmt.Errorf("layered readdir %s: %w", name, err)
	}
	return ents, nil
}

func (l *LayeredFS) layerFor(id string) *layeredEntry {
	src := l.sources[id]
	if src == "" {
		return nil
	}
	for i := range l.layers {
		if l.layers[i].name == src {
			return &l.layers[i]
		}
	}
	return nil
}

func (l *LayeredFS) openRoot() fs.File {
	return &layeredRootFile{l: l, entries: nil, pos: 0}
}

func (l *LayeredFS) readRoot() ([]fs.DirEntry, error) {
	ids := make([]string, 0, len(l.sources))
	for id := range l.sources {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]fs.DirEntry, 0, len(ids))
	for _, id := range ids {
		target := l.layerFor(id)
		info, err := fs.Stat(target.fsys, id)
		if err != nil {
			return nil, fmt.Errorf("layered: stat %s: %w", id, err)
		}
		out = append(out, fs.FileInfoToDirEntry(info))
	}
	return out, nil
}

func dirFSIfPresent(dir string) (fs.FS, bool) {
	if dir == "" {
		return nil, false
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return nil, false
	}
	return os.DirFS(dir), true
}

func firstSegment(p string) string {
	for i := 0; i < len(p); i++ {
		if p[i] == '/' {
			return p[:i]
		}
	}
	return p
}

// layeredRootFile is the synthetic directory returned for ".".
type layeredRootFile struct {
	l       *LayeredFS
	entries []fs.DirEntry
	pos     int
}

// Stat returns the synthetic root info.
func (*layeredRootFile) Stat() (fs.FileInfo, error) { return layeredRootInfo{}, nil }

// Read always reports "is a directory".
func (*layeredRootFile) Read(_ []byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: ".", Err: errIsDirectory}
}

// Close is a no-op (no resources to release).
func (*layeredRootFile) Close() error { return nil }

// ReadDir implements [fs.ReadDirFile]. n <= 0 returns every remaining entry
// (nil error); positive n returns up to n with [io.EOF] when no more remain.
func (r *layeredRootFile) ReadDir(n int) ([]fs.DirEntry, error) {
	if r.entries == nil {
		ents, err := r.l.readRoot()
		if err != nil {
			return nil, err
		}
		r.entries = ents
	}
	if n <= 0 {
		rest := r.entries[r.pos:]
		r.pos = len(r.entries)
		return rest, nil
	}
	if r.pos >= len(r.entries) {
		return nil, io.EOF
	}
	end := r.pos + n
	if end > len(r.entries) {
		end = len(r.entries)
	}
	out := r.entries[r.pos:end]
	r.pos = end
	return out, nil
}

// layeredRootInfo describes the synthetic "." directory. Name() returns "."
// because [fs.WalkDir] reports the root via Stat().Name(); Mode carries
// [fs.ModeDir] so callers see a directory.
type layeredRootInfo struct{}

// Name returns ".".
func (layeredRootInfo) Name() string { return "." }

// Size returns 0.
func (layeredRootInfo) Size() int64 { return 0 }

// Mode returns ModeDir | 0o555.
func (layeredRootInfo) Mode() fs.FileMode { return fs.ModeDir | 0o555 }

// ModTime returns the zero time.
func (layeredRootInfo) ModTime() time.Time { return time.Time{} }

// IsDir returns true.
func (layeredRootInfo) IsDir() bool { return true }

// Sys returns nil.
func (layeredRootInfo) Sys() any { return nil }
