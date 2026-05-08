package dockerx_test

import (
	"context"
	"testing"

	"github.com/sukekyo26/cocoon/internal/exec"
	"github.com/sukekyo26/cocoon/internal/exec/dockerx"
)

func TestInfoCombinedOutput(t *testing.T) {
	t.Parallel()
	r := exec.NewRecordingRunner()
	r.Stub("docker", []string{"info"}, exec.Stub{
		Stdout: []byte("OK\n"),
		Stderr: []byte("warn\n"),
	})
	c := dockerx.New(r)
	out, err := c.InfoCombinedOutput(context.Background())
	if err != nil {
		t.Fatalf("InfoCombinedOutput: %v", err)
	}
	if len(out) == 0 {
		t.Errorf("expected non-empty output")
	}
}

func TestVolumeRemove(t *testing.T) {
	t.Parallel()
	r := exec.NewRecordingRunner()
	r.Stub("docker", []string{"volume", "rm", "myvol"}, exec.Stub{})
	c := dockerx.New(r)
	if err := c.VolumeRemove(context.Background(), "myvol"); err != nil {
		t.Fatalf("VolumeRemove: %v", err)
	}
	if len(r.Calls) != 1 || r.Calls[0].Args[2] != "myvol" {
		t.Errorf("calls = %+v", r.Calls)
	}
}

func TestComposeVersion(t *testing.T) {
	t.Parallel()
	r := exec.NewRecordingRunner()
	r.Stub("docker", []string{"compose", "version"}, exec.Stub{})
	c := dockerx.New(r)
	if err := c.ComposeVersion(context.Background()); err != nil {
		t.Fatalf("ComposeVersion: %v", err)
	}
}

func TestComposeVersionShort(t *testing.T) {
	t.Parallel()
	r := exec.NewRecordingRunner()
	r.Stub("docker", []string{"compose", "version", "--short"},
		exec.Stub{Stdout: []byte("v2.20.0\n")})
	c := dockerx.New(r)
	got, err := c.ComposeVersionShort(context.Background())
	if err != nil {
		t.Fatalf("ComposeVersionShort: %v", err)
	}
	if got != "v2.20.0" {
		t.Errorf("got %q, want \"v2.20.0\"", got)
	}
}

func TestComposePS(t *testing.T) {
	t.Parallel()
	r := exec.NewRecordingRunner()
	r.Stub("docker", []string{"compose", "ps", "--format", "{{.Name}}"},
		exec.Stub{Stdout: []byte("svc1\nsvc2\n")})
	c := dockerx.New(r)
	lines, err := c.ComposePS(context.Background(), "{{.Name}}")
	if err != nil {
		t.Fatalf("ComposePS: %v", err)
	}
	if len(lines) != 2 || lines[0] != "svc1" {
		t.Errorf("lines = %v", lines)
	}
}
