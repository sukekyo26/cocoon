package gitx_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/sukekyo26/cocoon/internal/exec"
	"github.com/sukekyo26/cocoon/internal/exec/gitx"
)

func TestCloneMinimalArgs(t *testing.T) {
	t.Parallel()
	r := exec.NewRecordingRunner()
	c := gitx.New(r)
	_, err := c.Clone(context.Background(), gitx.CloneOptions{
		URL:               "https://github.com/example/repo.git",
		Target:            "/tmp/repo",
		Branch:            "",
		Depth:             0,
		RecurseSubmodules: false,
	})
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}
	want := []string{"clone", "https://github.com/example/repo.git", "/tmp/repo"}
	if !reflect.DeepEqual(r.Calls[0].Args, want) {
		t.Errorf("args = %v, want %v", r.Calls[0].Args, want)
	}
}

func TestCloneAllOptions(t *testing.T) {
	t.Parallel()
	r := exec.NewRecordingRunner()
	c := gitx.New(r)
	_, err := c.Clone(context.Background(), gitx.CloneOptions{
		URL:               "git@example.com:org/repo.git",
		Target:            "/tmp/repo",
		Branch:            "main",
		Depth:             1,
		RecurseSubmodules: true,
	})
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}
	want := []string{
		"clone",
		"--branch", "main",
		"--depth", "1",
		"--recurse-submodules",
		"git@example.com:org/repo.git",
		"/tmp/repo",
	}
	if !reflect.DeepEqual(r.Calls[0].Args, want) {
		t.Errorf("args = %v, want %v", r.Calls[0].Args, want)
	}
}
