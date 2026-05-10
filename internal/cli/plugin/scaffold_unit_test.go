//nolint:testpackage // white-box tests for unexported validators and prompter.
package plugincli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/huh"

	"github.com/sukekyo26/cocoon/internal/i18n"
)

func TestValidateNameInput(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		in      string
		wantErr error
	}{
		{"empty", "", errInputRequired},
		{"whitespace_only", "   \t  ", errInputRequired},
		{"normal", "GitHub CLI", nil},
		{"single_char", "x", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := validateNameInput(tc.in)
			if !errors.Is(got, tc.wantErr) {
				t.Errorf("validateNameInput(%q) = %v, want %v", tc.in, got, tc.wantErr)
			}
		})
	}
}

func TestValidateDescriptionInput(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		in      string
		wantErr error
	}{
		{"empty", "", errInputRequired},
		{"whitespace_only", "   ", errInputRequired},
		{"missing_url", "Just a description", errURLInDescription},
		{"missing_paren_only", "https://example.com", errURLInDescription},
		{"with_url", "GitHub CLI (https://cli.github.com)", nil},
		{"with_http_url", "Local server (http://localhost)", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := validateDescriptionInput(tc.in)
			if !errors.Is(got, tc.wantErr) {
				t.Errorf("validateDescriptionInput(%q) = %v, want %v", tc.in, got, tc.wantErr)
			}
		})
	}
}

func TestApplyPickedTemplate(t *testing.T) {
	t.Parallel()
	t.Run("non_empty_overwrites", func(t *testing.T) {
		t.Parallel()
		opts := &scaffoldOpts{template: tmplGeneric} //nolint:exhaustruct // only template matters
		applyPickedTemplate(opts, string(tmplTarball))
		if opts.template != tmplTarball {
			t.Errorf("template = %q, want %q", opts.template, tmplTarball)
		}
	})
	t.Run("empty_preserves_existing", func(t *testing.T) {
		t.Parallel()
		opts := &scaffoldOpts{template: tmplCurlPipe} //nolint:exhaustruct // only template matters
		applyPickedTemplate(opts, "")
		if opts.template != tmplCurlPipe {
			t.Errorf("template = %q, want %q (unchanged)", opts.template, tmplCurlPipe)
		}
	})
}

// fakePrompter is a deterministic prompter for promptMissing tests. It records
// the groups it was called with and returns onRunErr.
type fakePrompter struct {
	calls    int
	groupCnt int
	onRunErr error
}

func (f *fakePrompter) Run(groups []*huh.Group) error {
	f.calls++
	f.groupCnt = len(groups)
	return f.onRunErr
}

func TestPromptMissing_AllSetSkipsForm(t *testing.T) {
	t.Parallel()
	opts := &scaffoldOpts{
		setName:            true,
		setDescription:     true,
		setDefaultEnabled:  true,
		setRequiresRoot:    true,
		setVersionCapable:  true,
		setTemplate:        true,
		setWithInstallUser: true,
		template:           tmplGeneric,
	} //nolint:exhaustruct // remaining fields default; only setX flags matter
	p := &fakePrompter{}
	cat := i18n.New(i18n.Detect())
	if err := promptMissing(opts, cat, p); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if p.calls != 1 {
		t.Errorf("prompter.Run called %d times, want 1 (with empty groups)", p.calls)
	}
	if p.groupCnt != 0 {
		t.Errorf("groupCnt = %d, want 0 — every setX is true so no groups should be appended", p.groupCnt)
	}
}

func TestPromptMissing_NoneSetBuildsAllGroups(t *testing.T) {
	t.Parallel()
	//nolint:exhaustruct // setX defaults all-false; promptMissing builds all 7 groups.
	opts := &scaffoldOpts{template: tmplGeneric}
	p := &fakePrompter{}
	cat := i18n.New(i18n.Detect())
	if err := promptMissing(opts, cat, p); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	const wantGroups = 7
	if p.groupCnt != wantGroups {
		t.Errorf("groupCnt = %d, want %d (name, desc, default, requires-root, version-capable, template, with-install-user)",
			p.groupCnt, wantGroups)
	}
}

func TestPromptMissing_PrompterErrorPropagates(t *testing.T) {
	t.Parallel()
	//nolint:exhaustruct // setX defaults all-false
	opts := &scaffoldOpts{template: tmplGeneric}
	want := errors.New("prompter exploded")
	p := &fakePrompter{onRunErr: want}
	cat := i18n.New(i18n.Detect())
	err := promptMissing(opts, cat, p)
	if !errors.Is(err, want) {
		t.Errorf("err = %v, want errors.Is(err, %q)", err, want)
	}
}

func TestPromptMissing_CanceledPropagates(t *testing.T) {
	t.Parallel()
	//nolint:exhaustruct // setX defaults all-false
	opts := &scaffoldOpts{template: tmplGeneric}
	p := &fakePrompter{onRunErr: ErrCanceled}
	cat := i18n.New(i18n.Detect())
	err := promptMissing(opts, cat, p)
	if !errors.Is(err, ErrCanceled) {
		t.Errorf("err = %v, want errors.Is(err, ErrCanceled)", err)
	}
}

// TestInstallUserPromptDescription_ContainsUsageGuidance pins down that
// the scaffold's `Also generate install_user.sh?` prompt carries a
// description explaining when the file is needed. Authors who hit the
// prompt without prior context need enough cues (root + user phrasing,
// rc-file editing, a concrete example) to make a sensible choice
// without leaving the wizard. Both EN and JA catalogs must carry the
// same guidance.
func TestInstallUserPromptDescription_ContainsUsageGuidance(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		lang i18n.Lang
		want []string
	}{
		{
			name: "en",
			lang: i18n.LangEN,
			want: []string{"requires_root", "USERNAME", "starship", "~/.bashrc"},
		},
		{
			name: "ja",
			lang: i18n.LangJA,
			want: []string{"requires_root", "USERNAME", "starship", "~/.bashrc"},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cat := i18n.New(tc.lang)
			got := cat.Msg("plugin_scaffold_prompt_user_hook_desc")
			for _, want := range tc.want {
				if !strings.Contains(got, want) {
					t.Errorf("description missing %q\n--- got ---\n%s", want, got)
				}
			}
		})
	}
}

func TestDirExists(t *testing.T) {
	t.Parallel()
	t.Run("existing_dir", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		got, err := dirExists(dir)
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if !got {
			t.Errorf("dirExists(%q) = false, want true", dir)
		}
	})
	t.Run("nonexistent", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "does-not-exist")
		got, err := dirExists(path)
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if got {
			t.Errorf("dirExists(%q) = true, want false", path)
		}
	})
	t.Run("path_is_file_not_dir", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		filePath := filepath.Join(dir, "regular-file")
		if err := os.WriteFile(filePath, []byte("hi"), 0o600); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
		got, err := dirExists(filePath)
		if err == nil {
			t.Errorf("expected error when path is a file; got (exists=%v, err=nil)", got)
		}
		if !errors.Is(err, errPathNotADir) {
			t.Errorf("err = %v, want errors.Is(err, errPathNotADir)", err)
		}
	})
}
