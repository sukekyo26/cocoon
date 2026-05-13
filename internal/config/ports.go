package config

import (
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"sort"
	"strconv"
	"strings"
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

// IsLong reports whether this entry should be rendered as a YAML mapping.
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

// DevcontainerPortEntries skips entries that cannot reduce to a single TCP
// integer (port ranges, mode=host, protocol=udp). If warn is non-nil each
// skip is announced so the user can reconcile docker-compose-only ports
// with devcontainer output.
func DevcontainerPortEntries(forward []any, warn io.Writer) []int {
	if len(forward) == 0 {
		return nil
	}
	out := make([]int, 0, len(forward))
	for i, raw := range forward {
		port, ok, reason := devcontainerPort(raw)
		if !ok {
			if warn != nil {
				fmt.Fprintf(warn,
					"WARNING: ports.forward[%d] %s; skipping for devcontainer.json forwardPorts.\n",
					i, reason)
			}
			continue
		}
		out = append(out, port)
	}
	return out
}

// devcontainerPort returns (port, true, "") on success or (0, false, reason)
// when the entry cannot be expressed as a single integer.
func devcontainerPort(raw any) (int, bool, string) {
	switch v := raw.(type) {
	case string:
		return shortFormHostPort(v)
	case map[string]any:
		return longFormHostPort(v)
	default:
		return 0, false, fmt.Sprintf("has unsupported type %T", raw)
	}
}

func shortFormHostPort(s string) (int, bool, string) {
	m := rxPortShortForm.FindStringSubmatch(s)
	if m == nil {
		return 0, false, fmt.Sprintf("= %q is not a valid short-form port", s)
	}
	// devcontainer.json's forwardPorts is TCP-only — VS Code's port
	// tunnel does not carry UDP, so a UDP entry registered here would
	// show up in the Ports panel but silently fail to forward.
	if proto := m[rxPortShortForm.SubexpIndex("proto")]; proto == "udp" {
		return 0, false, fmt.Sprintf("= %q uses protocol = \"udp\"", s)
	}
	host := m[rxPortShortForm.SubexpIndex("host")]
	container := m[rxPortShortForm.SubexpIndex("container")]
	pick := host
	if pick == "" {
		pick = container
	}
	if strings.Contains(pick, "-") {
		return 0, false, fmt.Sprintf("= %q uses a port range", s)
	}
	n, err := strconv.Atoi(pick)
	if err != nil {
		return 0, false, fmt.Sprintf("= %q has unparseable port", s)
	}
	return n, true, ""
}

func longFormHostPort(m map[string]any) (int, bool, string) {
	if mode, ok := stringField(m, "mode"); ok && mode == "host" {
		return 0, false, "uses mode = \"host\""
	}
	// Symmetric with the short-form check above: devcontainer.json's
	// forwardPorts cannot carry UDP, so a long-form entry with
	// protocol = "udp" is skipped with the same warning class.
	if proto, ok := stringField(m, "protocol"); ok && proto == "udp" {
		return 0, false, "uses protocol = \"udp\""
	}
	if v, ok := m["published"]; ok {
		if n, parsed := publishedHostPort(v); parsed {
			return n, true, ""
		}
		if s, ok := v.(string); ok && strings.Contains(s, "-") {
			return 0, false, fmt.Sprintf("uses a published range %q", s)
		}
	}
	if v, ok := m["target"]; ok {
		if n, ok := intField(v); ok {
			return n, true, ""
		}
	}
	return 0, false, "is missing target/published"
}

// publishedHostPort returns (n, true) for int / int64 / integer-valued
// float64 / numeric string ("8080"); (0, false) for string ranges
// ("8000-8010") so the caller can skip the entry for devcontainer.json.
func publishedHostPort(v any) (int, bool) {
	if n, ok := intField(v); ok {
		return n, true
	}
	if s, ok := v.(string); ok && !strings.Contains(s, "-") {
		if n, err := strconv.Atoi(s); err == nil {
			return n, true
		}
	}
	return 0, false
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
func validatePortsForward(a *errAccumulator, forward []any) {
	for i, raw := range forward {
		idx := strconv.Itoa(i)
		switch v := raw.(type) {
		case string:
			if msg, ok := shortFormReason(v); !ok {
				a.add(msg, "forward", idx)
			}
		case map[string]any:
			validateLongForm(a, v, idx)
		case int, int64, float64:
			a.add(
				"int form was removed; use a string (\"3000:3000\") "+
					"or table ({ target = 3000 })",
				"forward", idx,
			)
		default:
			a.add(fmt.Sprintf("must be string or table (got %T)", raw),
				"forward", idx)
		}
	}
}

// ValidateShortForm wraps ErrPortShortForm on reject. The schema validator
// and `cocoon init` prompt share this rule so a string init accepts cannot
// be rejected later by gen.
func ValidateShortForm(s string) error {
	msg, ok := shortFormReason(s)
	if ok {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrPortShortForm, msg)
}

// shortFormReason returns ("", true) on accept and (reason, false) on
// reject. Shared by ValidateShortForm and validatePortsForward.
func shortFormReason(s string) (string, bool) {
	m := rxPortShortForm.FindStringSubmatch(s)
	if m == nil {
		return fmt.Sprintf(
			"%q does not match docker-compose short form "+
				"[HOST_IP:][HOST:]CONTAINER[/PROTOCOL]", s), false
	}
	for _, name := range []string{"host", "container"} {
		raw := m[rxPortShortForm.SubexpIndex(name)]
		if raw == "" {
			continue
		}
		for _, part := range strings.Split(raw, "-") {
			n, err := strconv.Atoi(part)
			if err != nil || n < portMin || n > portMax {
				return fmt.Sprintf("port must be in [%d,%d] (got %q)",
					portMin, portMax, part), false
			}
		}
	}
	if ip := m[rxPortShortForm.SubexpIndex("ip")]; ip != "" {
		bare := strings.TrimSuffix(strings.TrimPrefix(ip, "["), "]")
		if net.ParseIP(bare) == nil {
			return fmt.Sprintf("%q is not a valid IPv4/IPv6 address", ip), false
		}
	}
	return "", true
}

func validateLongForm(a *errAccumulator, m map[string]any, idx string) {
	if rejectUnknownLongFormKeys(a, m, idx) {
		return
	}
	if _, present := m["target"]; !present {
		a.add("target is required", "forward", idx, "target")
	}
	validateLongFormPortFields(a, m, idx)
	validateLongFormStringField(a, m, idx, "host_ip", validateHostIP)
	validateLongFormStringField(a, m, idx, "protocol", validateEnum("protocol", allowedProtocols))
	validateLongFormStringField(a, m, idx, "mode", validateEnum("mode", allowedModes))
}

func rejectUnknownLongFormKeys(a *errAccumulator, m map[string]any, idx string) bool {
	for k := range m {
		if _, ok := allowedLongFormKeys[k]; !ok {
			a.add(fmt.Sprintf(
				"unknown key %q (allowed: %s)",
				k, strings.Join(longFormKeyOrder, ", ")),
				"forward", idx)
			return true
		}
	}
	return false
}

func validateLongFormPortFields(a *errAccumulator, m map[string]any, idx string) {
	if v, ok := m["target"]; ok {
		validateIntPortField(a, v, idx, "target")
	}
	if v, ok := m["published"]; ok {
		validateLongFormPublished(a, v, idx)
	}
}

func validateIntPortField(a *errAccumulator, v any, idx, key string) {
	n, parsed := intField(v)
	if !parsed {
		a.add(fmt.Sprintf("%s must be an integer", key), "forward", idx, key)
		return
	}
	if n < portMin || n > portMax {
		a.add(fmt.Sprintf("%s must be in [%d,%d]", key, portMin, portMax),
			"forward", idx, key)
	}
}

// validateLongFormPublished accepts an integer or string matching
// `\d+(?:-\d+)?` (docker-compose's range form `published = "8000-8010"`).
// Each numeric component is bounded by [portMin, portMax].
func validateLongFormPublished(a *errAccumulator, v any, idx string) {
	if _, ok := intField(v); ok {
		validateIntPortField(a, v, idx, "published")
		return
	}
	s, ok := v.(string)
	if !ok {
		a.add("published must be an integer or a string", "forward", idx, "published")
		return
	}
	if !rxLongFormPublishedString.MatchString(s) {
		a.add(fmt.Sprintf(
			"published string %q must be a port or numeric range like \"8000-8010\"", s),
			"forward", idx, "published")
		return
	}
	for _, part := range strings.Split(s, "-") {
		n, err := strconv.Atoi(part)
		if err != nil || n < portMin || n > portMax {
			a.add(fmt.Sprintf("published port must be in [%d,%d] (got %q)",
				portMin, portMax, part),
				"forward", idx, "published")
			return
		}
	}
}

// validateLongFormStringField is a no-op when the key is absent. The
// `check` callback returns the message to emit (or "" to accept).
func validateLongFormStringField(
	a *errAccumulator,
	m map[string]any,
	idx, name string,
	check func(string) string,
) {
	if v, ok := stringField(m, name); ok {
		if msg := check(v); msg != "" {
			a.add(msg, "forward", idx, name)
		}
		return
	}
	if _, present := m[name]; present {
		a.add(name+" must be a string", "forward", idx, name)
	}
}

func validateHostIP(v string) string {
	if net.ParseIP(v) == nil {
		return fmt.Sprintf("host_ip %q is not a valid IP address", v)
	}
	return ""
}

func validateEnum(field string, allowed map[string]struct{}) func(string) string {
	return func(v string) string {
		if _, ok := allowed[v]; ok {
			return ""
		}
		return fmt.Sprintf("%s must be one of %s (got %q)", field, sortedKeys(allowed), v)
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

func sortedKeys(m map[string]struct{}) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, "/")
}
