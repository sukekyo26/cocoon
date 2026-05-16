// Package compose generates docker-compose.yml using ordered maps via the
// yamlx wrapper: 2-space block indent, sequences indented relative to their
// parent key, strings double-quoted only when they contain YAML-special
// characters.
package compose

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
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
// shares a name with an auto-derived plugin volume or with a name cocoon
// reserves at the compose level (see reservedVolumeNames).
var ErrVolumeNameConflict = errors.New(
	"compose: volume name conflicts with an enabled plugin or a cocoon reserved name",
)

// reservedVolumeNames are volume keys cocoon emits unconditionally in the
// generated compose. Plugins and workspace.toml [volumes] cannot reuse them.
var reservedVolumeNames = map[string]struct{}{
	"local":  {}, // /home/<user>/.local persistence
	"cocoon": {}, // /home/<user>/.cocoon persistence (.shellrc, history, etc.)
}

// reservedMountPaths are container target paths cocoon owns. A plugin or
// workspace.toml [volumes] entry that maps to one of these targets — even
// under a different volume name — would collide with the unconditional
// `local:`/`cocoon:` mounts emitted by buildVolumeMounts and break
// `docker compose up`. We reject them at gen time so the user sees a clear
// error rather than a runtime mount conflict.
var reservedMountPaths = map[string]struct{}{
	"/home/${USERNAME}/.local":  {},
	"/home/${USERNAME}/.cocoon": {},
}

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

	mounts := buildVolumeMounts(ctx, mergedPlugin, mergedCustom, ctx.HomeFileMounts())
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

// volSrc names the source of a mount path so dedup warnings can be precise.
type volSrc struct{ label, volName string }

// mergeVolumes mirrors ComposeGenerator's de-duplication and conflict checks.
func mergeVolumes(
	pluginVols []plugin.Volume,
	customVols map[string]string,
	warnings io.Writer,
) (mergedPlugin []plugin.Volume, mergedCustom []volPair, err error) {
	pathToSrc := map[string]volSrc{}

	mergedPlugin, err = mergePluginVolumes(pluginVols, pathToSrc, warnings)
	if err != nil {
		return nil, nil, err
	}

	pluginVolNames := map[string]struct{}{}
	for _, pv := range mergedPlugin {
		pluginVolNames[pv.VolumeName] = struct{}{}
	}

	mergedCustom, err = mergeCustomVolumes(customVols, pluginVolNames, pathToSrc, warnings)
	if err != nil {
		return nil, nil, err
	}
	return mergedPlugin, mergedCustom, nil
}

func mergePluginVolumes(
	pluginVols []plugin.Volume,
	pathToSrc map[string]volSrc,
	warnings io.Writer,
) ([]plugin.Volume, error) {
	// nameToPlugin tracks the first plugin that claimed a derived volume
	// name. DeriveVolumeName uses the path's basename, so two plugins with
	// different paths that share a basename (e.g. /foo/cache + /bar/cache)
	// would both emit the same `cache:` key in compose's volumes: section.
	// That collides at YAML render and at `docker compose up` time, so we
	// reject it at gen time.
	nameToPlugin := map[string]string{}
	out := make([]plugin.Volume, 0, len(pluginVols))
	for _, pv := range pluginVols {
		if _, reserved := reservedVolumeNames[pv.VolumeName]; reserved {
			return nil, fmt.Errorf(
				"%w: '%s' is reserved by cocoon (plugin '%s')",
				ErrVolumeNameConflict, pv.VolumeName, pv.PluginName)
		}
		if _, reserved := reservedMountPaths[pv.MountPath]; reserved {
			return nil, fmt.Errorf(
				"%w: plugin '%s' targets reserved mount path '%s'",
				ErrVolumeNameConflict, pv.PluginName, pv.MountPath)
		}
		if existing, dup := pathToSrc[pv.MountPath]; dup {
			if warnings != nil {
				fmt.Fprintf(warnings,
					"WARNING: Volume path '%s' is defined by both %s and plugin '%s'. Using single volume '%s'.\n",
					pv.MountPath, existing.label, pv.PluginName, existing.volName)
			}
			continue
		}
		if firstOwner, dup := nameToPlugin[pv.VolumeName]; dup {
			return nil, fmt.Errorf(
				"%w: plugins '%s' and '%s' both derive volume name '%s' "+
					"(rename one path so its basename differs)",
				ErrVolumeNameConflict, firstOwner, pv.PluginName, pv.VolumeName)
		}
		nameToPlugin[pv.VolumeName] = pv.PluginName
		pathToSrc[pv.MountPath] = volSrc{label: "plugin '" + pv.PluginName + "'", volName: pv.VolumeName}
		out = append(out, pv)
	}
	return out, nil
}

