package config_test

import (
	"bytes"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/sukekyo26/cocoon/internal/config"
)

// TestPortShortFormPattern guards the JSON Schema pattern against drift from
// the Go validator. Both must accept and reject the same set of strings so
// editor-side validation matches `wsd config validate-workspace`.
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
		name     string
		in       []any
		want     []int
		wantWarn string // substring expected in warning output
	}{
		{
			name: "container_only",
			in:   []any{"3000"},
			want: []int{3000},
		},
		{
			name: "host_container",
			in:   []any{"8080:80"},
			want: []int{8080},
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
			name:     "range_skipped",
			in:       []any{"3000-3005:3000-3005"},
			want:     []int{},
			wantWarn: "uses a port range",
		},
		{
			name: "long_form_published",
			in: []any{
				map[string]any{"target": int64(5432), "published": int64(15432)},
			},
			want: []int{15432},
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
			want:     []int{},
			wantWarn: "uses mode = \"host\"",
		},
		{
			name: "long_form_published_string_single",
			in: []any{
				map[string]any{"target": int64(8080), "published": "8080"},
			},
			want: []int{8080},
		},
		{
			name: "long_form_published_string_range",
			in: []any{
				map[string]any{"target": int64(8000), "published": "8000-8010"},
			},
			want:     []int{},
			wantWarn: "uses a published range",
		},
		{
			name: "mixed",
			in: []any{
				"3000",
				map[string]any{"target": int64(5432)},
				"3000-3005:3000-3005",
				"127.0.0.1:8080:8080/udp",
			},
			want:     []int{3000, 5432, 8080},
			wantWarn: "uses a port range",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var warn bytes.Buffer
			got := config.DevcontainerPortEntries(tc.in, &warn)
			if got == nil {
				got = []int{}
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("ports mismatch (-want +got):\n%s", diff)
			}
			if tc.wantWarn != "" && !strings.Contains(warn.String(), tc.wantWarn) {
				t.Errorf("warn = %q, want to contain %q", warn.String(), tc.wantWarn)
			}
			if tc.wantWarn == "" && warn.Len() != 0 {
				t.Errorf("unexpected warn = %q", warn.String())
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
			ve, ok := config.AsValidationError(err)
			if !ok {
				t.Fatalf("expected ValidationError")
			}
			joined := ""
			for _, fe := range ve.Errors {
				joined += fe.Message + "\n"
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
			ve, ok := config.AsValidationError(err)
			if !ok {
				t.Fatalf("expected ValidationError, got %T", err)
			}
			joined := ""
			for _, fe := range ve.Errors {
				joined += fe.Message + "\n"
			}
			if !strings.Contains(joined, tc.wantMsg) {
				t.Errorf("error messages do not contain %q:\n%s", tc.wantMsg, joined)
			}
		})
	}
}
