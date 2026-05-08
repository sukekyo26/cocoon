package i18n_test

import (
	"testing"

	"github.com/sukekyo26/cocoon/internal/i18n"
)

func TestDetect(t *testing.T) {
	cases := []struct {
		name string
		envs map[string]string
		want i18n.Lang
	}{
		{name: "default_en", envs: map[string]string{}, want: i18n.LangEN},
		{name: "workspace_lang_ja", envs: map[string]string{"WORKSPACE_LANG": "ja"}, want: i18n.LangJA},
		{name: "lc_all_ja", envs: map[string]string{"LC_ALL": "ja_JP.UTF-8"}, want: i18n.LangJA},
		{name: "lang_ja", envs: map[string]string{"LANG": "ja_JP.UTF-8"}, want: i18n.LangJA},
		{name: "lang_en", envs: map[string]string{"LANG": "en_US.UTF-8"}, want: i18n.LangEN},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Cannot t.Parallel() here because t.Setenv requires non-parallel.
			for _, k := range []string{"WORKSPACE_LANG", "LC_ALL", "LC_MESSAGES", "LANG"} {
				t.Setenv(k, "")
			}
			for k, v := range tc.envs {
				t.Setenv(k, v)
			}
			if got := i18n.Detect(); got != tc.want {
				t.Fatalf("Detect() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCatalogLang(t *testing.T) {
	t.Parallel()
	if got := i18n.New(i18n.LangEN).Lang(); got != i18n.LangEN {
		t.Errorf("Lang() = %v, want LangEN", got)
	}
	if got := i18n.New(i18n.LangJA).Lang(); got != i18n.LangJA {
		t.Errorf("Lang() = %v, want LangJA", got)
	}
	// Unknown language falls back to English.
	if got := i18n.New(i18n.Lang("xx")).Lang(); got != i18n.LangEN {
		t.Errorf("Lang() with unknown = %v, want LangEN fallback", got)
	}
}

func TestCatalogMsg(t *testing.T) {
	t.Parallel()
	en := i18n.New(i18n.LangEN)
	ja := i18n.New(i18n.LangJA)

	if got := en.Msg("clean_header"); got != "Docker Volume Cleanup Script" {
		t.Fatalf("EN clean_header: %q", got)
	}
	if got := ja.Msg("clean_header"); got != "Docker ボリュームクリーンアップスクリプト" {
		t.Fatalf("JA clean_header: %q", got)
	}
	// Format args.
	if got := en.Msg("clean_prefix_info", "myproj_dev_"); got != "  Prefix: myproj_dev_" {
		t.Fatalf("EN clean_prefix_info: %q", got)
	}
	// Missing key falls back to the key string itself.
	if got := en.Msg("__missing__"); got != "__missing__" {
		t.Fatalf("missing key fallback: %q", got)
	}
}
