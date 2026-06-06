package config_test

import (
	"errors"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/i18n"
	"github.com/sukekyo26/cocoon/internal/warn"
)

// portSkipReasons returns the reason code of each PortSkip diagnostic in sink,
// so devcontainer port-skip tests assert on stable codes rather than English
// warning text.
func portSkipReasons(t *testing.T, sink *warn.Sink) []string {
	t.Helper()
	var out []string
	for _, w := range sink.All() {
		if w.Code != warn.PortSkip {
			continue
		}
		if len(w.Args) < 2 {
			t.Fatalf("PortSkip warning has %d args, want >=2", len(w.Args))
		}
		ref, ok := w.Args[1].(warn.Ref)
		if !ok {
			t.Fatalf("PortSkip reason arg is %T, want warn.Ref", w.Args[1])
		}
		out = append(out, ref.Code)
	}
	return out
}

// TestPortShortFormPattern guards the JSON Schema pattern against drift from
// the Go validator. Both must accept and reject the same set of strings so
// editor-side validation matches what cocoon enforces at load time.
func TestPortShortFormPattern(t *testing.T) {
	t.Parallel()
	rx := regexp.MustCompile(config.PortShortFormPattern)

	accept := []string{
		"3000",
		"3000:3000",
		"127.0.0.1:5432:5432",
		"127.0.0.1:5432:5432/tcp",
		"3000-3005:3000-3005",
		"[::1]:80:80",
		"8080:8080/udp",
	}
	reject := []string{
		"",
		"abc",
		"3000:abc",
		"3000:3000/sctp",
		"3000:",
	}
	for _, s := range accept {
		if !rx.MatchString(s) {
			t.Errorf("PortShortFormPattern should accept %q", s)
		}
	}
	for _, s := range reject {
		if rx.MatchString(s) {
			t.Errorf("PortShortFormPattern should reject %q", s)
		}
	}
}

// TestValidateShortForm covers the public validator that both
// `[ports].forward` schema validation and the `cocoon init` prompt depend on.
// The accept set mirrors the docker-compose short-form patterns documented
// for `[ports]`; the reject set guards every rule the validator enforces
// (regex shape, [portMin, portMax] bounds, IP literal syntax).
func TestValidateShortForm(t *testing.T) {
	t.Parallel()
	accept := []string{
		"3000",
		"3000-3005",
		"8000:8000",
		"9090-9091:8080-8081",
		"49100:22",
		"127.0.0.1:8001:8001",
		"127.0.0.1:5000-5010:5000-5010",
		"6060:6060/udp",
		"[::1]:80:80",
		"3000:3000/tcp",
	}
	for _, s := range accept {
		s := s
		t.Run("accept/"+s, func(t *testing.T) {
			t.Parallel()
			if err := config.ValidateShortForm(s); err != nil {
				t.Errorf("ValidateShortForm(%q) = %v, want nil", s, err)
			}
		})
	}

	reject := []struct {
		in     string
		reason string // substring asserted in err.Error()
	}{
		{"", "does not match docker-compose short form"},
		{"abc", "does not match docker-compose short form"},
		{"3000:", "does not match docker-compose short form"},
		{"3000:3000/sctp", "does not match docker-compose short form"},
		{"3000:abc", "does not match docker-compose short form"},
		{"99999", "port must be in [1,65535]"},
		{"99999:80", "port must be in [1,65535]"},
		{"0:80", "port must be in [1,65535]"},
		{"3000-99999:3000", "port must be in [1,65535]"},
		{"999.999.999.999:80:80", "is not a valid IPv4/IPv6 address"},
	}
	for _, tc := range reject {
		tc := tc
		t.Run("reject/"+tc.in, func(t *testing.T) {
			t.Parallel()
			err := config.ValidateShortForm(tc.in)
			if err == nil {
				t.Fatalf("ValidateShortForm(%q) = nil, want error", tc.in)
			}
			if !errors.Is(err, config.ErrPortShortForm) {
				t.Errorf("ValidateShortForm(%q) err = %v, want errors.Is ErrPortShortForm", tc.in, err)
			}
			if !strings.Contains(err.Error(), tc.reason) {
				t.Errorf("ValidateShortForm(%q) err = %q, want substring %q", tc.in, err.Error(), tc.reason)
			}
		})
	}
}

