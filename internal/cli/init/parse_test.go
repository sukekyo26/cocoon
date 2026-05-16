//nolint:testpackage // exercises unexported parsers and validators.
package initcli

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/i18n"
)

func TestMakeStrictValidator_Empty(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		lang    i18n.Lang
		wantSub string
	}{
		{"en", i18n.LangEN, "please enter"},
		{"ja", i18n.LangJA, "必須"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cat := i18n.New(tc.lang)
			err := makeStrictValidator(rxServiceName, "init_err_service_name_fmt", cat)("")
			if err == nil {
				t.Fatal("expected error for empty input")
			}
			msg := err.Error()
			if strings.Contains(msg, "^[") || strings.Contains(msg, "*$") {
				t.Errorf("must not leak regex: %q", msg)
			}
			if !strings.Contains(strings.ToLower(msg), strings.ToLower(tc.wantSub)) {
				t.Errorf("msg %q missing %q", msg, tc.wantSub)
			}
		})
	}
}

func TestMakeStrictValidator_BadCharsServiceName(t *testing.T) {
	t.Parallel()
	cat := i18n.New(i18n.LangEN)
	for _, in := range []string{"Bad-Name", "_underscore", "1starts-with-digit", "has space", "has.dot", "has/slash"} {
		err := makeStrictValidator(rxServiceName, "init_err_service_name_fmt", cat)(in)
		if err == nil {
			t.Errorf("expected error for service-name %q, got nil", in)
			continue
		}
		if strings.Contains(err.Error(), "^[") {
			t.Errorf("service-name %q error leaks regex: %q", in, err.Error())
		}
	}
}

func TestMakeStrictValidator_BadCharsUsername(t *testing.T) {
	t.Parallel()
	cat := i18n.New(i18n.LangEN)
	for _, in := range []string{"BadUser", "1user", "user space", "user.dot"} {
		err := makeStrictValidator(rxUsername, "init_err_username_fmt", cat)(in)
		if err == nil {
			t.Errorf("expected error for username %q, got nil", in)
		}
	}
}

func TestMakeStrictValidator_AcceptsValid(t *testing.T) {
	t.Parallel()
	cat := i18n.New(i18n.LangEN)
	for _, in := range []string{"a", "myapp", "my-api", "my_api", "api123", "x-1_2-3"} {
		if err := makeStrictValidator(rxServiceName, "init_err_service_name_fmt", cat)(in); err != nil {
			t.Errorf("strict svc validator rejected %q: %v", in, err)
		}
	}
	for _, in := range []string{"dev", "_dev", "user-1", "u_2", "_"} {
		if err := makeStrictValidator(rxUsername, "init_err_username_fmt", cat)(in); err != nil {
			t.Errorf("strict username validator rejected %q: %v", in, err)
		}
	}
}

// Pin: username allows leading underscore, service-name does not.
func TestRegex_LeadingUnderscoreAsymmetry(t *testing.T) {
	t.Parallel()
	if rxServiceName.MatchString("_underscore") {
		t.Error("service-name must reject leading _")
	}
	if !rxUsername.MatchString("_underscore") {
		t.Error("username must accept leading _")
	}
}

func TestParseAptCategories(t *testing.T) {
	t.Parallel()
	out, err := parseAptCategories("text-editors,build")
	if err != nil || len(out) != 2 {
		t.Errorf("got %v %v", out, err)
	}
	out, err = parseAptCategories("")
	if err != nil || len(out) != 0 {
		t.Errorf("empty string → empty list, got %v %v", out, err)
	}
	out, err = parseAptCategories(",,,")
	if err != nil || len(out) != 0 {
		t.Errorf("only commas → empty list, got %v %v", out, err)
	}
	if _, err := parseAptCategories("not-a-real-cat"); !errors.Is(err, clihelpers.ErrUsage) {
		t.Errorf("unknown category should be ErrUsage, got %v", err)
	}
}

