package compose_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/generate"
	"github.com/sukekyo26/cocoon/internal/generate/compose"
)

func TestGenerate_RejectsReservedCustomVolume(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"local", "cocoon"} {
		ws := &config.Workspace{
			Container: config.ContainerSpec{
				ServiceName: "dev",
				Username:    "u",
				Os:          "ubuntu",
				OsVersion:   "26.04",
			},
			Volumes: map[string]string{
				name: "/home/u/some-path",
			},
		}
		ctx := &generate.WorkspaceContext{WS: ws}
		var warns bytes.Buffer
		_, err := compose.Generate(ctx, compose.Options{Warnings: &warns})
		if !errors.Is(err, compose.ErrVolumeNameConflict) {
			t.Errorf("custom volume %q: expected ErrVolumeNameConflict, got %v", name, err)
		}
	}
}
