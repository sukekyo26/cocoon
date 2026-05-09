package plugin

// Volume holds a single (PluginName, VolumeName, MountPath) tuple derived
// from `[install].volumes` of a plugin.
type Volume struct {
	PluginName string
	VolumeName string
	MountPath  string
}

// GetVolumes returns the list of plugin-declared volumes for the given
// `enabled` ids, in input order. Volume names are auto-derived from the
// mount path's basename via DeriveVolumeName.
func GetVolumes(enabled []string, plugins map[string]*Plugin) []Volume {
	out := make([]Volume, 0)
	for _, id := range enabled {
		p, ok := plugins[id]
		if !ok {
			continue
		}
		name := p.Metadata.Name
		if name == "" {
			name = id
		}
		for _, path := range p.Install.Volumes {
			out = append(out, Volume{
				PluginName: name,
				VolumeName: DeriveVolumeName(path),
				MountPath:  path,
			})
		}
	}
	return out
}

// DeriveVolumeName strips a trailing slash, takes the basename, then trims
// a leading dot so the result satisfies Docker's volume naming rules
// ([a-zA-Z0-9][a-zA-Z0-9_.-]*).
func DeriveVolumeName(path string) string {
	for len(path) > 1 && path[len(path)-1] == '/' {
		path = path[:len(path)-1]
	}
	base := path
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			base = path[i+1:]
			break
		}
	}
	for len(base) > 0 && base[0] == '.' {
		base = base[1:]
	}
	return base
}
