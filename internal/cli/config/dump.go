package configcli

import (
	"fmt"
	"io"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/sukekyo26/cocoon/internal/logx"
)

// cmdDumpDevcontainer prints the [devcontainer] section as canonical TOML
// text via the standard go-toml/v2 marshaller. Map keys come out in
// alphabetical order, which is the documented stable choice for this
// command (Go map iteration is random).
func cmdDumpDevcontainer(args []string, stdout, stderr io.Writer) error {
	log := logx.New(stdout, stderr)
	if err := requireArgs(args, 1, "dump-devcontainer", stderr); err != nil {
		return err
	}
	data, err := decodeRaw(args[0])
	if err != nil {
		log.Errorf("ERROR: %s", err)
		return ErrFailure
	}
	dc, ok := data["devcontainer"].(map[string]any)
	if !ok || len(dc) == 0 {
		return nil
	}
	body, err := toml.Marshal(map[string]any{"devcontainer": dc})
	if err != nil {
		return fmt.Errorf("marshal devcontainer: %w", err)
	}
	if _, err := stdout.Write(body); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return nil
}

// cmdDumpRepositories prints [repositories] as canonical inline TOML.
func cmdDumpRepositories(args []string, stdout, stderr io.Writer) error {
	log := logx.New(stdout, stderr)
	if err := requireArgs(args, 1, "dump-repositories", stderr); err != nil {
		return err
	}
	file := args[0]
	if !isFile(file) {
		log.Errorf("ERROR: File not found: %s", file)
		return ErrFailure
	}
	data, err := decodeRaw(file)
	if err != nil {
		log.Errorf("ERROR: %s", err)
		return ErrFailure
	}
	repos, ok := data["repositories"].(map[string]any)
	if !ok {
		return nil
	}
	clone := asSliceAny(repos["clone"])
	if len(clone) == 0 {
		return nil
	}
	lines := []string{"[repositories]", "clone = ["}
	for _, raw := range clone {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		lines = append(lines, "  "+formatRepoEntryInline(entry)+",")
	}
	lines = append(lines, "]")
	log.Info(strings.Join(lines, "\n"))
	return nil
}

// repoEntry holds one [[repositories.clone]] row in a typed shape so the
// inline-formatter can rely on Go's struct field semantics (presence,
// zero-value checks) instead of map[string]any lookups scattered across
// the call site.
type repoEntry struct {
	URL               string
	Path              string
	Branch            string
	Depth             int
	HasDepth          bool
	RecurseSubmodules bool
}

// formatRepoEntryInline returns one entry as an inline TOML table
// `{ url = "...", path = "...", ... }`. Optional fields are omitted when
// empty / absent. String values are escaped via go-toml/v2's encoder so
// embedded quotes / backslashes / control characters are handled correctly.
func formatRepoEntryInline(entry map[string]any) string {
	e := repoEntry{
		URL:               asString(entry["url"], ""),
		Path:              asString(entry["path"], ""),
		Branch:            asString(entry["branch"], ""),
		Depth:             0,
		HasDepth:          false,
		RecurseSubmodules: asBool(entry["recurse_submodules"], false),
	}
	if v, ok := entry["depth"]; ok {
		e.Depth = asInt(v, 0)
		e.HasDepth = true
	}

	parts := []string{fmt.Sprintf("url = %s", tomlString(e.URL))}
	if e.Path != "" {
		parts = append(parts, fmt.Sprintf("path = %s", tomlString(e.Path)))
	}
	if e.Branch != "" {
		parts = append(parts, fmt.Sprintf("branch = %s", tomlString(e.Branch)))
	}
	if e.HasDepth {
		parts = append(parts, fmt.Sprintf("depth = %d", e.Depth))
	}
	if e.RecurseSubmodules {
		parts = append(parts, "recurse_submodules = true")
	}
	return "{ " + strings.Join(parts, ", ") + " }"
}

// tomlString returns s as a TOML basic-string literal, including the
// surrounding double quotes. Delegates to go-toml/v2 so all escape
// sequences (\, ", \b, \t, \n, \f, \r, \uXXXX) are produced correctly.
func tomlString(s string) string {
	b, err := toml.Marshal(map[string]string{"k": s})
	if err != nil {
		// toml.Marshal of a single string into a 1-key map cannot fail in
		// practice; fall back to a manual escape if it ever does.
		return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(s) + `"`
	}
	// Output looks like: "k = \"<encoded>\"\n"
	const prefix = "k = "
	out := strings.TrimRight(string(b), "\n")
	return strings.TrimPrefix(out, prefix)
}

// pythonJSON converts compact encoding/json output to Python json.dumps()
// default formatting (", " and ": " separators) without disturbing string
// literals. This is a small targeted rewriter sufficient for our nested
// list-of-maps shape; arbitrary JSON is out of scope.
func pythonJSON(in []byte) string {
	out := make([]byte, 0, len(in)*2)
	inStr := false
	escape := false
	for _, c := range in {
		switch {
		case escape:
			out = append(out, c)
			escape = false
			continue
		case c == '\\' && inStr:
			out = append(out, c)
			escape = true
			continue
		case c == '"':
			out = append(out, c)
			inStr = !inStr
			continue
		}
		if inStr {
			out = append(out, c)
			continue
		}
		switch c {
		case ',':
			out = append(out, ',', ' ')
		case ':':
			out = append(out, ':', ' ')
		default:
			out = append(out, c)
		}
	}
	return string(out)
}
