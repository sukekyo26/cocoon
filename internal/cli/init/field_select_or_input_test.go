//nolint:testpackage // white-box test for unexported selectOrInputField rendering.
package initcli

import (
	"bytes"
	"strings"
	"testing"
)

// TestSelectOrInputField_AccessibleHeaderIncludesURLLine pins down the new
// URLLine setter: when set, RunAccessible's printed header carries the URL
// on its own line under the description. The accessible path is the
// rendering surface that doesn't depend on a live tty / lipgloss theme.
func TestSelectOrInputField_AccessibleHeaderIncludesURLLine(t *testing.T) {
	t.Parallel()
	var target string
	f := newSelectOrInputField("k", &target, []string{"LATEST"}, "Other").
		Title("Pin version for go").
		Description("desc here").
		URLLine("https://github.com/golang/go")

	var buf bytes.Buffer
	f.printAccessibleHeader(&buf)
	got := buf.String()
	for _, want := range []string{
		"Pin version for go",
		"desc here",
		"https://github.com/golang/go",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("accessible header missing %q\n--- got ---\n%s", want, got)
		}
	}
	// URL line should be on its own line, between description and the
	// suggestion list.
	descIdx := strings.Index(got, "desc here")
	urlIdx := strings.Index(got, "https://github.com/golang/go")
	latestIdx := strings.Index(got, "LATEST")
	if descIdx >= urlIdx || urlIdx >= latestIdx {
		t.Errorf("expected order: description < url < suggestions; got positions desc=%d url=%d latest=%d\n--- got ---\n%s",
			descIdx, urlIdx, latestIdx, got)
	}
}

// TestSelectOrInputField_URLLineEmptyOmitsRow pins down that callers that
// do not set a URL (or pass "") get no extra blank row in the accessible
// header.
func TestSelectOrInputField_URLLineEmptyOmitsRow(t *testing.T) {
	t.Parallel()
	var target string
	f := newSelectOrInputField("k", &target, []string{"LATEST"}, "Other").
		Title("title").
		Description("desc")

	var buf bytes.Buffer
	f.printAccessibleHeader(&buf)
	got := buf.String()
	if strings.Contains(got, "https://") {
		t.Errorf("URL row leaked into header when URLLine was unset:\n%s", got)
	}
	// Exactly three lines before the suggestion list: title, description,
	// suggestions (no URL row). Tolerate trailing whitespace.
	prefix := strings.SplitN(got, "1. LATEST", 2)[0]
	lines := strings.Split(strings.TrimRight(prefix, "\n"), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 header lines (title, description); got %d:\n%q", len(lines), lines)
	}
}
