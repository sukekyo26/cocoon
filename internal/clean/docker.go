package clean

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// PruneKind enumerates the docker prune operations supported by DockerCleanRun.
type PruneKind int

const (
	// PruneContainers corresponds to `docker container prune -f`.
	PruneContainers PruneKind = iota
	// PruneBuilder corresponds to `docker builder prune -f`.
	PruneBuilder
	// PruneImages corresponds to `docker image prune -f`.
	PruneImages
	// PruneNetworks corresponds to `docker network prune -f`.
	PruneNetworks
	// PruneVolumes corresponds to `docker volume prune -f`.
	PruneVolumes
)

// SystemPruner abstracts the docker prune / system df calls. The default
// implementation shells out via os/exec; tests inject fakes.
type SystemPruner interface {
	SystemDF(ctx context.Context, w io.Writer) error
	Prune(ctx context.Context, kind PruneKind, w io.Writer) error
}

// MultiSelector chooses zero or more cleanup operations to run.
type MultiSelector interface {
	SelectMulti(title string, options []string, preselected []int) ([]int, error)
}

// DockerOptions configures DockerCleanRun.
type DockerOptions struct {
	Stdout   io.Writer
	Stderr   io.Writer
	Catalog  Translator
	Pruner   SystemPruner
	Selector MultiSelector
}

// pruneSpec maps an enum value onto its localized message keys.
type pruneSpec struct {
	kind       PruneKind
	optionKey  string
	runningKey string
	doneKey    string
	failKey    string
}

//nolint:gochecknoglobals // table-driven specs scoped to this package.
var pruneSpecs = []pruneSpec{
	{
		PruneContainers,
		"docker_clean_opt_containers",
		"docker_clean_running_containers",
		"docker_clean_done_containers",
		"docker_clean_fail_containers",
	},
	{
		PruneBuilder,
		"docker_clean_opt_builder",
		"docker_clean_running_builder",
		"docker_clean_done_builder",
		"docker_clean_fail_builder",
	},
	{
		PruneImages,
		"docker_clean_opt_images",
		"docker_clean_running_images",
		"docker_clean_done_images",
		"docker_clean_fail_images",
	},
	{
		PruneNetworks,
		"docker_clean_opt_networks",
		"docker_clean_running_networks",
		"docker_clean_done_networks",
		"docker_clean_fail_networks",
	},
	{
		PruneVolumes,
		"docker_clean_opt_volumes",
		"docker_clean_running_volumes",
		"docker_clean_done_volumes",
		"docker_clean_fail_volumes",
	},
}

// DockerCleanRun performs the interactive docker cleanup flow.
func DockerCleanRun(ctx context.Context, opts DockerOptions) error {
	opts = dockerCleanDefaults(opts)
	if opts.Catalog == nil {
		return fmt.Errorf("%w: Catalog is required", ErrConfig)
	}
	t := opts.Catalog
	out := opts.Stdout

	printDockerHeader(out, t)
	if err := pingDocker(ctx, opts.Pruner, t); err != nil {
		return err
	}

	fmt.Fprintln(out, t.Msg("docker_clean_disk_usage"))
	fmt.Fprintln(out)
	if err := opts.Pruner.SystemDF(ctx, out); err != nil {
		fmt.Fprintln(opts.Stderr, err)
	}
	fmt.Fprintln(out)

	options := make([]string, len(pruneSpecs))
	for i, s := range pruneSpecs {
		options[i] = t.Msg(s.optionKey)
	}
	preselected := []int{0, 1, 2}
	picked, err := opts.Selector.SelectMulti(t.Msg("docker_clean_select_title"), options, preselected)
	if err != nil {
		fmt.Fprintln(out, t.Msg("docker_clean_cancelled"))
		return ErrCanceled
	}
	if len(picked) == 0 {
		fmt.Fprintln(out, t.Msg("docker_clean_cancelled"))
		return nil
	}

	fmt.Fprintln(out)
	success, fail := runPruneOps(ctx, opts, picked)

	fmt.Fprintln(out, t.Msg("docker_clean_disk_usage_after"))
	fmt.Fprintln(out)
	if dfErr := opts.Pruner.SystemDF(ctx, out); dfErr != nil {
		fmt.Fprintln(opts.Stderr, dfErr)
	}
	fmt.Fprintln(out)

	if fail == 0 {
		fmt.Fprintln(out, t.Msg("docker_clean_all_done", success))
		return nil
	}
	fmt.Fprintln(out, t.Msg("docker_clean_partial_done", success, fail))
	return ErrPartial
}

func runPruneOps(ctx context.Context, opts DockerOptions, picked []int) (success, fail int) {
	t := opts.Catalog
	out := opts.Stdout
	for _, idx := range picked {
		if idx < 0 || idx >= len(pruneSpecs) {
			continue
		}
		spec := pruneSpecs[idx]
		fmt.Fprintln(out, t.Msg(spec.runningKey))
		if err := opts.Pruner.Prune(ctx, spec.kind, out); err != nil {
			fmt.Fprintln(out, t.Msg(spec.failKey))
			fail++
		} else {
			fmt.Fprintln(out, t.Msg(spec.doneKey))
			success++
		}
		fmt.Fprintln(out)
	}
	return success, fail
}

func printDockerHeader(out io.Writer, t Translator) {
	fmt.Fprintln(out)
	fmt.Fprintln(out, "========================================")
	fmt.Fprintln(out, " "+t.Msg("docker_clean_header"))
	fmt.Fprintln(out, "========================================")
	fmt.Fprintln(out)
}

func pingDocker(ctx context.Context, p SystemPruner, t Translator) error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("%w: %s", ErrPrereq, t.Msg("docker_clean_not_found"))
	}
	if err := p.SystemDF(ctx, io.Discard); err != nil {
		// Treat any failure as daemon-not-running for parity with the bash version.
		var execErr *exec.Error
		if errors.As(err, &execErr) {
			return fmt.Errorf("%w: %s", ErrPrereq, t.Msg("docker_clean_not_found"))
		}
		return fmt.Errorf("%w: %s", ErrPrereq, t.Msg("docker_clean_not_running"))
	}
	return nil
}

func dockerCleanDefaults(opts DockerOptions) DockerOptions {
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	if opts.Pruner == nil {
		opts.Pruner = NewExecPruner()
	}
	return opts
}
