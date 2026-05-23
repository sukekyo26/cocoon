//nolint:testpackage // exercises unexported mapWorkspaceErr classification.
package gencli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
	"github.com/sukekyo26/cocoon/internal/generate/codeworkspace"
	"github.com/sukekyo26/cocoon/internal/i18n"
)

// TestMapWorkspaceErr_ClassifiesSentinels covers every arm of
// mapWorkspaceErr's switch, including the default arm that wraps an
// unknown error as ErrFailure (the path that runGenWorkspace falls back
// to when codeworkspace.Generate returns something we did not anticipate).
func TestMapWorkspaceErr_ClassifiesSentinels(t *testing.T) {
	t.Parallel()

	cat := i18n.New(i18n.LangEN)

	// The no_folders arm rewrites the message via cat.Msg, dropping the
	// underlying sentinel from the chain; the other three arms use %w
	// and must preserve it for debugging.
	cases := []struct {
		name        string
		in          error
		want        error
		preservesIn bool
	}{
		{name: "no_folders", in: codeworkspace.ErrNoFolders, want: clihelpers.ErrUsage, preservesIn: false},
		{name: "invalid_folder_path", in: fmt.Errorf("%w: empty path", codeworkspace.ErrInvalidFolderPath), want: clihelpers.ErrUsage, preservesIn: true},
		{name: "missing_home_dir", in: fmt.Errorf("%w: ~/foo", codeworkspace.ErrMissingHomeDir), want: clihelpers.ErrFailure, preservesIn: true},
		{name: "unknown_default_arm", in: errors.New("system EIO"), want: clihelpers.ErrFailure, preservesIn: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := mapWorkspaceErr(tc.in, cat)
			if !errors.Is(got, tc.want) {
				t.Fatalf("err = %v, want errors.Is %v", got, tc.want)
			}
			if tc.preservesIn && !errors.Is(got, tc.in) {
				t.Errorf("err = %v lost original cause %v", got, tc.in)
			}
		})
	}
}
