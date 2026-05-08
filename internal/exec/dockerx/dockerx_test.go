package dockerx_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/sukekyo26/cocoon/internal/exec"
	"github.com/sukekyo26/cocoon/internal/exec/dockerx"
)

func TestInfoBuildsCorrectArgs(t *testing.T) {
	t.Parallel()
	r := exec.NewRecordingRunner()
	c := dockerx.New(r)
	if err := c.Info(context.Background()); err != nil {
		t.Fatalf("Info: %v", err)
	}
	if len(r.Calls) != 1 || r.Calls[0].Name != "docker" {
		t.Fatalf("expected one docker call, got %+v", r.Calls)
	}
	if got := r.Calls[0].Args; len(got) != 1 || got[0] != "info" {
		t.Errorf("args = %v, want [info]", got)
	}
}

func TestVolumeNamesParsesLines(t *testing.T) {
	t.Parallel()
	r := exec.NewRecordingRunner()
	r.Stub("docker", []string{"volume", "ls", "--format", "{{.Name}}"},
		exec.Stub{Stdout: []byte("a\nb\nc\n")})
	c := dockerx.New(r)
	got, err := c.VolumeNames(context.Background())
	if err != nil {
		t.Fatalf("VolumeNames: %v", err)
	}
	if len(got) != 3 || got[0] != "a" || got[2] != "c" {
		t.Errorf("got %v", got)
	}
}

func TestContainersByProjectInjectsLabel(t *testing.T) {
	t.Parallel()
	r := exec.NewRecordingRunner()
	c := dockerx.New(r)
	if _, err := c.ContainersByProject(context.Background(), "myproj"); err != nil {
		t.Fatalf("ContainersByProject: %v", err)
	}
	args := r.Calls[0].Args
	want := []string{"ps", "-aq", "--filter", "label=com.docker.compose.project=myproj"}
	if !equal(args, want) {
		t.Errorf("args = %v, want %v", args, want)
	}
}

func TestContainerForceRemoveSkipsEmpty(t *testing.T) {
	t.Parallel()
	r := exec.NewRecordingRunner()
	c := dockerx.New(r)
	if err := c.ContainerForceRemove(context.Background(), nil); err != nil {
		t.Fatalf("ContainerForceRemove: %v", err)
	}
	if len(r.Calls) != 0 {
		t.Errorf("no call should be made for empty ids; got %+v", r.Calls)
	}
}

func TestRunBashPassesScriptVerbatim(t *testing.T) {
	t.Parallel()
	r := exec.NewRecordingRunner()
	r.Stub("docker", []string{
		"run", "--rm", "--entrypoint", "/bin/bash",
		"img:tag", "-c", "echo hi",
	}, exec.Stub{Stdout: []byte("hi\n")})
	c := dockerx.New(r)
	got, err := c.RunBash(context.Background(), "img:tag", "echo hi")
	if err != nil {
		t.Fatalf("RunBash: %v", err)
	}
	if got != "hi" {
		t.Errorf("got %q, want hi", got)
	}
}

func TestComposeConfigPassesFile(t *testing.T) {
	t.Parallel()
	r := exec.NewRecordingRunner()
	c := dockerx.New(r)
	if err := c.ComposeConfig(context.Background(), "compose.yaml"); err != nil {
		t.Fatalf("ComposeConfig: %v", err)
	}
	want := []string{"compose", "-f", "compose.yaml", "config", "-q"}
	if !equal(r.Calls[0].Args, want) {
		t.Errorf("args = %v, want %v", r.Calls[0].Args, want)
	}
}

func TestImageInspectFormatTrimsOutput(t *testing.T) {
	t.Parallel()
	r := exec.NewRecordingRunner()
	r.Stub("docker", []string{"image", "inspect", "img", "--format", "{{.Created}}"},
		exec.Stub{Stdout: []byte("2026-04-26T11:45:00Z\n")})
	c := dockerx.New(r)
	got, err := c.ImageInspectFormat(context.Background(), "img", "{{.Created}}")
	if err != nil {
		t.Fatalf("ImageInspectFormat: %v", err)
	}
	if got != "2026-04-26T11:45:00Z" {
		t.Errorf("got %q", got)
	}
}

func TestSystemDFRoutesStdout(t *testing.T) {
	t.Parallel()
	r := exec.NewRecordingRunner()
	r.Stub("docker", []string{"system", "df"}, exec.Stub{Stdout: []byte("TYPE TOTAL\n")})
	c := dockerx.New(r)
	var buf bytes.Buffer
	if err := c.SystemDF(context.Background(), &buf); err != nil {
		t.Fatalf("SystemDF: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("TYPE")) {
		t.Errorf("stdout not routed: %q", buf.String())
	}
}

func TestPruneUnknownKindErrors(t *testing.T) {
	t.Parallel()
	r := exec.NewRecordingRunner()
	c := dockerx.New(r)
	err := c.Prune(context.Background(), dockerx.PruneKind(99), nil)
	if err == nil {
		t.Fatal("expected error for unknown PruneKind")
	}
}

var errStub = errors.New("docker daemon not reachable")

func TestErrorIsWrapped(t *testing.T) {
	t.Parallel()
	r := exec.NewRecordingRunner()
	r.Stub("docker", []string{"info"}, exec.Stub{Err: errStub})
	c := dockerx.New(r)
	err := c.Info(context.Background())
	if !errors.Is(err, errStub) {
		t.Errorf("expected wrapped errStub; got %v", err)
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