func mergeCustomVolumes(
	customVols map[string]string,
	pluginVolNames map[string]struct{},
	pathToSrc map[string]volSrc,
	warnings io.Writer,
) ([]volPair, error) {
	customNames := slices.Sorted(maps.Keys(customVols))
	for _, name := range customNames {
		if _, reserved := reservedVolumeNames[name]; reserved {
			return nil, fmt.Errorf(
				"%w: '%s' is reserved by cocoon (remove it from workspace.toml [volumes])",
				ErrVolumeNameConflict, name)
		}
		if _, conflict := pluginVolNames[name]; conflict {
			return nil, fmt.Errorf(
				"%w: '%s' (remove it from workspace.toml or disable the plugin)",
				ErrVolumeNameConflict, name)
		}
	}

	out := make([]volPair, 0, len(customNames))
	for _, name := range customNames {
		path := customVols[name]
		if _, reserved := reservedMountPaths[path]; reserved {
			return nil, fmt.Errorf(
				"%w: workspace.toml [volumes].%s targets reserved mount path '%s'",
				ErrVolumeNameConflict, name, path)
		}
		if existing, dup := pathToSrc[path]; dup {
			if warnings != nil {
				fmt.Fprintf(warnings,
					"WARNING: Volume path '%s' is defined by both %s and workspace.toml volume '%s'. Using single volume '%s'.\n",
					path, existing.label, name, existing.volName)
			}
			continue
		}
		pathToSrc[path] = volSrc{label: "workspace.toml volume '" + name + "'", volName: name}
		out = append(out, volPair{Name: name, Path: path})
	}
	return out, nil
}

type volPair struct {
	Name string
	Path string
}

