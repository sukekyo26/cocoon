package config

import (
	"errors"
	"fmt"
	"maps"
	"net"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/sukekyo26/cocoon/internal/i18n"
	"github.com/sukekyo26/cocoon/internal/warn"
)

// ErrPortShortForm marks every rejection from ValidateShortForm so callers
// (the init prompt and `--ports` flag parser) can identify the class via
// errors.Is and surface their own usage prefix without double-wrapping.
var ErrPortShortForm = errors.New("invalid port short form")

// PortShortFormPattern is the ECMA-262 regex used in workspace.schema.json
// for `[ports].forward` short-form strings:
//
//	[HOST_IP:][HOST:]CONTAINER[/PROTOCOL]
//
// HOST_IP is IPv4 (1.2.3.4) or [IPv6]; HOST and CONTAINER may be a single
// port or a numeric range (N or N-M); PROTOCOL is tcp|udp. Keep this in
// sync with rxPortShortForm below — they must accept the same set.
const PortShortFormPattern = `^(?:(?:(?:\d{1,3}\.){3}\d{1,3}|\[[\da-fA-F:]+\]):)?` +
	`(?:\d+(?:-\d+)?:)?` +
	`\d+(?:-\d+)?` +
	`(?:/(?:tcp|udp))?$`

// rxPortShortForm is the Go (RE2) twin of PortShortFormPattern with named
// capture groups for IP / host / container / protocol.
var rxPortShortForm = regexp.MustCompile(
	`^(?:(?P<ip>(?:\d{1,3}\.){3}\d{1,3}|\[[\da-fA-F:]+\]):)?` +
		`(?:(?P<host>\d+(?:-\d+)?):)?` +
		`(?P<container>\d+(?:-\d+)?)` +
		`(?:/(?P<proto>tcp|udp))?$`,
)

// longFormKeyOrder fixes YAML output order so docker-compose.yml is
// deterministic.
var longFormKeyOrder = []string{"target", "published", "host_ip", "protocol", "mode"}

var allowedLongFormKeys = map[string]struct{}{
	"target":    {},
	"published": {},
	"host_ip":   {},
	"protocol":  {},
	"mode":      {},
}

// docker-compose v3 subset.
var (
	allowedProtocols = map[string]struct{}{"tcp": {}, "udp": {}}
	allowedModes     = map[string]struct{}{"ingress": {}, "host": {}}
)

// rxLongFormPublishedString matches a single port or numeric range. Each
// component is bounded by [portMin, portMax] in validateLongFormPublished.
var rxLongFormPublishedString = regexp.MustCompile(`^\d+(?:-\d+)?$`)

// ComposePort is a normalized port entry. Exactly one of Short / Long is
// populated.
type ComposePort struct {
	// Short is the raw docker-compose short-form ("3000:3000",
	// "127.0.0.1:5432:5432/tcp", "3000-3005:3000-3005", ...).
	Short string
	// Long holds the long-form keys (target / published / host_ip /
	// protocol / mode). Values are normalized to int (target / published)
	// or string (host_ip / protocol / mode).
	Long map[string]any
}

func (p ComposePort) IsLong() bool { return p.Long != nil }

// ComposePortEntries trusts that PortsSpec.validate has already run; it
// only normalizes the raw decoded shapes.
func ComposePortEntries(forward []any) []ComposePort {
	if len(forward) == 0 {
		return nil
	}
	out := make([]ComposePort, 0, len(forward))
	for _, raw := range forward {
		switch v := raw.(type) {
		case string:
			out = append(out, ComposePort{Short: v, Long: nil})
		case map[string]any:
			out = append(out, ComposePort{Short: "", Long: normalizeLongForm(v)})
		}
	}
	return out
}

// DevcontainerPortEntries returns the container-side port of each forward
// entry. devcontainer.json's forwardPorts lists ports inside the container
// (where the app listens), which VS Code forwards to the local machine — not
// the published host port. Entries that cannot reduce to a single TCP integer
// (container-side port ranges, mode=host, protocol=udp) are skipped; each skip
// is recorded in sink (nil drops them) so the user can reconcile
// docker-compose-only ports with devcontainer output.
func DevcontainerPortEntries(forward []any, sink *warn.Sink) []int {
	if len(forward) == 0 {
		return nil
	}
	out := make([]int, 0, len(forward))
	for i, raw := range forward {
		port, ok, reason := devcontainerPort(raw)
		if !ok {
			sink.Warn(warn.PortSkip, i, reason)
			continue
		}
		out = append(out, port)
	}
	return out
}

// devcontainerPort returns (container port, true, zero Ref) on success or
// (0, false, reason) when the entry cannot be expressed as a single integer.
// The reason is a warn.Ref so the drain site localizes it.
func devcontainerPort(raw any) (int, bool, warn.Ref) {
	switch v := raw.(type) {
	case string:
		return shortFormContainerPort(v)
	case map[string]any:
		return longFormContainerPort(v)
	default:
		return 0, false, warn.Reason(warn.PortReasonType, fmt.Sprintf("%T", raw))
	}
}

