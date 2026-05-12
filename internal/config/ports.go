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

// ErrPortShortForm marks every rejection emitted by ValidateShortForm so
// callers (e.g. the init prompt and the `--ports` flag parser) can identify
// the failure class via errors.Is and surface their own usage prefix without
// double-wrapping the package's sentinel chain.
var ErrPortShortForm = errors.New("invalid port short form")

// PortShortFormPattern is the ECMA-262 regex used in workspace.schema.json
// for `[ports].forward` short-form strings:
//
//	[HOST_IP:][HOST:]CONTAINER[/PROTOCOL]
//
// HOST_IP is IPv4 (1.2.3.4) or [IPv6]; HOST and CONTAINER may be a single
// port or a numeric range (N or N-M); PROTOCOL is tcp|udp.
//
// Editors that consume the JSON Schema use this pattern for early validation
// and autocomplete. Keep this in sync with rxPortShortForm below — they
// must accept the same set of strings.
const PortShortFormPattern = `^(?:(?:(?:\d{1,3}\.){3}\d{1,3}|\[[\da-fA-F:]+\]):)?` +
	`(?:\d+(?:-\d+)?:)?` +
	`\d+(?:-\d+)?` +
	`(?:/(?:tcp|udp))?$`

// rxPortShortForm is the Go (RE2) version of PortShortFormPattern with named
// capture groups so validateShortForm and shortFormHostPort can extract the
// IP / host / container / protocol fragments.
var rxPortShortForm = regexp.MustCompile(
	`^(?:(?P<ip>(?:\d{1,3}\.){3}\d{1,3}|\[[\da-fA-F:]+\]):)?` +
		`(?:(?P<host>\d+(?:-\d+)?):)?` +
		`(?P<container>\d+(?:-\d+)?)` +
		`(?:/(?P<proto>tcp|udp))?$`,
)

// longFormKeyOrder fixes the YAML output order for long-form port entries so
// generated docker-compose.yml is deterministic.
var longFormKeyOrder = []string{"target", "published", "host_ip", "protocol", "mode"}

// allowedLongFormKeys is the set used to reject unknown keys at validation.
var allowedLongFormKeys = map[string]struct{}{
	"target":    {},
	"published": {},
	"host_ip":   {},
	"protocol":  {},
	"mode":      {},
}

// allowedProtocols / allowedModes mirror the docker-compose v3 spec subset.
var (
	allowedProtocols = map[string]struct{}{"tcp": {}, "udp": {}}
	allowedModes     = map[string]struct{}{"ingress": {}, "host": {}}
)

// rxLongFormPublishedString matches the string form of long-form `published`:
// a single port (`"8080"`) or a numeric range (`"8000-8010"`). Each numeric
// component is bounded by [portMin, portMax] in validateLongFormPublished.
var rxLongFormPublishedString = regexp.MustCompile(`^\d+(?:-\d+)?$`)

// ComposePort holds a normalized port entry ready for docker-compose YAML
// emission. Exactly one of Short / Long is populated.
type ComposePort struct {
	// Short carries the raw docker-compose short-form string ("3000:3000",
	// "127.0.0.1:5432:5432/tcp", "3000-3005:3000-3005", ...).
	Short string
	// Long carries the long-form keys (target / published / host_ip /
	// protocol / mode). Only valid keys are kept; values are normalized to
	// int (target / published) or string (host_ip / protocol / mode).
	Long map[string]any
}

// IsLong reports whether this entry should be rendered as a YAML mapping.
func (p ComposePort) IsLong() bool { return p.Long != nil }

// ComposePortEntries converts the raw [ports].forward array (decoded by
// pelletier/go-toml v2 as []any of strings or map[string]any) into a slice of
// ComposePort. Validation of individual entries is the caller's job —
// ComposePortEntries trusts that PortsSpec.validate has already run.
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

// DevcontainerPortEntries returns the published-port integers that
// devcontainer.json `forwardPorts` can express. Entries that cannot be
// reduced to a single integer (port ranges, mode=host) are skipped; if warn
// is non-nil each skip is announced as a single line so the user can
// reconcile their docker-compose-only ports with the devcontainer output.
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

// devcontainerPort extracts the host-side port from one entry. Returns
// (port, true, "") on success, or (0, false, reason) when the entry is not
// representable as a single integer.
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

// publishedHostPort extracts a single integer host port from a long-form
// `published` value. Returns (n, true) for int / int64 / integer-valued
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

// normalizeLongForm produces a map containing only the allowed long-form
// keys, with int / string values coerced from go-toml's decoded shapes.
// Unknown keys are dropped (validation already rejected them upstream).
//
// `target` is always int; `published` is preserved as int or string so the
// docker-compose range form (`published = "8000-8010"`) flows through to the
// generated YAML without losing the dash.
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

// LongFormKeyOrder returns the fixed key order for emitting long-form port
// mappings to YAML. Generators iterate over this slice and look up each key
// in ComposePort.Long.
func LongFormKeyOrder() []string {
	return append([]string{}, longFormKeyOrder...)
}

// validatePortsForward runs the per-entry validation that PortsSpec.validate
// dispatches into. It is exported only via PortsSpec; the function lives here
// so the parsing logic stays alongside the normalizer.
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

// ValidateShortForm reports whether s is a valid docker-compose short-form
// port mapping (regex shape + host/container in [portMin, portMax] +
// IPv4/IPv6 literal). Returns nil on accept; on reject returns an error that
// wraps ErrPortShortForm with a message naming the rule that failed. The
// `[ports].forward` schema validator and the `cocoon init` prompt share this
// single rule so a string that init accepts cannot be rejected later by gen.
func ValidateShortForm(s string) error {
	msg, ok := shortFormReason(s)
	if ok {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrPortShortForm, msg)
}

// shortFormReason returns ("", true) on accept and (humanReadableReason,
// false) on reject. Used by both ValidateShortForm (which wraps the reason
// in ErrPortShortForm for external callers) and the accumulator-flavored
// validateShortForm (which already carries field/idx context).
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

// validateLongFormPublished accepts either an integer (single port) or a
// string matching `\d+(?:-\d+)?`. The string form mirrors docker-compose's
// long-form spec for port ranges (`published = "8000-8010"`); each numeric
// component is bounded by [portMin, portMax].
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

// validateLongFormStringField applies check to m[name] when it is a string,
// emits a "must be a string" error when present but typed wrong, and is a
// no-op when the key is absent. The `check` callback returns the message to
// emit (or "" to accept).
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
