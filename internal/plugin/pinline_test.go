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