// workspaceBindMount returns the host:container bind mount line for the
// workspace, choosing between cwd-only and parent-dir mounts based on
// [workspace] mount_root.
//
// docker-compose resolves bind mount relative paths against the
// directory that holds the compose file, not the current working
// directory. The generated compose file lives at
// .devcontainer/docker-compose.yml, so reaching the project root takes
// `..` and reaching the project's parent (the sibling-repos workspace
// requested by mount_root = "..") takes `../..`.
//
// The :cached flag is a no-op on Linux but is the macOS performance
// hint v1 used to set in the override compose file; keeping it in the
// generated output preserves that behaviour.
func workspaceBindMount(ctx *generate.WorkspaceContext) string {
	switch ctx.WS.Workspace.MountRootOrDefault() {
	case "..":
		return "../..:/home/${USERNAME}/workspace:cached"
	default:
		return "..:/home/${USERNAME}/workspace/" + ctx.ServiceName() + ":cached"
	}
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
		3+len(pluginVols)+len(customVols)+len(ctx.Mounts())+len(homeFiles),
	)
	mounts = append(mounts,
		yamlx.QuotedIfSpecial(workspaceBindMount(ctx)),
		yamlx.QuotedIfSpecial("local:/home/${USERNAME}/.local"),
		yamlx.QuotedIfSpecial("cocoon:/home/${USERNAME}/.cocoon"),
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
	// The compose file lives at .devcontainer/docker-compose.yml;
	// `context: ..` makes the project root the build context so the
	// Dockerfile's `COPY .devcontainer/docker-entrypoint.sh ...`
	// resolves. dockerfile is set explicitly because the default
	// (./Dockerfile) would point at the project root, not at our
	// generated .devcontainer/Dockerfile.
	buildPairs := []yamlx.Pair{
		{Key: "context", Value: yamlx.QuotedIfSpecial("..")},
		{Key: "dockerfile", Value: yamlx.QuotedIfSpecial(".devcontainer/Dockerfile")},
	}
	if ctx.CertificatesEnabled() {
		buildPairs = append(buildPairs, yamlx.Pair{
			Key: "additional_contexts",
			Value: yamlx.Map(yamlx.Pair{
				Key:   generate.CertsBuildContextName,
				Value: yamlx.QuotedIfSpecial(generate.CertsHostPath),
			}),
		})
	}
	buildPairs = append(buildPairs, yamlx.Pair{Key: "args", Value: yamlx.Seq(
		yamlx.QuotedIfSpecial("IMAGE=${IMAGE}"),
		yamlx.QuotedIfSpecial("IMAGE_VERSION=${IMAGE_VERSION}"),
		yamlx.QuotedIfSpecial("USERNAME=${USERNAME}"),
	)})

	pairs := []yamlx.Pair{
		{Key: "container_name", Value: yamlx.QuotedIfSpecial("${CONTAINER_SERVICE_NAME}")},
		{Key: "build", Value: yamlx.Map(buildPairs...)},
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
		for _, k := range slices.Sorted(maps.Keys(hosts)) {
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
		for _, k := range slices.Sorted(maps.Keys(sysctls)) {
			entries = append(entries, yamlx.Pair{Key: k, Value: sysctlNode(sysctls[k])})
		}
		pairs = append(pairs, yamlx.Pair{Key: "sysctls", Value: yamlx.Map(entries...)})
	}
	pairs = append(pairs, runtimeOptionPairs(ctx)...)
	if ctx.WS.Workspace.MountRootOrDefault() == "." {
		// `mount_root = "."` mounts only the project so the host bind point
		// already includes the project basename. Emit a matching
		// working_dir so `cocoon exec` lands inside the project tree
		// rather than at the parent /home/<user>/workspace WORKDIR baked
		// into the image.
		pairs = append(pairs, yamlx.Pair{
			Key:   "working_dir",
			Value: yamlx.QuotedIfSpecial("/home/${USERNAME}/workspace/" + ctx.ServiceName()),
		})
	}
	pairs = append(pairs, yamlx.Pair{Key: "tty", Value: yamlx.Bool(true)})
	pairs = append(pairs, yamlx.Pair{Key: "init", Value: yamlx.Bool(true)})
	pairs = append(pairs, yamlx.Pair{Key: "command", Value: yamlx.QuotedIfSpecial("sleep infinity")})
	pairs = append(pairs, applyResources(ctx)...)
	return yamlx.Map(pairs...)
}

// runtimeOptionPairs emits the optional capability / security / device /
// namespace service fields in a fixed order so generated YAML is
// deterministic. Each entry is omitted when its source section is unset.
func runtimeOptionPairs(ctx *generate.WorkspaceContext) []yamlx.Pair {
	pairs := make([]yamlx.Pair, 0, 7)
	if add := ctx.CapAdd(); len(add) > 0 {
		pairs = append(pairs, yamlx.Pair{Key: "cap_add", Value: stringSeq(add)})
	}
	if drop := ctx.CapDrop(); len(drop) > 0 {
		pairs = append(pairs, yamlx.Pair{Key: "cap_drop", Value: stringSeq(drop)})
	}
	if sec := ctx.SecurityOptions(); len(sec) > 0 {
		pairs = append(pairs, yamlx.Pair{Key: "security_opt", Value: stringSeq(sec)})
	}
	if g := ctx.GroupAdd(); len(g) > 0 {
		pairs = append(pairs, yamlx.Pair{Key: "group_add", Value: stringSeq(g)})
	}
	if dev := ctx.Devices(); len(dev) > 0 {
		pairs = append(pairs, yamlx.Pair{Key: "devices", Value: stringSeq(dev)})
	}
	if ipc := ctx.IPC(); ipc != "" {
		pairs = append(pairs, yamlx.Pair{Key: "ipc", Value: yamlx.QuotedIfSpecial(ipc)})
	}
	if gpus := ctx.Gpus(); gpus != "" {
		pairs = append(pairs, yamlx.Pair{Key: "gpus", Value: yamlx.QuotedIfSpecial(gpus)})
	}
	return pairs
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
	out := make([]yamlx.Pair, 0, 2+len(plugin)+len(custom))
	out = append(out,
		yamlx.Pair{
			Key:   "local",
			Value: namedVolume("${COMPOSE_PROJECT_NAME}_${CONTAINER_SERVICE_NAME}_local"),
		},
		yamlx.Pair{
			Key:   "cocoon",
			Value: namedVolume("${COMPOSE_PROJECT_NAME}_${CONTAINER_SERVICE_NAME}_cocoon"),
		},
	)
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

// sysctlNode renders numerics as int, everything else as a quoted string.
// Validation already rejected unsupported types.
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
