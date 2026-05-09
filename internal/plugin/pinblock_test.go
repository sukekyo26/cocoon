package plugin_test

import (
	"testing"

	"github.com/sukekyo26/cocoon/internal/plugin"
)

func TestFormatPinBlock_PinOnly(t *testing.T) {
	t.Parallel()

	got := plugin.FormatPinBlock("go", "1.23.4", "", "")
	want := "[plugins.versions.go]\npin = \"1.23.4\"\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatPinBlock_WithBothChecksums(t *testing.T) {
	t.Parallel()

	got := plugin.FormatPinBlock("starship", "1.21.1", "abc123", "def456")
	want := "[plugins.versions.starship]\n" +
		"pin = \"1.21.1\"\n" +
		"checksum_amd64 = \"abc123\"\n" +
		"checksum_arm64 = \"def456\"\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatPinBlock_OnlyAmd64Checksum(t *testing.T) {
	t.Parallel()

	got := plugin.FormatPinBlock("go", "1.23.4", "abc", "")
	want := "[plugins.versions.go]\npin = \"1.23.4\"\nchecksum_amd64 = \"abc\"\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}