func TestParsePlugins(t *testing.T) {
	t.Parallel()
	plugins := loadPluginsForTest(t)

	out, err := parsePlugins("go,uv,github-cli", plugins)
	if err != nil || len(out) != 3 {
		t.Errorf("got %v %v", out, err)
	}
	out, err = parsePlugins("", plugins)
	if err != nil || len(out) != 0 {
		t.Errorf("empty string → empty list, got %v %v", out, err)
	}
	out, err = parsePlugins(" go , uv , ", plugins)
	if err != nil || len(out) != 2 || out[0] != "go" || out[1] != "uv" {
		t.Errorf("whitespace handling: got %v %v", out, err)
	}
	if _, err := parsePlugins("does-not-exist", plugins); !errors.Is(err, clihelpers.ErrUsage) {
		t.Errorf("unknown plugin should be ErrUsage, got %v", err)
	}
}

func TestParsePluginVersions(t *testing.T) {
	t.Parallel()
	plugins := loadPluginsForTest(t)
	enabled := []string{"go", "uv", "starship"}

	out, err := parsePluginVersions("go=1.23.4,uv=0.5.7", plugins, enabled)
	if err != nil {
		t.Fatalf("happy path: %v", err)
	}
	if out["go"] != "1.23.4" || out["uv"] != "0.5.7" || len(out) != 2 {
		t.Errorf("happy path map: %v", out)
	}

	// parsePluginVersions returns a non-nil empty map for whitespace-only
	// or empty input so it doesn't trip golangci-lint's `nilnil` rule. The
	// writer keys behavior on len(...) == 0, so the empty-vs-nil distinction
	// is invisible to callers in practice.
	if out, err := parsePluginVersions("", plugins, enabled); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v", out, err)
	}

	if out, err := parsePluginVersions(" go = 1.23.4 ,  uv=0.5.7 ", plugins, enabled); err != nil ||
		out["go"] != "1.23.4" || out["uv"] != "0.5.7" {
		t.Errorf("whitespace handling: out=%v err=%v", out, err)
	}

	for _, bad := range []string{"go", "go=", "=1.23", "go==1.23"} {
		if _, err := parsePluginVersions(bad, plugins, enabled); !errors.Is(err, clihelpers.ErrUsage) {
			t.Errorf("malformed token %q: expected ErrUsage, got %v", bad, err)
		}
	}

	if _, err := parsePluginVersions("does-not-exist=1.0", plugins, enabled); !errors.Is(err, clihelpers.ErrUsage) {
		t.Errorf("unknown plugin: expected ErrUsage, got %v", err)
	}

	// docker-cli ships in the embedded catalog but is `version_capable=false`.
	if _, err := parsePluginVersions("docker-cli=1.0", plugins, []string{"docker-cli"}); !errors.Is(err, clihelpers.ErrUsage) {
		t.Errorf("non-version-capable: expected ErrUsage, got %v", err)
	}

	// `go` is version_capable but the caller did not list it in --plugins.
	if _, err := parsePluginVersions("go=1.23.4", plugins, []string{"uv"}); !errors.Is(err, clihelpers.ErrUsage) {
		t.Errorf("missing-from-enable: expected ErrUsage, got %v", err)
	}

	if _, err := parsePluginVersions("go=1.23.4,go=1.24.0", plugins, enabled); !errors.Is(err, clihelpers.ErrUsage) {
		t.Errorf("duplicate id: expected ErrUsage, got %v", err)
	}
}

func TestParseAliasBundles(t *testing.T) {
	t.Parallel()
	out, err := parseAliasBundles("git,ls,docker")
	if err != nil || len(out) != 3 {
		t.Errorf("got %v %v", out, err)
	}
	out, err = parseAliasBundles("")
	if err != nil || len(out) != 0 {
		t.Errorf("empty string -> empty list, got %v %v", out, err)
	}
	out, err = parseAliasBundles(" git , ls , ")
	if err != nil || len(out) != 2 || out[0] != "git" || out[1] != "ls" {
		t.Errorf("whitespace handling: got %v %v", out, err)
	}
	if _, err := parseAliasBundles("k8s"); !errors.Is(err, clihelpers.ErrUsage) {
		t.Errorf("unknown bundle should be ErrUsage, got %v", err)
	}
}

