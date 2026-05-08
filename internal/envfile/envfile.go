// Package envfile reads simple KEY=VALUE lines from .env-style files.
package envfile

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// ReadOr returns the value of key in path, or fallback when the file does
// not exist or the key is absent. Quotes around the value are stripped.
func ReadOr(path, key, fallback string) string {
	f, err := os.Open(path) //nolint:gosec // path is built from a trusted workspace dir.
	if err != nil {
		return fallback
	}
	defer func() { _ = f.Close() }()
	prefix := key + "="
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		v := strings.TrimPrefix(line, prefix)
		v = strings.Trim(v, `"'`)
		return v
	}
	return fallback
}

// ConfirmYN writes prompt to out, reads a single line from in, and returns
// true iff the answer is "y" or "Y" (after trimming whitespace).
func ConfirmYN(in io.Reader, out io.Writer, prompt string) (bool, error) {
	if _, err := fmt.Fprint(out, prompt); err != nil {
		return false, fmt.Errorf("prompt: %w", err)
	}
	scanner := bufio.NewScanner(in)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return false, fmt.Errorf("read confirm: %w", err)
		}
		return false, nil
	}
	answer := strings.TrimSpace(scanner.Text())
	return answer == "y" || answer == "Y", nil
}