// TestValidateShortForm_Localizable pins that a rejection carries the reason as
// a localizable error (i18n.Localizer): the `--ports` flag path renders it in
// the active language at the CLI boundary rather than freezing English. The
// classification sentinel must still be reachable via errors.Is.
func TestValidateShortForm_Localizable(t *testing.T) {
	t.Parallel()
	err := config.ValidateShortForm("abc")
	if err == nil {
		t.Fatal("ValidateShortForm(\"abc\") = nil, want error")
	}
	if !errors.Is(err, config.ErrPortShortForm) {
		t.Fatalf("err = %v, want errors.Is ErrPortShortForm", err)
	}
	var loc i18n.Localizer
	if !errors.As(err, &loc) {
		t.Fatalf("err = %v, want it to satisfy i18n.Localizer", err)
	}
	en := loc.Localize(i18n.New(i18n.LangEN))
	ja := loc.Localize(i18n.New(i18n.LangJA))
	if en == ja {
		t.Fatalf("Localize(en) == Localize(ja) = %q, want language-specific reasons", en)
	}
	if !strings.Contains(en, "does not match docker-compose short form") {
		t.Errorf("Localize(en) = %q, want English short-form reason", en)
	}
	if !strings.Contains(ja, "短縮形式") {
		t.Errorf("Localize(ja) = %q, want Japanese short-form reason", ja)
	}
}

func TestComposePortEntries_Strings(t *testing.T) {
	t.Parallel()
	in := []any{
		"3000",
		"8080:80",
		"127.0.0.1:5432:5432/tcp",
		"3000-3005:3000-3005",
		"[::1]:8080:8080",
	}
	got := config.ComposePortEntries(in)
	if len(got) != len(in) {
		t.Fatalf("len = %d, want %d", len(got), len(in))
	}
	for i, p := range got {
		if p.IsLong() {
			t.Errorf("entry[%d] should be short, got long", i)
		}
		if p.Short != in[i] {
			t.Errorf("entry[%d].Short = %q, want %q", i, p.Short, in[i])
		}
	}
}

func TestComposePortEntries_LongForm(t *testing.T) {
	t.Parallel()
	in := []any{
		map[string]any{
			"target":    int64(5432),
			"published": int64(5432),
			"host_ip":   "127.0.0.1",
			"protocol":  "tcp",
			"mode":      "ingress",
			"unknown":   "dropped", // normalizer drops; validator would have caught
		},
	}
	got := config.ComposePortEntries(in)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	want := map[string]any{
		"target":    5432,
		"published": 5432,
		"host_ip":   "127.0.0.1",
		"protocol":  "tcp",
		"mode":      "ingress",
	}
	if diff := cmp.Diff(want, got[0].Long); diff != "" {
		t.Errorf("Long mismatch (-want +got):\n%s", diff)
	}
}