func shortFormContainerPort(s string) (int, bool, warn.Ref) {
	m := rxPortShortForm.FindStringSubmatch(s)
	if m == nil {
		return 0, false, warn.Reason(warn.PortReasonShortInvalid, s)
	}
	// devcontainer.json's forwardPorts is TCP-only — VS Code's port
	// tunnel does not carry UDP, so a UDP entry registered here would
	// show up in the Ports panel but silently fail to forward.
	if proto := m[rxPortShortForm.SubexpIndex("proto")]; proto == "udp" {
		return 0, false, warn.Reason(warn.PortReasonShortUDP, s)
	}
	// forwardPorts lists the port inside the container (where the app
	// listens), not the published host port — for "30002:3000" VS Code
	// forwards container port 3000, not 30002.
	container := m[rxPortShortForm.SubexpIndex("container")]
	if strings.Contains(container, "-") {
		return 0, false, warn.Reason(warn.PortReasonRange, s)
	}
	n, err := strconv.Atoi(container)
	if err != nil {
		return 0, false, warn.Reason(warn.PortReasonUnparseable, s)
	}
	return n, true, warn.Reason("")
}

func longFormContainerPort(m map[string]any) (int, bool, warn.Ref) {
	if mode, ok := stringField(m, "mode"); ok && mode == "host" {
		return 0, false, warn.Reason(warn.PortReasonHostMode)
	}
	// Symmetric with the short-form check above: devcontainer.json's
	// forwardPorts cannot carry UDP, so a long-form entry with
	// protocol = "udp" is skipped with the same warning class.
	if proto, ok := stringField(m, "protocol"); ok && proto == "udp" {
		return 0, false, warn.Reason(warn.PortReasonLongUDP)
	}
	// `target` is the container-side port that VS Code forwards; `published`
	// is the host port and is irrelevant to forwardPorts (see
	// shortFormContainerPort).
	v, ok := m["target"]
	if !ok {
		return 0, false, warn.Reason(warn.PortReasonMissingTarget)
	}
	n, ok := intField(v)
	if !ok {
		return 0, false, warn.Reason(warn.PortReasonNonIntegerTarget, v)
	}
	return n, true, warn.Reason("")
}

// normalizeLongForm keeps only allowed keys. `target` is always int;
// `published` is preserved as int or string so the docker-compose range
// form (`published = "8000-8010"`) flows through to the generated YAML
// without losing the dash.
func normalizeLongForm(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for _, k := range longFormKeyOrder {
		v, ok := in[k]
		if !ok {
			continue
		}
		switch k {
		case "target":
			if n, ok := intField(v); ok {
				out[k] = n
			}
		case "published":
			if n, ok := intField(v); ok {
				out[k] = n
			} else if s, ok := v.(string); ok {
				out[k] = s
			}
		default:
			if s, ok := v.(string); ok {
				out[k] = s
			}
		}
	}
	return out
}

// LongFormKeyOrder returns a copy of the fixed YAML emission order.
func LongFormKeyOrder() []string {
	return append([]string{}, longFormKeyOrder...)
}

// validatePortsForward keeps the per-entry parsing alongside the normalizer.
func validatePortsForward(a *Accumulator, forward []any) {
	for i, raw := range forward {
		idx := strconv.Itoa(i)
		switch v := raw.(type) {
		case string:
			if code, args, ok := shortFormReason(v); !ok {
				a.AddCode(code, args, "forward", idx)
			}
		case map[string]any:
			validateLongForm(a, v, idx)
		case int, int64, float64:
			a.AddCode(
				"err_portfld_int_form_removed",
				nil,
				"forward", idx,
			)
		default:
			a.AddCode("err_portfld_must_be_string_or_table", []any{raw},
				"forward", idx)
		}
	}
}

// ValidateShortForm reports a rejection as a localizable error on reject (nil
// on accept). The schema validator and `cocoon init` prompt share this rule so
// a string init accepts cannot be rejected later by gen. The returned error
// wraps ErrPortShortForm for errors.Is and renders the reason in the active
// language at the CLI boundary (the `--ports` flag path), not frozen to English.
func ValidateShortForm(s string) error {
	code, args, ok := shortFormReason(s)
	if ok {
		return nil
	}
	return &portShortFormError{code: code, args: args}
}

// portShortFormError is ValidateShortForm's localizable rejection: it wraps
// ErrPortShortForm so callers classify via errors.Is, and carries the reason as
// a catalog (code, args) pair so the boundary renders it in the active language
// (i18n.Localizer). Error() renders English for logs / non-boundary callers.
type portShortFormError struct {
	code string
	args []any
}

func (e *portShortFormError) Error() string                   { return i18n.English().Msg(e.code, e.args...) }
func (e *portShortFormError) Localize(c *i18n.Catalog) string { return c.Msg(e.code, e.args...) }
func (*portShortFormError) Unwrap() error                     { return ErrPortShortForm }

