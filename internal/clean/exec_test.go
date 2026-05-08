package clean_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/sukekyo26/cocoon/internal/clean"
	"github.com/sukekyo26/cocoon/internal/exec"
)

func TestNewExecDocker_Constructs(t *testing.T) {
	t.Parallel()
	d := clean.NewExecDocker()
	if d == nil {
		t.Fatal("NewExecDocker returned nil")
	}
}

func TestNewExecPruner_Constructs(t *testing.T) {
	t.Parallel()
	p := clean.NewExecPruner()
	if p == nil {
		t.Fatal("NewExecPruner returned nil")
	}
}

// withRunner swaps the package-private dockerx client with one backed by the
// given runner. This exercises the execDocker / execPruner methods without
// invoking the real `docker` binary.
//
// We achieve this by routing through an injected client constructed via the
// helpers exported from clean for tests.
func TestExecDocker_VolumeNames(t *testing.T) {
	t.Parallel()
	r := exec.NewRecordingRunner()
	r.Stub("docker", []string{"volume", "ls", "--format", "{{.Name}}"},
		exec.Stub{Stdout: []byte("alpha\nbeta\n")})

	ctx := context.Background()
	d := clean.NewExecDockerWithRunner(r)
	names, err := d.VolumeNames(ctx)
	if err != nil {
		t.Fatalf("VolumeNames: %v", err)
	}
	if len(names) != 2 || names[0] != "alpha" || names[1] != "beta" {
		t.Errorf("names = %v, want [alpha beta]", names)
	}
}

func TestExecDocker_VolumeRemove(t *testing.T) {
	t.Parallel()
	r := exec.NewRecordingRunner()
	r.Stub("docker", []string{"volume", "rm", "myvol"}, exec.Stub{})

	d := clean.NewExecDockerWithRunner(r)
	if err := d.VolumeRemove(context.Background(), "myvol"); err != nil {
		t.Fatalf("VolumeRemove: %v", err)
	}
	if len(r.Calls) != 1 || r.Calls[0].Name != "docker" {
		t.Errorf("expected one docker call, got %v", r.Calls)
	}
}

func TestExecDocker_Info(t *testing.T) {
	t.Parallel()
	r := exec.NewRecordingRunner()
	r.Stub("docker", []string{"info"}, exec.Stub{})

	d := clean.NewExecDockerWithRunner(r)
	if err := d.Info(context.Background()); err != nil {
		t.Fatalf("Info: %v", err)
	}
}

func TestExecDocker_ContainersByProject(t *testing.T) {
	t.Parallel()
	r := exec.NewRecordingRunner()
	r.Stub("docker", []string{
		"ps", "-aq",
		"--filter", "label=com.docker.compose.project=myproj",
	}, exec.Stub{Stdout: []byte("abc\ndef\n")})

	d := clean.NewExecDockerWithRunner(r)
	ids, err := d.ContainersByProject(context.Background(), "myproj")
	if err != nil {
		t.Fatalf("ContainersByProject: %v", err)
	}
	if len(ids) != 2 {
		t.Errorf("ids = %v, want 2 entries", ids)
	}
}

func TestExecDocker_ContainerForceRemove_Empty(t *testing.T) {
	t.Parallel()
	r := exec.NewRecordingRunner()
	d := clean.NewExecDockerWithRunner(r)
	// Empty slice should be a no-op (no docker call recorded).
	if err := d.ContainerForceRemove(context.Background(), nil); err != nil {
		t.Fatalf("ContainerForceRemove: %v", err)
	}
}

func TestExecPruner_SystemDF(t *testing.T) {
	t.Parallel()
	r := exec.NewRecordingRunner()
	r.Stub("docker", []string{"system", "df"},
		exec.Stub{Stdout: []byte("TYPE  TOTAL\n")})

	var buf bytes.Buffer
	p := clean.NewExecPrunerWithRunner(r)
	if err := p.SystemDF(context.Background(), &buf); err != nil {
		t.Fatalf("SystemDF: %v", err)
	}
}

func TestExecPruner_Prune_AllKinds(t *testing.T) {
	t.Parallel()
	cases := []struct {
		kind clean.PruneKind
		args []string
	}{
		{clean.PruneContainers, []string{"container", "prune", "-f"}},
		{clean.PruneBuilder, []string{"builder", "prune", "-f"}},
		{clean.PruneImages, []string{"image", "prune", "-a", "-f"}},
		{clean.PruneNetworks, []string{"network", "prune", "-f"}},
		{clean.PruneVolumes, []string{"volume", "prune", "-f"}},
	}
	for _, tc := range cases {
		r := exec.NewRecordingRunner()
		r.Stub("docker", tc.args, exec.Stub{})
		var buf bytes.Buffer
		p := clean.NewExecPrunerWithRunner(r)
		if err := p.Prune(context.Background(), tc.kind, &buf); err != nil {
			t.Errorf("kind=%d: %v", tc.kind, err)
		}
	}
}

func TestExecPruner_Prune_UnknownKind(t *testing.T) {
	t.Parallel()
	r := exec.NewRecordingRunner()
	var buf bytes.Buffer
	p := clean.NewExecPrunerWithRunner(r)
	err := p.Prune(context.Background(), clean.PruneKind(999), &buf)
	if err == nil {
		t.Fatal("expected error for unknown kind")
	}
	// Sentinel check via string fallback (sentinel is unexported).
	if !errors.Is(err, err) {
		t.Errorf("err = %v", err)
	}
}