func TestDevcontainerPortEntries(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		in         []any
		want       []int
		wantReason string // warn.Ref code expected in a PortSkip diagnostic ("" = none)
	}{
		{
			name: "container_only",
			in:   []any{"3000"},
			want: []int{3000},
		},
		{
			name: "host_container",
			in:   []any{"8080:80"},
			want: []int{80},
		},
		{
			// forwardPorts takes the container-side port (3000), not the
			// published host port (30002).
			name: "host_neq_container_uses_container",
			in:   []any{"30002:3000"},
			want: []int{3000},
		},
		{
			name: "ip_bind_proto",
			in:   []any{"127.0.0.1:5432:5432/tcp"},
			want: []int{5432},
		},
		{
			name: "ipv6",
			in:   []any{"[::1]:8080:8080"},
			want: []int{8080},
		},
		{
			name:       "range_skipped",
			in:         []any{"3000-3005:3000-3005"},
			want:       []int{},
			wantReason: warn.PortReasonRange,
		},
		{
			// Long form: forwardPorts takes `target` (container 5432), not
			// the published host port (15432).
			name: "long_form_published",
			in: []any{
				map[string]any{"target": int64(5432), "published": int64(15432)},
			},
			want: []int{5432},
		},
		{
			name: "long_form_target_only",
			in: []any{
				map[string]any{"target": int64(8080)},
			},
			want: []int{8080},
		},
		{
			name: "long_form_host_mode",
			in: []any{
				map[string]any{"target": int64(9090), "mode": "host"},
			},
			want:       []int{},
			wantReason: warn.PortReasonHostMode,
		},
		{
			name: "long_form_published_string_single",
			in: []any{
				map[string]any{"target": int64(8080), "published": "8080"},
			},
			want: []int{8080},
		},
		{
			// `published` may be a range while `target` is a single
			// container port; forwardPorts takes target, so this is kept,
			// not skipped.
			name: "long_form_published_string_range",
			in: []any{
				map[string]any{"target": int64(8000), "published": "8000-8010"},
			},
			want: []int{8000},
		},
		{
			// A long-form entry that omits `target` cannot pass
			// validateLongForm ("target is required"), but
			// DevcontainerPortEntries is exported and exercised directly
			// (here, and by any caller that hands it raw data), so it must
			// still report a clear "missing target" reason rather than the
			// confusing "has a non-integer target <nil>" the absent value
			// would otherwise produce.
			name: "long_form_missing_target",
			in: []any{
				map[string]any{"published": int64(8080)},
			},
			want:       []int{},
			wantReason: warn.PortReasonMissingTarget,
		},
		{
			// `target` present but not a plain integer is reported as a
			// non-integer target, not as "missing target".
			name: "long_form_non_integer_target",
			in: []any{
				map[string]any{"target": "5432-5433"},
			},
			want:       []int{},
			wantReason: warn.PortReasonNonIntegerTarget,
		},
		{
			// UDP entries are TCP-incompatible for the devcontainer
			// port tunnel; they're skipped with a "protocol = \"udp\""
			// warning. Sibling TCP / unspecified-proto entries flow
			// through. The range entry triggers its own skip warning,
			// covering both reasons in one fixture.
			name: "mixed",
			in: []any{
				"3000",
				map[string]any{"target": int64(5432)},
				"3000-3005:3000-3005",
				"127.0.0.1:8080:8080/udp",
			},
			want:       []int{3000, 5432},
			wantReason: warn.PortReasonRange,
		},
		{
			name:       "short_form_udp_skipped",
			in:         []any{"6060:6060/udp"},
			want:       []int{},
			wantReason: warn.PortReasonShortUDP,
		},
		{
			name: "long_form_udp_skipped",
			in: []any{
				map[string]any{
					"target":    int64(53),
					"published": int64(53),
					"protocol":  "udp",
				},
			},
			want:       []int{},
			wantReason: warn.PortReasonLongUDP,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sink := warn.New()
			got := config.DevcontainerPortEntries(tc.in, sink)
			if got == nil {
				got = []int{}
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("ports mismatch (-want +got):\n%s", diff)
			}
			reasons := portSkipReasons(t, sink)
			if tc.wantReason != "" && !slices.Contains(reasons, tc.wantReason) {
				t.Errorf("skip reasons = %v, want to contain %q", reasons, tc.wantReason)
			}
			if tc.wantReason == "" && len(reasons) != 0 {
				t.Errorf("unexpected skip reasons = %v", reasons)
			}
		})
	}
}

func TestLongFormKeyOrder(t *testing.T) {
	t.Parallel()
	got := config.LongFormKeyOrder()
	want := []string{"target", "published", "host_ip", "protocol", "mode"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("LongFormKeyOrder mismatch (-want +got):\n%s", diff)
	}
	got[0] = "tampered" // ensure caller mutation cannot poison the package state
	got2 := config.LongFormKeyOrder()
	if got2[0] != "target" {
		t.Errorf("LongFormKeyOrder must return a copy; got2[0] = %q", got2[0])
	}
}

func TestComposePortEntries_NormalizesIntKinds(t *testing.T) {
	t.Parallel()
	in := []any{
		map[string]any{"target": 3000, "published": float64(8080)},
	}
	got := config.ComposePortEntries(in)
	if len(got) != 1 || got[0].Long["target"] != 3000 || got[0].Long["published"] != 8080 {
		t.Errorf("Long mismatch: %+v", got[0].Long)
	}
}

func TestPortsSpec_Validate_TypeMismatches(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		forward []any
		wantMsg string
	}{
		{
			"target_string_rejected",
			[]any{map[string]any{"target": "3000"}},
			"target must be an integer",
		},
		{
			"host_ip_int_rejected",
			[]any{map[string]any{"target": int64(3000), "host_ip": int64(1)}},
			"host_ip must be a string",
		},
		{
			"protocol_int_rejected",
			[]any{map[string]any{"target": int64(3000), "protocol": int64(1)}},
			"protocol must be a string",
		},
		{
			"mode_int_rejected",
			[]any{map[string]any{"target": int64(3000), "mode": int64(1)}},
			"mode must be a string",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ws := &config.Workspace{
				Container: config.ContainerSpec{
					ServiceName:  "dev",
					Username:     "developer",
					Image:        "ubuntu",
					ImageVersion: "24.04",
				},
				Ports: &config.PortsSpec{Forward: tc.forward},
			}
			err := ws.Validate("test.toml")
			if err == nil {
				t.Fatalf("expected error")
			}
			var ve *config.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("expected ValidationError")
			}
			joined := ""
			for _, fe := range ve.Errors {
				joined += fe.Localize(i18n.English()) + "\n"
			}
			if !strings.Contains(joined, tc.wantMsg) {
				t.Errorf("error messages do not contain %q:\n%s", tc.wantMsg, joined)
			}
		})
	}
}

