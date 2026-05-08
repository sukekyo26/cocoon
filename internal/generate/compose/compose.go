// Package compose generates docker-compose.yml using ordered maps via the
// yamlx wrapper: 2-space block indent, sequences indented relative to their
// parent key, strings double-quoted only when they contain YAML-special
// characters.
package compose

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/generate"
	"github.com/sukekyo26/cocoon/internal/generate/yamlx"
	"github.com/sukekyo26/cocoon/internal/plugin"
)

const header = "# Auto-generated from workspace.toml — do not edit directly.\n"

// ErrVolumeNameConflict is returned when a workspace.toml [volumes] entry
// shares a name with an auto-derived plugin volume.
var ErrVolumeNameConflict = errors.New("compose: volume name conflicts with enabled plugin")

// Defaults applied to [container.resources] when the workspace.toml omits a
// field.
const (
	defaultStopGracePeriod = "30s"
	defaultShmSize         = "1gb"
	defaultPidsLimit       = 4096
	defaultNofileSoft      = 65536
	defaultNofileHard      = 1048576
)

// Options carries the inputs required by Generate beyond the WorkspaceContext
// (specifically the loaded plugin metadata used to derive named volumes).
type Options struct {
	Plugins  map[string]*plugin.Plugin
	Warnings io.Writer
}

// Generate renders docker-compose.yml from ctx and opts.
func Generate(ctx *generate.WorkspaceContext, opts Options) (string, error) {
	enabled := ctx.EnabledPlugins()
	pluginVols := plugin.GetVolumes(enabled, opts.Plugins)
	customVols := ctx.CustomVolumes()

	mergedPlugin, mergedCustom, err := mergeVolumes(pluginVols, customVols, opts.Warnings)
	if err != nil {
		return "", err
	}

	homeMounts, err := ctx.HomeFileMounts("")
	if err != nil {
		return "", fmt.Errorf("compose: %w", err)
	}
	mounts := buildVolumeMounts(ctx, mergedPlugin, mergedCustom, homeMounts)
	service := buildService(ctx, mounts)
	volDefs := buildVolDefs(mergedPlugin, mergedCustom)

	servicesContent := []yamlx.Pair{{Key: ctx.ServiceName(), Value: service}}
	for _, sc := range ctx.SidecarNames() {
		spec := ctx.Sidecars()[sc]
		sidecarSvc, sidecarVols := buildSidecar(sc, spec)
		servicesContent = append(servicesContent, yamlx.Pair{Key: sc, Value: sidecarSvc})
		volDefs = append(volDefs, sidecarVols...)
	}

	doc := yamlx.Map(
		yamlx.Pair{Key: "services", Value: yamlx.Map(servicesContent...)},
		yamlx.Pair{Key: "volumes", Value: yamlx.Map(volDefs...)},
	)
	body, err := yamlx.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("compose: %w", err)
	}
	return header + string(body), nil
}

