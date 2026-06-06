package lockcli

import (
	"context"
	"errors"
	"maps"

	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/generate"
	"github.com/sukekyo26/cocoon/internal/i18n"
	"github.com/sukekyo26/cocoon/internal/lockfile"
	"github.com/sukekyo26/cocoon/internal/logx"
	"github.com/sukekyo26/cocoon/internal/plugin/resolve"
)

// pluginSpec is one enabled version_capable plugin's resolution input.
type pluginSpec struct {
	id        string
	requested string // "latest" or "=1.2.3"
	override  config.PluginVersionOverride
}

// requestedSpecs returns the enabled version_capable plugins in enable order,
// each with its requested constraint ("latest" when the plugin is unpinned).
func requestedSpecs(wctx *generate.WorkspaceContext) []pluginSpec {
	var out []pluginSpec
	for _, id := range wctx.WS.Plugins.Enable {
		p, ok := wctx.Plugins[id]
		if !ok || !p.Version.VersionCapable {
			continue
		}
		ov := wctx.WS.Plugins.Versions[id]
		spec := config.VersionSpecLatest
		if ov.Spec != "" {
			spec = ov.Spec
		}
		out = append(out, pluginSpec{id: id, requested: spec, override: ov})
	}
	return out
}

func specHashInput(specs []pluginSpec) map[string]string {
	m := make(map[string]string, len(specs))
	for _, s := range specs {
		m[s.id] = s.requested
	}
	return m
}

// checkLock verifies (offline) that the lock matches workspace.toml: present,
// inputs_hash current, and an entry for every enabled plugin whose requested
// spec and [plugins.options] extras still match. (A manual [plugins.options]
// checksum is not compared: it is never recorded in the lock and gen bakes it
// live from the workspace, so it cannot go stale.) Any drift is a usage error
// so CI can gate on it. A workspace with no version-capable plugins needs no
// lock, so --check passes there even when the file is absent.
func checkLock(
	log *logx.Logger, cat *i18n.Catalog, lockPath string, existing *lockfile.Lock, specs []pluginSpec,
) error {
	if len(specs) == 0 {
		log.Success(cat.Msg("lock_nothing_to_lock"))
		return nil
	}
	if existing == nil {
		return clihelpers.UsageErr("err_lock_lock_missing", lockPath)
	}
	if existing.InputsHash != lockfile.ComputeInputsHash(specHashInput(specs)) {
		return clihelpers.UsageErr("err_lock_out_of_date", lockPath)
	}
	for _, s := range specs {
		entry, ok := existing.Find(s.id)
		if !ok {
			return clihelpers.UsageErr("err_lock_no_entry", lockPath, s.id)
		}
		if entry.Requested != s.requested {
			return clihelpers.UsageErr("err_lock_requested_mismatch", lockPath, s.id, entry.Requested, s.requested)
		}
		if !maps.Equal(entry.Extra, s.override.Extra) {
			return clihelpers.UsageErr("err_lock_stale_options", lockPath, s.id)
		}
	}
	log.Success(cat.Msg("lock_up_to_date", lockPath, len(specs)))
	return nil
}

// buildLock resolves each plugin (reusing an unchanged existing entry unless
// --upgrade forces a floating re-resolution) and assembles the new lock.
func buildLock(
	ctx context.Context,
	wctx *generate.WorkspaceContext,
	specs []pluginSpec,
	existing *lockfile.Lock,
	upgrade bool,
	log *logx.Logger,
	cat *i18n.Catalog,
) (*lockfile.Lock, error) {
	resolver := resolve.New(defaultFetcher)
	entries := make([]lockfile.LockPlugin, 0, len(specs))
	for _, s := range specs {
		if reused, ok := reuseEntry(existing, s, upgrade); ok {
			// The pinned version + checksums are still valid, but the
			// [plugins.options] knobs (Extra) are not network-resolved — refresh
			// them from the current workspace so a changed option is reflected in
			// the output lock without forcing a re-resolution.
			reused.Extra = s.override.Extra
			entries = append(entries, reused)
			log.Success(cat.Msg("lock_reused", s.id, reused.Version))
			continue
		}
		res, err := resolver.Resolve(ctx, resolve.Request{
			ID:       s.id,
			Source:   wctx.Plugins[s.id].Version.Source,
			Version:  s.override.Pin, // concrete for "=x.y.z", "" for "latest"
			IsLatest: s.requested == config.VersionSpecLatest,
			Arches:   []string{"amd64", "arm64"},
		})
		if err != nil {
			if errors.Is(err, resolve.ErrLatestUnsupported) {
				return nil, clihelpers.UsageErr("err_lock_latest_unsupported", s.id, s.id)
			}
			return nil, clihelpers.FailureWrap(err, "")
		}
		entries = append(entries, toLockPlugin(s, res))
		log.Success(cat.Msg("lock_locked", s.id, res.Version))
	}
	return &lockfile.Lock{
		LockVersion: lockfile.Version,
		InputsHash:  lockfile.ComputeInputsHash(specHashInput(specs)),
		Plugins:     entries,
	}, nil
}

// reuseEntry returns an existing lock entry when it still matches the request
// and need not be re-resolved: exact pins are fixed by definition, and a
// floating "latest" is reused unless --upgrade asks to refresh it.
func reuseEntry(existing *lockfile.Lock, s pluginSpec, upgrade bool) (lockfile.LockPlugin, bool) {
	if existing == nil {
		return lockfile.LockPlugin{}, false //nolint:exhaustruct // not-found
	}
	entry, ok := existing.Find(s.id)
	if !ok || entry.Requested != s.requested {
		return lockfile.LockPlugin{}, false //nolint:exhaustruct // not-found
	}
	if upgrade && s.requested == config.VersionSpecLatest {
		return lockfile.LockPlugin{}, false //nolint:exhaustruct // forced refresh
	}
	return entry, true
}

func toLockPlugin(s pluginSpec, res resolve.Resolved) lockfile.LockPlugin {
	return lockfile.LockPlugin{
		ID:            s.id,
		Requested:     s.requested,
		Version:       res.Version,
		ChecksumAMD64: derefOr(res.ChecksumAMD64),
		ChecksumARM64: derefOr(res.ChecksumARM64),
		Extra:         s.override.Extra,
	}
}

func derefOr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
