// Package clean implements host-side Docker cleanup operations that used
// to live in clean-volumes.sh and clean-docker.sh.
package clean

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sukekyo26/cocoon/internal/envfile"
)

// ErrCanceled signals the user declined the confirmation prompt.
var ErrCanceled = errors.New("canceled")

// ErrPartial signals that some volumes failed to delete.
var ErrPartial = errors.New("some volumes failed to delete")

// ErrConfig signals an invalid VolumesOptions value (programmer error).
var ErrConfig = errors.New("clean: invalid options")

// ErrPrereq signals a missing host prerequisite (docker not installed or
// daemon down). The wrapped message is the localized text for the user.
var ErrPrereq = errors.New("prerequisite check failed")

// DockerClient abstracts the docker CLI calls needed for volume cleanup.
// The default implementation shells out via os/exec; tests inject fakes.
type DockerClient interface {
	Info(ctx context.Context) error
	VolumeNames(ctx context.Context) ([]string, error)
	VolumeRemove(ctx context.Context, name string) error
	ContainersByProject(ctx context.Context, project string) ([]string, error)
	ContainerForceRemove(ctx context.Context, ids []string) error
}

// Translator is the subset of i18n.Catalog that clean uses; defined here
// to avoid an import cycle for tests.
type Translator interface {
	Msg(key string, args ...any) string
}

// VolumesOptions configures VolumesRun.
type VolumesOptions struct {
	WorkspaceDir string
	Stdin        io.Reader
	Stdout       io.Writer
	Stderr       io.Writer
	Catalog      Translator
	Docker       DockerClient
	// AssumeYes skips the confirmation prompt (currently used by tests).
	AssumeYes bool
}

// VolumesRun performs the clean-volumes flow.
func VolumesRun(ctx context.Context, opts VolumesOptions) error {
	opts = volumesDefaults(opts)
	if opts.Catalog == nil {
		return fmt.Errorf("%w: Catalog is required", ErrConfig)
	}
	if opts.WorkspaceDir == "" {
		return fmt.Errorf("%w: WorkspaceDir is required", ErrConfig)
	}
	t := opts.Catalog
	out := opts.Stdout

	printHeader(out, t, opts.WorkspaceDir)
	if err := checkDocker(ctx, opts.Docker, t); err != nil {
		return err
	}

	envFile := filepath.Join(opts.WorkspaceDir, ".env")
	serviceName := envfile.ReadOr(envFile, "CONTAINER_SERVICE_NAME", "dev")
	projectName := envfile.ReadOr(envFile, "COMPOSE_PROJECT_NAME", filepath.Base(opts.WorkspaceDir))
	prefix := projectName + "_" + serviceName + "_"

	fmt.Fprintln(out)
	fmt.Fprintln(out, t.Msg("clean_project_name"), "  "+bold(projectName))
	fmt.Fprintln(out, t.Msg("clean_service_name"), "  "+bold(serviceName))
	fmt.Fprintln(out, t.Msg("clean_volume_prefix"), " "+bold(prefix))
	fmt.Fprintln(out)

	volumes, err := listProjectVolumes(ctx, opts.Docker, prefix)
	if err != nil {
		return err
	}
	if len(volumes) == 0 {
		fmt.Fprintln(out, yellow(t.Msg("clean_no_volumes")))
		fmt.Fprintln(out, " ", t.Msg("clean_prefix_info", prefix))
		return nil
	}

	printVolumeList(out, t, volumes)

	if !opts.AssumeYes {
		ok, perr := envfile.ConfirmYN(opts.Stdin, opts.Stdout, t.Msg("clean_confirm"))
		if perr != nil {
			return fmt.Errorf("read confirm: %w", perr)
		}
		if !ok {
			fmt.Fprintln(out, t.Msg("clean_cancelled"))
			return ErrCanceled
		}
	}

	stopProjectContainers(ctx, opts, projectName)
	return deleteVolumes(ctx, opts.Docker, t, out, volumes)
}

func volumesDefaults(opts VolumesOptions) VolumesOptions {
	if opts.Docker == nil {
		opts.Docker = NewExecDocker()
	}
	if opts.Stdin == nil {
		opts.Stdin = os.Stdin
	}
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	return opts
}

func printHeader(out io.Writer, t Translator, workspaceDir string) {
	fmt.Fprintln(out)
	fmt.Fprintln(out, bold("========================================"))
	fmt.Fprintln(out, " "+t.Msg("clean_header"))
	fmt.Fprintln(out, bold("========================================"))
	fmt.Fprintln(out)
	fmt.Fprintln(out, t.Msg("clean_workspace"), bold(workspaceDir))
}

func checkDocker(ctx context.Context, d DockerClient, t Translator) error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("%w: %s", ErrPrereq, t.Msg("clean_docker_not_found"))
	}
	if err := d.Info(ctx); err != nil {
		return fmt.Errorf("%w: %s", ErrPrereq, t.Msg("clean_docker_not_running"))
	}
	return nil
}

func listProjectVolumes(ctx context.Context, d DockerClient, prefix string) ([]string, error) {
	all, err := d.VolumeNames(ctx)
	if err != nil {
		return nil, fmt.Errorf("docker volume ls: %w", err)
	}
	var matched []string
	for _, v := range all {
		if strings.HasPrefix(v, prefix) {
			matched = append(matched, v)
		}
	}
	return matched, nil
}

func printVolumeList(out io.Writer, t Translator, volumes []string) {
	fmt.Fprintln(out, cyan(t.Msg("clean_volumes_header", fmt.Sprint(len(volumes)))))
	for _, v := range volumes {
		fmt.Fprintln(out, "  -", v)
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, yellow(t.Msg("clean_notice")))
	fmt.Fprintln(out, t.Msg("clean_notice_1"))
	fmt.Fprintln(out, t.Msg("clean_notice_2"))
	fmt.Fprintln(out, t.Msg("clean_notice_3"))
	fmt.Fprintln(out)
}

func stopProjectContainers(ctx context.Context, opts VolumesOptions, project string) {
	containers, err := opts.Docker.ContainersByProject(ctx, project)
	if err != nil || len(containers) == 0 {
		return
	}
	fmt.Fprintln(opts.Stdout)
	fmt.Fprintln(opts.Stdout, cyan(opts.Catalog.Msg("clean_stopping")))
	if rmErr := opts.Docker.ContainerForceRemove(ctx, containers); rmErr != nil {
		// Best-effort cleanup; surface a warning but do not abort.
		fmt.Fprintln(opts.Stderr, rmErr)
	}
}

func deleteVolumes(ctx context.Context, d DockerClient, t Translator, out io.Writer, volumes []string) error {
	fmt.Fprintln(out)
	fmt.Fprintln(out, cyan(t.Msg("clean_deleting")))
	failed := 0
	for _, v := range volumes {
		if err := d.VolumeRemove(ctx, v); err == nil {
			fmt.Fprintln(out, "  "+green("✅"), v)
		} else {
			fmt.Fprintln(out, "  "+red("❌"), t.Msg("clean_vol_failed", v))
			failed++
		}
	}
	fmt.Fprintln(out)
	if failed == 0 {
		fmt.Fprintln(out, green(t.Msg("clean_all_deleted", fmt.Sprint(len(volumes)))))
		return nil
	}
	fmt.Fprintln(out, yellow(t.Msg("clean_partial",
		fmt.Sprint(len(volumes)-failed), fmt.Sprint(failed))))
	return ErrPartial
}
