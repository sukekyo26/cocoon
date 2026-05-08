package clean

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/sukekyo26/cocoon/internal/exec"
	"github.com/sukekyo26/cocoon/internal/exec/dockerx"
)

// errUnknownPruneKind is returned by execPruner.Prune for invalid PruneKind values.
//
//nolint:gochecknoglobals // sentinel error value.
var errUnknownPruneKind = errors.New("clean: unknown prune kind")

// NewExecDocker returns a DockerClient backed by the docker CLI.
func NewExecDocker() DockerClient { return execDocker{client: dockerx.New(exec.New())} }

// NewExecPruner returns a SystemPruner backed by the docker CLI.
func NewExecPruner() SystemPruner { return execPruner{client: dockerx.New(exec.New())} }

// NewExecDockerWithRunner is like [NewExecDocker] but takes an explicit runner
// so tests can inject [exec.RecordingRunner].
func NewExecDockerWithRunner(r exec.Runner) DockerClient {
	return execDocker{client: dockerx.New(r)}
}

// NewExecPrunerWithRunner is like [NewExecPruner] but takes an explicit runner
// so tests can inject [exec.RecordingRunner].
func NewExecPrunerWithRunner(r exec.Runner) SystemPruner {
	return execPruner{client: dockerx.New(r)}
}

type execDocker struct{ client *dockerx.Client }

type execPruner struct{ client *dockerx.Client }

// SystemDF runs `docker system df` and writes the output to w.
func (p execPruner) SystemDF(ctx context.Context, w io.Writer) error {
	return p.client.SystemDF(ctx, w) //nolint:wrapcheck // dockerx already prefixes the error.
}

// Prune runs the appropriate `docker <kind> prune -f` command.
func (p execPruner) Prune(ctx context.Context, kind PruneKind, w io.Writer) error {
	dx, err := pruneKindToDockerx(kind)
	if err != nil {
		return err
	}
	return p.client.Prune(ctx, dx, w) //nolint:wrapcheck // dockerx already prefixes the error.
}

// Info runs `docker info` and returns nil if the daemon responded.
func (d execDocker) Info(ctx context.Context) error {
	return d.client.Info(ctx) //nolint:wrapcheck // dockerx already prefixes the error.
}

// VolumeNames returns the names of all docker volumes on the host.
func (d execDocker) VolumeNames(ctx context.Context) ([]string, error) {
	return d.client.VolumeNames(ctx) //nolint:wrapcheck // dockerx already prefixes the error.
}

// VolumeRemove deletes a single docker volume by name.
func (d execDocker) VolumeRemove(ctx context.Context, name string) error {
	return d.client.VolumeRemove(ctx, name) //nolint:wrapcheck // dockerx already prefixes the error.
}

// ContainersByProject returns container IDs labeled for the given compose project.
func (d execDocker) ContainersByProject(ctx context.Context, project string) ([]string, error) {
	return d.client.ContainersByProject(ctx, project) //nolint:wrapcheck // dockerx already prefixes the error.
}

// ContainerForceRemove force-removes the given container IDs.
func (d execDocker) ContainerForceRemove(ctx context.Context, ids []string) error {
	return d.client.ContainerForceRemove(ctx, ids) //nolint:wrapcheck // dockerx already prefixes the error.
}

func pruneKindToDockerx(k PruneKind) (dockerx.PruneKind, error) {
	switch k {
	case PruneContainers:
		return dockerx.PruneContainers, nil
	case PruneBuilder:
		return dockerx.PruneBuilder, nil
	case PruneImages:
		return dockerx.PruneImages, nil
	case PruneNetworks:
		return dockerx.PruneNetworks, nil
	case PruneVolumes:
		return dockerx.PruneVolumes, nil
	default:
		return 0, fmt.Errorf("%w: %d", errUnknownPruneKind, int(k))
	}
}
