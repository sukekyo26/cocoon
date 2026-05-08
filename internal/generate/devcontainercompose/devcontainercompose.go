// Package devcontainercompose generates the .devcontainer/docker-compose.yml
// file. The output is rendered through text/template; YAML quoting on service
// names is handled by the safeName helper.
package devcontainercompose

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"github.com/sukekyo26/cocoon/internal/generate"
	"github.com/sukekyo26/cocoon/internal/generate/tmplx"
)

type templateData struct {
	ServiceName string
	Sidecars    []string
}

var tmpl = tmplx.MustParse("devcontainer-compose", `# Auto-generated from workspace.toml — do not edit directly.
services:
  {{ safeName .ServiceName }}:
    volumes:
      - ..:/home/${USERNAME}/workspace:cached
      - /var/run/docker.sock:/var/run/docker.sock

    # GID is set automatically by setup-docker.sh
    group_add:
      - "${DOCKER_GID}"

    command: sleep infinity{{ if .Sidecars }}

    depends_on:{{ range .Sidecars }}
      - {{ safeName . }}{{ end }}{{ end }}
`, template.FuncMap{"safeName": safeName})

// Generate returns the devcontainer compose body. Sidecar services are
// emitted in the alphabetical order produced by ctx.SidecarNames.
func Generate(ctx *generate.WorkspaceContext) (string, error) {
	data := templateData{
		ServiceName: ctx.ServiceName(),
		Sidecars:    ctx.SidecarNames(),
	}
	out, err := tmplx.Render(tmpl, data)
	if err != nil {
		return "", fmt.Errorf("devcontainer compose: %w", err)
	}
	return out, nil
}

// safeName double-quotes service names containing YAML metacharacters by
// round-tripping through json.Marshal (which produces valid YAML double
// quotes for any string).
func safeName(s string) string {
	if strings.ContainsAny(s, ":{}[]#&*!|>%@, ") {
		b, err := json.Marshal(s)
		if err == nil {
			return string(b)
		}
	}
	return s
}
