package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/pelletier/go-toml/v2"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/fsx"
	"github.com/sukekyo26/cocoon/internal/generate/tmplx"
	"github.com/sukekyo26/cocoon/internal/logx"
)

const (
	envFile = ".env"
	// shellCustomFile / shellCustomExample are the user-editable rc fragment
	// and its seed template. Both are shell-agnostic — bash, zsh and fish all
	// source the same file. Users put either POSIX or fish syntax inside,
	// matching their [container.shell].default selection.
	shellCustomFile    = "shell_custom"
	shellCustomExample = "shell_custom.example"
)

var workspaceTomlTemplate = template.Must(template.New("workspace.toml").Funcs(template.FuncMap{
	"formatStrSlice":   formatStrSlice,
	"formatPortsSlice": formatPortsSlice,
	"marshalTOML": func(v interface{}) (string, error) {
		b, err := toml.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("%w: marshal: %w", ErrTemplate, err)
		}
		return string(b), nil
	},
	"dict": dict,
}).Parse(`# workspace.toml — workspace-docker configuration
# Edit this file and run setup-docker.sh to regenerate

[container]
service_name = {{ printf "%q" .Container.ServiceName }}
username = {{ printf "%q" .Container.Username }}
os = {{ printf "%q" .Container.Os }}
os_version = {{ printf "%q" .Container.OsVersion }}

[plugins]
enable = {{ formatStrSlice .Plugins.Enable }}

[ports]
forward = {{ if .Ports }}{{ formatPortsSlice .Ports.Forward }}{{ else }}[]{{ end }}

[apt]
packages = {{ if .Apt }}{{ formatStrSlice .Apt.Packages }}{{ else }}[]{{ end }}

[volumes]
{{- range $k := .SortedVolumeKeys }}
{{ $k }} = {{ printf "%q" (index $.Volumes $k) }}
{{- end }}
{{ if .Devcontainer }}
{{ marshalTOML (dict "devcontainer" .Devcontainer) }}
{{- end }}
{{- if .Repositories }}
{{ marshalTOML (dict "repositories" .Repositories) }}
{{- end }}
`))

// workspaceTemplateData wraps Workspace with extra helpers for the template.
type workspaceTemplateData struct {
	*config.Workspace
}

// SortedVolumeKeys returns the [volumes] keys in deterministic order so the
// rendered workspace.toml stays diff-stable.
func (d workspaceTemplateData) SortedVolumeKeys() []string {
	keys := make([]string, 0, len(d.Volumes))
	for k := range d.Volumes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func dict(pairs ...interface{}) (map[string]interface{}, error) {
	if len(pairs)%2 != 0 {
		return nil, fmt.Errorf("%w: dict requires an even number of args", ErrTemplate)
	}
	m := make(map[string]interface{}, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		key, ok := pairs[i].(string)
		if !ok {
			return nil, fmt.Errorf("%w: dict key must be string", ErrTemplate)
		}
		m[key] = pairs[i+1]
	}
	return m, nil
}

// partialWS allows loading workspace.toml without requiring [container].
// Container is loaded leniently — when [container] is absent the pointer is
// nil; when present, only fields the interactive setup needs to preserve are
// captured (Container.Os / Container.OsVersion are read on `--init` to keep
// the user's previous OS pick as the picker default).
type partialWS struct {
	Container    *partialContainer        `toml:"container"`
	Plugins      config.PluginsSpec       `toml:"plugins"`
	Ports        *config.PortsSpec        `toml:"ports"`
	Apt          *config.AptSpec          `toml:"apt"`
	Volumes      map[string]string        `toml:"volumes"`
	Devcontainer config.Devcontainer      `toml:"devcontainer"`
	Repositories *config.RepositoriesSpec `toml:"repositories"`
}

type partialContainer struct {
	Os        string `toml:"os"`
	OsVersion string `toml:"os_version"`
}

func hasContainerSection(wsPath string) bool {
	data, err := os.ReadFile(wsPath)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "[container]" {
			return true
		}
	}
	return false
}

// loadPartialWorkspace loads workspace.toml without requiring [container].
// Uses lenient (non-strict) TOML decode so unknown sections are silently ignored.
func loadPartialWorkspace(wsPath string) (*partialWS, error) {
	data, err := os.ReadFile(wsPath) //nolint:gosec // wsPath is the configured workspace TOML path.
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", wsPath, err)
	}
	var p partialWS
	if err := toml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse %s: %w", wsPath, err)
	}
	return &p, nil
}