// mergeVolumes mirrors ComposeGenerator's de-duplication and conflict checks.
func mergeVolumes(
	pluginVols []plugin.Volume,
	customVols map[string]string,
	warnings io.Writer,
) (mergedPlugin []plugin.Volume, mergedCustom []volPair, err error) {
	type src struct{ label, volName string }
	pathToSrc := map[string]src{}

	for _, pv := range pluginVols {
		if existing, dup := pathToSrc[pv.MountPath]; dup {
			if warnings != nil {
				fmt.Fprintf(warnings,
					"WARNING: Volume path '%s' is defined by both %s and plugin '%s'. Using single volume '%s'.\n",
					pv.MountPath, existing.label, pv.PluginName, existing.volName)
			}
			continue
		}
		pathToSrc[pv.MountPath] = src{label: "plugin '" + pv.PluginName + "'", volName: pv.VolumeName}
		mergedPlugin = append(mergedPlugin, pv)
	}

	pluginVolNames := map[string]struct{}{}
	for _, pv := range mergedPlugin {
		pluginVolNames[pv.VolumeName] = struct{}{}
	}

	customNames := sortedKeys(customVols)
	for _, name := range customNames {
		if _, conflict := pluginVolNames[name]; conflict {
			return nil, nil, fmt.Errorf(
				"%w: '%s' (remove it from workspace.toml or disable the plugin)",
				ErrVolumeNameConflict, name)
		}
	}

	for _, name := range customNames {
		path := customVols[name]
		if existing, dup := pathToSrc[path]; dup {
			if warnings != nil {
				fmt.Fprintf(warnings,
					"WARNING: Volume path '%s' is defined by both %s and workspace.toml volume '%s'. Using single volume '%s'.\n",
					path, existing.label, name, existing.volName)
			}
			continue
		}
		pathToSrc[path] = src{label: "workspace.toml volume '" + name + "'", volName: name}
		mergedCustom = append(mergedCustom, volPair{Name: name, Path: path})
	}
	return mergedPlugin, mergedCustom, nil
}

type volPair struct {
	Name string
	Path string
}

func buildVolumeMounts(
	ctx *generate.WorkspaceContext,
	pluginVols []plugin.Volume,
	customVols []volPair,
	homeFiles []config.Mount,
) []*yaml.Node {
	mounts := make(
		[]*yaml.Node,
		0,
		2+len(pluginVols)+len(customVols)+len(ctx.Mounts())+len(homeFiles),
	)
	mounts = append(mounts,
		yamlx.QuotedIfSpecial("..:/home/${USERNAME}/workspace:cached"),
		yamlx.QuotedIfSpecial("local:/home/${USERNAME}/.local"),
	)
	if ctx.WS.Container.DockerSocketEnabled() {
		mounts = append(mounts, yamlx.QuotedIfSpecial("/var/run/docker.sock:/var/run/docker.sock"))
	}
	for _, pv := range pluginVols {
		mounts = append(mounts, yamlx.QuotedIfSpecial(pv.VolumeName+":"+pv.MountPath))
	}
	for _, cv := range customVols {
		mounts = append(mounts, yamlx.QuotedIfSpecial(cv.Name+":"+cv.Path))
	}
	for _, m := range ctx.Mounts() {
		mount := m.Source + ":" + m.Target
		if m.Readonly {
			mount += ":ro"
		}
		mounts = append(mounts, yamlx.QuotedIfSpecial(mount))
	}
	for _, m := range homeFiles {
		mounts = append(mounts, yamlx.QuotedIfSpecial(m.Source+":"+m.Target))
	}
	return mounts
}

