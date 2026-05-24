package plugin_test

import (
	"testing"

	"github.com/sukekyo26/cocoon/internal/plugin"
)

func TestFormatPinLine_PinOnly(t *testing.T) {
	t.Parallel()

	got := plugin.FormatPinLine("go", "1.23.4", "", "")
	want := "go = { pin = \"1.23.4\" }\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatPinLine_WithBothChecksums(t *testing.T) {
	t.Parallel()

	got := plugin.FormatPinLine("starship", "1.21.1", "abc123", "def456")
	want := "starship = { pin = \"1.21.1\", checksum_amd64 = \"abc123\", checksum_arm64 = \"def456\" }\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatPinLine_OnlyAmd64Checksum(t *testing.T) {
	t.Parallel()

	got := plugin.FormatPinLine("go", "1.23.4", "abc", "")
	want := "go = { pin = \"1.23.4\", checksum_amd64 = \"abc\" }\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatPinSection_Empty(t *testing.T) {
	t.Parallel()

	got := plugin.FormatPinSection(nil)
	if got != "" {
		t.Errorf("empty pins → empty string, got %q", got)
	}
}

func TestFormatPinSection_SortedByID(t *testing.T) {
	t.Parallel()

	got := plugin.FormatPinSection([]plugin.PinLine{
		{ID: "uv", Ref: "0.5.7"},
		{ID: "go", Ref: "1.23.4", ChecksumAmd64: "abc"},
		{ID: "bun", Ref: "1.3.3"},
	})
	want := "[plugins.versions]\n" +
		"bun = { pin = \"1.3.3\" }\n" +
		"go = { pin = \"1.23.4\", checksum_amd64 = \"abc\" }\n" +
		"uv = { pin = \"0.5.7\" }\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// TestFormatPinLineWithExtras_NilEqualsFormatPinLine pins that
// FormatPinLineWithExtras with a nil extras map reproduces
// FormatPinLine's output byte-for-byte. This is the contract that lets
// the existing FormatPinLine call site keep working without a flag day.
func TestFormatPinLineWithExtras_NilEqualsFormatPinLine(t *testing.T) {
	t.Parallel()
	plain := plugin.FormatPinLine("go", "1.23.4", "abc", "def")
	withExtras := plugin.FormatPinLineWithExtras("go", "1.23.4", "abc", "def", nil)
	if plain != withExtras {
		t.Errorf("nil extras must match FormatPinLine:\nplain:   %q\nextras:  %q", plain, withExtras)
	}
}

// TestFormatPinLineWithExtras_SortedKeyOrder pins that extra keys are
// emitted in alphabetical order so the output is stable across calls
// (Go map iteration is randomised, which would otherwise drift the
// workspace.toml diff produced by pin --write).
func TestFormatPinLineWithExtras_SortedKeyOrder(t *testing.T) {
	t.Parallel()
	got := plugin.FormatPinLineWithExtras("android-sdk", "14742923", "", "",
		map[string]string{
			"build_tools": "35.0.0",
			"api_level":   "35",
		})
	want := "android-sdk = { pin = \"14742923\", api_level = \"35\", build_tools = \"35.0.0\" }\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// TestFormatPinLineWithExtras_QuotedValues pins that values flow
// through %q so quotes and backslashes round-trip correctly.
func TestFormatPinLineWithExtras_QuotedValues(t *testing.T) {
	t.Parallel()
	got := plugin.FormatPinLineWithExtras("x", "1.0", "", "",
		map[string]string{"label": `quoted "value"`})
	want := "x = { pin = \"1.0\", label = \"quoted \\\"value\\\"\" }\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}