func TestPortsSpec_Validate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		forward []any
		wantOK  bool
		// wantMsg is a substring of any FieldError.Message when wantOK is false.
		wantMsg string
	}{
		{
			name:    "empty",
			forward: nil,
			wantOK:  true,
		},
		{
			name: "string_short_forms",
			forward: []any{
				"3000",
				"8080:80",
				"127.0.0.1:5432:5432/tcp",
				"3000-3005:3000-3005",
				"[::1]:8080:8080",
				"3000:3000/udp",
			},
			wantOK: true,
		},
		{
			name: "long_form_minimal",
			forward: []any{
				map[string]any{"target": int64(3000)},
			},
			wantOK: true,
		},
		{
			name: "long_form_full",
			forward: []any{
				map[string]any{
					"target":    int64(5432),
					"published": int64(5432),
					"host_ip":   "127.0.0.1",
					"protocol":  "tcp",
					"mode":      "host",
				},
			},
			wantOK: true,
		},
		{
			name: "long_form_published_string_single",
			forward: []any{
				map[string]any{"target": int64(8080), "published": "8080"},
			},
			wantOK: true,
		},
		{
			name: "long_form_published_string_range",
			forward: []any{
				map[string]any{"target": int64(8000), "published": "8000-8010"},
			},
			wantOK: true,
		},
		{
			name: "long_form_published_string_garbage",
			forward: []any{
				map[string]any{"target": int64(3000), "published": "abc"},
			},
			wantOK:  false,
			wantMsg: "must be a port or numeric range",
		},
		{
			name: "long_form_published_string_out_of_range",
			forward: []any{
				map[string]any{"target": int64(3000), "published": "8000-99999"},
			},
			wantOK:  false,
			wantMsg: "published port must be in",
		},
		{
			name: "long_form_published_bool_rejected",
			forward: []any{
				map[string]any{"target": int64(3000), "published": true},
			},
			wantOK:  false,
			wantMsg: "published must be an integer or a string",
		},
		{
			name:    "int_form_rejected",
			forward: []any{int64(3000)},
			wantOK:  false,
			wantMsg: "int form was removed",
		},
		{
			name:    "int_native_rejected",
			forward: []any{3000},
			wantOK:  false,
			wantMsg: "int form was removed",
		},
		{
			name:    "unknown_key",
			forward: []any{map[string]any{"target": int64(3000), "foo": "x"}},
			wantOK:  false,
			wantMsg: "unknown key",
		},
		{
			name:    "missing_target",
			forward: []any{map[string]any{"published": int64(3000)}},
			wantOK:  false,
			wantMsg: "target is required",
		},
		{
			name:    "invalid_protocol",
			forward: []any{map[string]any{"target": int64(3000), "protocol": "sctp"}},
			wantOK:  false,
			wantMsg: "protocol must be one of",
		},
		{
			name:    "invalid_mode",
			forward: []any{map[string]any{"target": int64(3000), "mode": "bogus"}},
			wantOK:  false,
			wantMsg: "mode must be one of",
		},
		{
			name:    "invalid_host_ip",
			forward: []any{map[string]any{"target": int64(3000), "host_ip": "not-an-ip"}},
			wantOK:  false,
			wantMsg: "is not a valid IP",
		},
		{
			name:    "target_out_of_range",
			forward: []any{map[string]any{"target": int64(70000)}},
			wantOK:  false,
			wantMsg: "target must be in",
		},
		{
			name:    "short_form_garbage",
			forward: []any{"abc:def"},
			wantOK:  false,
			wantMsg: "does not match docker-compose short form",
		},
		{
			name:    "short_form_port_too_high",
			forward: []any{"99999:99999"},
			wantOK:  false,
			wantMsg: "port must be in",
		},
		{
			name:    "wrong_type",
			forward: []any{true},
			wantOK:  false,
			wantMsg: "must be string or table",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ws := &config.Workspace{
				Container: config.ContainerSpec{
					ServiceName:  "dev",
					Username:     "developer",
					Image:        "ubuntu",
					ImageVersion: "24.04",
				},
				Ports: &config.PortsSpec{Forward: tc.forward},
			}
			err := ws.Validate("test.toml")
			if tc.wantOK {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error, got none")
			}
			var ve *config.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("expected ValidationError, got %T", err)
			}
			joined := ""
			for _, fe := range ve.Errors {
				joined += fe.Localize(i18n.English()) + "\n"
			}
			if !strings.Contains(joined, tc.wantMsg) {
				t.Errorf("error messages do not contain %q:\n%s", tc.wantMsg, joined)
			}
		})
	}
}