func buildService(ctx *generate.WorkspaceContext, mounts []*yaml.Node) *yaml.Node {
	pairs := []yamlx.Pair{
		{Key: "container_name", Value: yamlx.QuotedIfSpecial("${CONTAINER_SERVICE_NAME}")},
		{Key: "build", Value: yamlx.Map(
			yamlx.Pair{Key: "context", Value: yamlx.QuotedIfSpecial(".")},
			yamlx.Pair{Key: "args", Value: yamlx.Seq(
				yamlx.QuotedIfSpecial("OS_IMAGE=${OS_IMAGE}"),
				yamlx.QuotedIfSpecial("OS_VERSION=${OS_VERSION}"),
				yamlx.QuotedIfSpecial("USERNAME=${USERNAME}"),
				yamlx.QuotedIfSpecial("UID=${UID}"),
				yamlx.QuotedIfSpecial("GID=${GID}"),
				yamlx.QuotedIfSpecial("DOCKER_GID=${DOCKER_GID}"),
			)},
		)},
		{Key: "user", Value: yamlx.QuotedIfSpecial("${UID}:${GID}")},
		{Key: "environment", Value: stringSeq(ctx.BuildEnvironment())},
		{Key: "volumes", Value: yamlx.Seq(mounts...)},
	}
	if ports := ctx.ComposeForwardPorts(); len(ports) > 0 {
		items := make([]*yaml.Node, 0, len(ports))
		for _, p := range ports {
			items = append(items, portNode(p))
		}
		pairs = append(pairs, yamlx.Pair{Key: "ports", Value: yamlx.Seq(items...)})
	}
	if hosts := ctx.ExtraHosts(); len(hosts) > 0 {
		items := make([]*yaml.Node, 0, len(hosts))
		for _, k := range sortedKeys(hosts) {
			items = append(items, yamlx.QuotedIfSpecial(k+":"+hosts[k]))
		}
		pairs = append(pairs, yamlx.Pair{Key: "extra_hosts", Value: yamlx.Seq(items...)})
	}
	if servers := ctx.DNSServers(); len(servers) > 0 {
		pairs = append(pairs, yamlx.Pair{Key: "dns", Value: stringSeq(servers)})
	}
	if search := ctx.DNSSearch(); len(search) > 0 {
		pairs = append(pairs, yamlx.Pair{Key: "dns_search", Value: stringSeq(search)})
	}
	if sysctls := ctx.Sysctls(); len(sysctls) > 0 {
		entries := make([]yamlx.Pair, 0, len(sysctls))
		for _, k := range sortedKeys(sysctls) {
			entries = append(entries, yamlx.Pair{Key: k, Value: sysctlNode(sysctls[k])})
		}
		pairs = append(pairs, yamlx.Pair{Key: "sysctls", Value: yamlx.Map(entries...)})
	}
	if add := ctx.CapAdd(); len(add) > 0 {
		pairs = append(pairs, yamlx.Pair{Key: "cap_add", Value: stringSeq(add)})
	}
	if drop := ctx.CapDrop(); len(drop) > 0 {
		pairs = append(pairs, yamlx.Pair{Key: "cap_drop", Value: stringSeq(drop)})
	}
	if sec := ctx.SecurityOptions(); len(sec) > 0 {
		pairs = append(pairs, yamlx.Pair{Key: "security_opt", Value: stringSeq(sec)})
	}
	if ctx.WS.Container.DockerSocketEnabled() {
		pairs = append(pairs, yamlx.Pair{
			Key:   "group_add",
			Value: yamlx.Seq(yamlx.QuotedIfSpecial("${DOCKER_GID}")),
		})
	}
	pairs = append(pairs, yamlx.Pair{Key: "tty", Value: yamlx.Bool(true)})
	pairs = append(pairs, yamlx.Pair{Key: "init", Value: yamlx.Bool(true)})
	pairs = append(pairs, yamlx.Pair{Key: "command", Value: yamlx.QuotedIfSpecial("sleep infinity")})
	pairs = append(pairs, applyResources(ctx)...)
	return yamlx.Map(pairs...)
}

func applyResources(ctx *generate.WorkspaceContext) []yamlx.Pair {
	r := ctx.Resources()
	stopGrace := defaultStopGracePeriod
	shmSize := defaultShmSize
	pidsLimit := defaultPidsLimit
	nofileSoft := defaultNofileSoft
	nofileHard := defaultNofileHard
	var cpus *float64
	var memory string
	if r != nil {
		if r.StopGracePeriod != nil {
			stopGrace = *r.StopGracePeriod
		}
		if r.ShmSize != nil {
			shmSize = *r.ShmSize
		}
		if r.PidsLimit != nil {
			pidsLimit = *r.PidsLimit
		}
		if r.NofileSoft != nil {
			nofileSoft = *r.NofileSoft
		}
		if r.NofileHard != nil {
			nofileHard = *r.NofileHard
		}
		if r.CPUs != nil {
			cpus = r.CPUs
		}
		if r.Memory != nil {
			memory = *r.Memory
		}
	}
	pairs := []yamlx.Pair{
		{Key: "stop_grace_period", Value: yamlx.QuotedIfSpecial(stopGrace)},
		{Key: "shm_size", Value: yamlx.QuotedIfSpecial(shmSize)},
		{Key: "pids_limit", Value: yamlx.Int(pidsLimit)},
		{Key: "ulimits", Value: yamlx.Map(
			yamlx.Pair{Key: "nofile", Value: yamlx.Map(
				yamlx.Pair{Key: "soft", Value: yamlx.Int(nofileSoft)},
				yamlx.Pair{Key: "hard", Value: yamlx.Int(nofileHard)},
			)},
		)},
	}
	if cpus != nil {
		pairs = append(pairs, yamlx.Pair{Key: "cpus", Value: floatNode(*cpus)})
	}
	if memory != "" {
		pairs = append(pairs, yamlx.Pair{Key: "mem_limit", Value: yamlx.QuotedIfSpecial(memory)})
	}
	return pairs
}