// shortFormReason returns ("", nil, true) on accept and (code, args, false) on
// reject. Shared by ValidateShortForm and validatePortsForward so the same rule
// drives both the localized validation error and the init prompt.
func shortFormReason(s string) (code string, args []any, ok bool) {
	m := rxPortShortForm.FindStringSubmatch(s)
	if m == nil {
		return "err_portfld_short_form_nomatch", []any{s}, false
	}
	for _, name := range []string{"host", "container"} {
		raw := m[rxPortShortForm.SubexpIndex(name)]
		if raw == "" {
			continue
		}
		for _, part := range strings.Split(raw, "-") {
			n, err := strconv.Atoi(part)
			if err != nil || n < portMin || n > portMax {
				return "err_portfld_short_form_range", []any{portMin, portMax, part}, false
			}
		}
	}
	if ip := m[rxPortShortForm.SubexpIndex("ip")]; ip != "" {
		bare := strings.TrimSuffix(strings.TrimPrefix(ip, "["), "]")
		if net.ParseIP(bare) == nil {
			return "err_portfld_short_form_ip", []any{ip}, false
		}
	}
	return "", nil, true
}

func validateLongForm(a *Accumulator, m map[string]any, idx string) {
	if rejectUnknownLongFormKeys(a, m, idx) {
		return
	}
	if _, present := m["target"]; !present {
		a.AddCode("err_portfld_target_required", nil, "forward", idx, "target")
	}
	validateLongFormPortFields(a, m, idx)
	validateLongFormStringField(a, m, idx, "host_ip", validateHostIP)
	validateLongFormStringField(a, m, idx, "protocol", validateEnum("protocol", allowedProtocols))
	validateLongFormStringField(a, m, idx, "mode", validateEnum("mode", allowedModes))
}

func rejectUnknownLongFormKeys(a *Accumulator, m map[string]any, idx string) bool {
	for k := range m {
		if _, ok := allowedLongFormKeys[k]; !ok {
			a.AddCode("err_portfld_unknown_key",
				[]any{k, strings.Join(longFormKeyOrder, ", ")},
				"forward", idx)
			return true
		}
	}
	return false
}

func validateLongFormPortFields(a *Accumulator, m map[string]any, idx string) {
	if v, ok := m["target"]; ok {
		validateIntPortField(a, v, idx, "target")
	}
	if v, ok := m["published"]; ok {
		validateLongFormPublished(a, v, idx)
	}
}

func validateIntPortField(a *Accumulator, v any, idx, key string) {
	n, parsed := intField(v)
	if !parsed {
		a.AddCode("err_portfld_must_be_integer", []any{key}, "forward", idx, key)
		return
	}
	if n < portMin || n > portMax {
		a.AddCode("err_portfld_must_be_in_range", []any{key, portMin, portMax},
			"forward", idx, key)
	}
}

// validateLongFormPublished accepts an integer or string matching
// `\d+(?:-\d+)?` (docker-compose's range form `published = "8000-8010"`).
// Each numeric component is bounded by [portMin, portMax].
func validateLongFormPublished(a *Accumulator, v any, idx string) {
	if _, ok := intField(v); ok {
		validateIntPortField(a, v, idx, "published")
		return
	}
	s, ok := v.(string)
	if !ok {
		a.AddCode("err_portfld_published_int_or_string", nil, "forward", idx, "published")
		return
	}
	if !rxLongFormPublishedString.MatchString(s) {
		a.AddCode("err_portfld_published_string_form",
			[]any{s},
			"forward", idx, "published")
		return
	}
	for _, part := range strings.Split(s, "-") {
		n, err := strconv.Atoi(part)
		if err != nil || n < portMin || n > portMax {
			a.AddCode("err_portfld_published_port_range",
				[]any{portMin, portMax, part},
				"forward", idx, "published")
			return
		}
	}
}

// validateLongFormStringField is a no-op when the key is absent. The `check`
// callback returns ("", nil) to accept or (code, args) to emit a localized
// failure.
func validateLongFormStringField(
	a *Accumulator,
	m map[string]any,
	idx, name string,
	check func(string) (string, []any),
) {
	if v, ok := stringField(m, name); ok {
		if code, args := check(v); code != "" {
			a.AddCode(code, args, "forward", idx, name)
		}
		return
	}
	if _, present := m[name]; present {
		a.AddCode("err_portfld_long_form_value_string", []any{name}, "forward", idx, name)
	}
}

func validateHostIP(v string) (string, []any) {
	if net.ParseIP(v) == nil {
		return "err_portfld_host_ip_invalid", []any{v}
	}
	return "", nil
}

func validateEnum(field string, allowed map[string]struct{}) func(string) (string, []any) {
	return func(v string) (string, []any) {
		if _, ok := allowed[v]; ok {
			return "", nil
		}
		return "err_portfld_enum", []any{field, strings.Join(slices.Sorted(maps.Keys(allowed)), "/"), v}
	}
}

func intField(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		if float64(int(n)) == n {
			return int(n), true
		}
	}
	return 0, false
}

func stringField(m map[string]any, k string) (string, bool) {
	if v, ok := m[k]; ok {
		if s, ok := v.(string); ok {
			return s, true
		}
	}
	return "", false
}
