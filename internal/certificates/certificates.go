// Package certificates validates and lists PEM certificates under the
// workspace-docker certs/ directory. It mirrors lib/certificates.sh so the
// `wsd certificates` subcommand and bash callers share a single source.
package certificates

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ErrInvalid is returned by Validate when the file is not a valid PEM cert.
var ErrInvalid = errors.New("invalid certificate")

// Validate checks that path is an existing .crt file containing at least one
// PEM-encoded X.509 certificate. The check matches lib/certificates.sh:
// extension is .crt, first line begins with "-----BEGIN CERTIFICATE-----"
// and the file contains "-----END CERTIFICATE-----" on some line.
func Validate(path string) error {
	if !strings.HasSuffix(path, ".crt") {
		return ErrInvalid
	}
	f, err := os.Open(path) //nolint:gosec // path provided by trusted caller
	if err != nil {
		return ErrInvalid
	}
	defer func() { _ = f.Close() }()

	sc := bufio.NewScanner(f)
	first := true
	hasEnd := false
	for sc.Scan() {
		line := sc.Text()
		if first {
			if !strings.HasPrefix(line, "-----BEGIN CERTIFICATE-----") {
				return ErrInvalid
			}
			first = false
		}
		if line == "-----END CERTIFICATE-----" {
			hasEnd = true
		}
	}
	if first || !hasEnd {
		return ErrInvalid
	}
	return nil
}

// List returns the basename of every valid certificate under <projectRoot>/certs.
// Output is sorted lexicographically (matches the bash glob expansion order).
func List(projectRoot string) ([]string, error) {
	dir := filepath.Join(projectRoot, "certs")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err //nolint:wrapcheck // direct os err is descriptive enough for callers
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".crt") {
			continue
		}
		full := filepath.Join(dir, name)
		if err := Validate(full); err != nil {
			continue
		}
		out = append(out, name)
	}
	sort.Strings(out)
	return out, nil
}

// Has reports whether at least one valid certificate exists under certs/.
func Has(projectRoot string) bool {
	list, err := List(projectRoot)
	if err != nil {
		return false
	}
	return len(list) > 0
}