func buildVolDefs(plugin []plugin.Volume, custom []volPair) []yamlx.Pair {
	out := make([]yamlx.Pair, 0, 1+len(plugin)+len(custom))
	out = append(out, yamlx.Pair{
		Key:   "local",
		Value: namedVolume("${COMPOSE_PROJECT_NAME}_${CONTAINER_SERVICE_NAME}_local"),
	})
	for _, pv := range plugin {
		out = append(out, yamlx.Pair{
			Key:   pv.VolumeName,
			Value: namedVolume(fmt.Sprintf("${COMPOSE_PROJECT_NAME}_${CONTAINER_SERVICE_NAME}_%s", pv.VolumeName)),
		})
	}
	for _, cv := range custom {
		out = append(out, yamlx.Pair{
			Key:   cv.Name,
			Value: namedVolume(fmt.Sprintf("${COMPOSE_PROJECT_NAME}_${CONTAINER_SERVICE_NAME}_%s", cv.Name)),
		})
	}
	return out
}

func namedVolume(name string) *yaml.Node {
	return yamlx.Map(yamlx.Pair{Key: "name", Value: yamlx.QuotedIfSpecial(name)})
}

func stringSeq(values []string) *yaml.Node {
	items := make([]*yaml.Node, 0, len(values))
	for _, v := range values {
		items = append(items, yamlx.QuotedIfSpecial(v))
	}
	return yamlx.Seq(items...)
}

// sysctlNode emits the sysctl value as a plain int when it parsed as a
// numeric, otherwise as a (possibly quoted) string. Validation already
// rejects any other type.
func sysctlNode(v any) *yaml.Node {
	switch n := v.(type) {
	case int:
		return yamlx.Int(n)
	case int64:
		return yamlx.Int(int(n))
	case string:
		return yamlx.QuotedIfSpecial(n)
	default:
		return yamlx.QuotedIfSpecial(fmt.Sprintf("%v", v))
	}
}

func floatNode(f float64) *yaml.Node {
	s := strconv.FormatFloat(f, 'f', -1, 64)
	if !strings.ContainsAny(s, ".eE") {
		s += ".0"
	}
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!float",
		Value: s,
	}
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// portNode emits a single docker-compose ports entry. Short form becomes a
// (possibly quoted) scalar; long form becomes a mapping with the fixed key
// order target, published, host_ip, protocol, mode so generated YAML is
// stable across runs regardless of TOML decoder map iteration.
func portNode(p config.ComposePort) *yaml.Node {
	if !p.IsLong() {
		return yamlx.QuotedIfSpecial(p.Short)
	}
	pairs := make([]yamlx.Pair, 0, len(p.Long))
	for _, k := range config.LongFormKeyOrder() {
		v, ok := p.Long[k]
		if !ok {
			continue
		}
		pairs = append(pairs, yamlx.Pair{Key: k, Value: anyNode(v)})
	}
	return yamlx.Map(pairs...)
}