func TestParsePorts(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		raw     string
		want    []string
		wantErr bool
	}{
		{"empty_returns_nil", "", nil, false},
		{"whitespace_only_returns_nil", " , ,\t,", nil, false},
		{"single_short", "3000:3000", []string{"3000:3000"}, false},
		{"multiple_short", "3000:3000,5432:5432", []string{"3000:3000", "5432:5432"}, false},
		{"trims_whitespace", " 3000:3000 , 5432:5432 ", []string{"3000:3000", "5432:5432"}, false},
		{
			"accepts_all_documented_forms",
			"3000,3000-3005,8000:8000,9090-9091:8080-8081,49100:22," +
				"127.0.0.1:8001:8001,127.0.0.1:5000-5010:5000-5010,6060:6060/udp",
			[]string{
				"3000", "3000-3005", "8000:8000", "9090-9091:8080-8081", "49100:22",
				"127.0.0.1:8001:8001", "127.0.0.1:5000-5010:5000-5010", "6060:6060/udp",
			},
			false,
		},
		{"rejects_garbage", "abc", nil, true},
		{"rejects_out_of_range", "99999:80", nil, true},
		{"rejects_bad_ip", "999.999.999.999:80:80", nil, true},
		{"rejects_unknown_proto", "3000:3000/sctp", nil, true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parsePorts(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parsePorts(%q) err = nil, want error", tc.raw)
				}
				if !errors.Is(err, clihelpers.ErrUsage) {
					t.Errorf("parsePorts(%q) err = %v, want errors.Is ErrUsage", tc.raw, err)
				}
				if !errors.Is(err, config.ErrPortShortForm) {
					t.Errorf("parsePorts(%q) err = %v, want errors.Is config.ErrPortShortForm", tc.raw, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("parsePorts(%q) err = %v, want nil", tc.raw, err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("parsePorts(%q) mismatch (-want +got):\n%s", tc.raw, diff)
			}
		})
	}
}

// TestPortsInputValidator pins the i18n behavior of the interactive prompt
// validator: rejection messages come from the catalog (EN / JA), not from
// config.ValidateShortForm's English text. Accept paths return nil so huh
// advances to the next group.
func TestPortsInputValidator(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		lang i18n.Lang
		in   string
		// substring asserted in err.Error(); empty = nil expected.
		wantSubstr string
	}{
		{"en_accept_blank", i18n.LangEN, "", ""},
		{"en_accept_single", i18n.LangEN, "3000:3000", ""},
		{
			"en_accept_all_forms", i18n.LangEN,
			"3000,3000-3005,8000:8000,127.0.0.1:8001:8001,6060:6060/udp", "",
		},
		{
			"en_reject_uses_catalog_phrase", i18n.LangEN, "abc",
			"is not a valid port short form",
		},
		{
			"ja_reject_uses_catalog_phrase", i18n.LangJA, "abc",
			"はポート指定として無効です",
		},
		{
			"ja_reject_out_of_range", i18n.LangJA, "99999:80",
			"はポート指定として無効です",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cat := i18n.New(tc.lang)
			err := portsInputValidator(cat)(tc.in)
			if tc.wantSubstr == "" {
				if err != nil {
					t.Fatalf("validator(%q) = %v, want nil", tc.in, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("validator(%q) = nil, want error containing %q", tc.in, tc.wantSubstr)
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Errorf("validator(%q) err = %q, want substring %q",
					tc.in, err.Error(), tc.wantSubstr)
			}
		})
	}
}
