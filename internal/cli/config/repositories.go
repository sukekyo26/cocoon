package configcli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/sukekyo26/cocoon/internal/config"
	"github.com/sukekyo26/cocoon/internal/logx"
)

// cmdRepositories emits [repositories].clone as a normalized JSON array.
//
// Each entry has at minimum {url, path}; optional keys (branch, depth,
// recurse_submodules) are included only when present in the source TOML.
// The path is resolved via config.ResolveRepoPath so consumers receive the
// final target directory regardless of whether the user specified path/.
func cmdRepositories(args []string, stdout, stderr io.Writer) error {
	log := logx.New(stdout, stderr)
	if err := requireArgs(args, 1, "repositories", stderr); err != nil {
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

	repos := asMap(data["repositories"])
	clone := asSliceAny(repos["clone"])
	type repoOut struct {
		URL               string `json:"url"`
		Path              string `json:"path"`
		Branch            string `json:"branch,omitempty"`
		Depth             *int   `json:"depth,omitempty"`
		RecurseSubmodules *bool  `json:"recurse_submodules,omitempty"`
	}
	out := make([]repoOut, 0, len(clone))
	for _, raw := range clone {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		url := asString(entry["url"], "")
		path := ""
		if p, hasPath := entry["path"]; hasPath {
			path = asString(p, "")
		}
		ro := repoOut{
			URL:               url,
			Path:              config.ResolveRepoPath(path, url),
			Branch:            "",
			Depth:             nil,
			RecurseSubmodules: nil,
		}
		if v, ok := entry["branch"]; ok {
			ro.Branch = asString(v, "")
		}
		if v, ok := entry["depth"]; ok {
			d := asInt(v, 0)
			ro.Depth = &d
		}
		if v, ok := entry["recurse_submodules"]; ok {
			b := asBool(v, false)
			ro.RecurseSubmodules = &b
		}
		out = append(out, ro)
	}

	enc, err := json.Marshal(out)
	if err != nil {
		return fmt.Errorf("marshal repositories: %w", err)
	}
	// Mirror Python's json.dumps default formatting: ", " and ": " separators
	// with an ASCII payload. encoding/json uses no spaces by default; expand.
	pretty := pythonJSON(enc)
	log.Info(pretty)
	return nil
}

func cmdFormatRepositories(args []string, stdout, stderr io.Writer) error {
	log := logx.New(stdout, stderr)
	if err := requireArgs(args, 1, "format-repositories", stderr); err != nil {
		return err
	}
	src := args[0]
	var payload []byte
	if src == "-" {
		buf, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}
		payload = buf
	} else {
		if !isFile(src) {
			log.Errorf("ERROR: File not found: %s", src)
			return ErrFailure
		}
		buf, err := os.ReadFile(src) //nolint:gosec // path supplied by trusted caller.
		if err != nil {
			log.Errorf("ERROR: %s", err)
			return ErrFailure
		}
		payload = buf
	}
	if len(payload) == 0 || onlyWhitespace(payload) {
		return nil
	}
	var entries []map[string]any
	if err := json.Unmarshal(payload, &entries); err != nil {
		log.Errorf("ERROR: Invalid JSON: %s", err)
		return ErrFailure
	}
	if len(entries) == 0 {
		return nil
	}
	lines := []string{"[repositories]", "clone = ["}
	for _, entry := range entries {
		lines = append(lines, "  "+formatRepoEntryInline(entry)+",")
	}
	lines = append(lines, "]")
	for _, l := range lines {
		log.Info(l)
	}
	return nil
}

func onlyWhitespace(b []byte) bool {
	for _, c := range b {
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			return false
		}
	}
	return true
}