func buildWorkspace(
	serviceName, username, osID, osVersion string,
	plugins []string,
	ports []any,
	existing *partialWS,
) *config.Workspace {
	ws := &config.Workspace{
		Container: config.ContainerSpec{
			ServiceName: serviceName,
			Username:    username,
			Os:          osID,
			OsVersion:   osVersion,
		},
		Plugins: config.PluginsSpec{Enable: plugins},
	}
	if len(ports) > 0 {
		ws.Ports = &config.PortsSpec{Forward: ports}
	} else {
		ws.Ports = &config.PortsSpec{Forward: []any{}}
	}
	ws.Apt = &config.AptSpec{Packages: []string{}}
	ws.Volumes = map[string]string{}

	if existing != nil {
		if existing.Apt != nil && len(existing.Apt.Packages) > 0 {
			ws.Apt = existing.Apt
		}
		if len(existing.Volumes) > 0 {
			ws.Volumes = existing.Volumes
		}
		if len(existing.Devcontainer) > 0 {
			ws.Devcontainer = existing.Devcontainer
		}
		if existing.Repositories != nil {
			ws.Repositories = existing.Repositories
		}
	}
	return ws
}

func writeWorkspaceToml(path string, ws *config.Workspace) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	data := workspaceTemplateData{Workspace: ws}
	if err := workspaceTomlTemplate.Execute(f, data); err != nil {
		return fmt.Errorf("render workspace.toml: %w", err)
	}
	return nil
}

type envData struct {
	ProjectName string
	ServiceName string
	Username    string
	UID         int
	GID         int
	DockerGID   int
	OsImage     string
	OsVersion   string
}

var envTmpl = tmplx.MustParse("env", `# Environment variables for docker-compose
# Auto-generated from workspace.toml — do not edit manually
# Regenerate with: ./setup-docker.sh

COMPOSE_PROJECT_NAME={{ .ProjectName }}
CONTAINER_SERVICE_NAME={{ .ServiceName }}
USERNAME={{ .Username }}
UID={{ .UID }}
GID={{ .GID }}
DOCKER_GID={{ .DockerGID }}
OS_IMAGE={{ .OsImage }}
OS_VERSION={{ .OsVersion }}
`, nil)

func renderEnv(workspaceDir string, ws *config.Workspace, uid, gid, dockerGID int) (string, error) {
	data := envData{
		ProjectName: filepath.Base(workspaceDir),
		ServiceName: ws.Container.ServiceName,
		Username:    ws.Container.Username,
		UID:         uid,
		GID:         gid,
		DockerGID:   dockerGID,
		OsImage:     ws.Container.Os,
		OsVersion:   ws.Container.OsVersion,
	}
	out, err := tmplx.Render(envTmpl, data)
	if err != nil {
		return "", fmt.Errorf("env: %w", err)
	}
	return out, nil
}

func writeEnv(workspaceDir string, ws *config.Workspace, uid, gid, dockerGID int) error {
	body, err := renderEnv(workspaceDir, ws, uid, gid, dockerGID)
	if err != nil {
		return err
	}
	path := filepath.Join(workspaceDir, envFile)
	if err := fsx.AtomicWriteFile(path, []byte(body), 0o600); err != nil {
		return fmt.Errorf("write .env: %w", err)
	}
	return nil
}

func copyShellrcCustomIfMissing(workspaceDir string, log *logx.Logger, t Translator) error {
	dst := filepath.Join(workspaceDir, "config", shellCustomFile)
	if fileExists(dst) {
		return nil
	}
	src := filepath.Join(workspaceDir, "config", shellCustomExample)
	if !fileExists(src) {
		return nil
	}
	data, err := os.ReadFile(src) //nolint:gosec // src is workspaceDir/config/<known fixed name>.
	if err != nil {
		return fmt.Errorf("read shellrc seed: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(dst), err)
	}
	// shellrc fragments are sourced by the user's interactive shell; 0o644 is
	// the documented permission for these files in workspace-docker.
	if err := os.WriteFile(dst, data, 0o644); err != nil { //nolint:gosec // see comment above.
		return fmt.Errorf("write %s: %w", dst, err)
	}
	log.Info(t.Msg("setup_created_shellrc", shellCustomFile))
	return nil
}
