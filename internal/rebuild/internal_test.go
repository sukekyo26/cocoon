package rebuild

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFormatTimestamp(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"valid_rfc3339", "2024-01-15T10:30:45Z", "2024-01-15 10:30:45"},
		{"valid_with_trailing_ws", "  2024-01-15T10:30:45Z  ", "2024-01-15 10:30:45"},
		{"invalid_returns_raw", "not-a-timestamp", "not-a-timestamp"},
		{"empty_returns_empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := formatTimestamp(tc.in); got != tc.want {
				t.Errorf("formatTimestamp(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestDaysAgo(t *testing.T) {
	t.Parallel()
	now := time.Now()
	cases := []struct {
		name string
		in   string
		want int
	}{
		{"three_days_ago", now.Add(-72 * time.Hour).Format(time.RFC3339Nano), 3},
		{"future_clamped_zero", now.Add(48 * time.Hour).Format(time.RFC3339Nano), 0},
		{"invalid_returns_zero", "not-a-timestamp", 0},
		{"empty_returns_zero", "", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := daysAgo(tc.in); got != tc.want {
				t.Errorf("daysAgo(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestImageName(t *testing.T) {
	t.Parallel()
	t.Run("reads_service_from_env", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		envPath := filepath.Join(dir, ".env")
		if err := os.WriteFile(envPath, []byte("CONTAINER_SERVICE_NAME=app\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		got := imageName(dir)
		want := filepath.Base(dir) + "-app"
		if got != want {
			t.Errorf("imageName = %q, want %q", got, want)
		}
	})
	t.Run("defaults_to_dev_when_env_missing", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		got := imageName(dir)
		want := filepath.Base(dir) + "-dev"
		if got != want {
			t.Errorf("imageName = %q, want %q", got, want)
		}
	})
}
