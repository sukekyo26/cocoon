package selfupdatecli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
	"github.com/sukekyo26/cocoon/internal/logx"
	"github.com/sukekyo26/cocoon/internal/release"
	"github.com/sukekyo26/cocoon/internal/version"
)

const (
	apiTimeout = 30 * time.Second
	// ExitNewerAvailable is the exit code `--check-only` uses to signal a
	// newer release is available without downloading it.
	ExitNewerAvailable = 100
)

const selfUpdateLong = `cocoon self-update — replace the current binary with the latest release

Hits the GitHub Releases API for sukekyo26/cocoon, compares the
release tag against the build's compiled-in version string, and on
update downloads the matching cocoon-<os>-<arch> asset under SHA-256
verification before atomically replacing this executable.

Exit codes:
  0   already up to date, or replacement succeeded
  100 (only with --check-only) a newer version exists
  1   any other failure`

// NewCommand returns the cobra command for `cocoon self-update`.
func NewCommand(stdout, stderr io.Writer) *cobra.Command {
	var checkOnly, force bool
	cmd := &cobra.Command{
		Use:           "self-update",
		Short:         "Replace this binary with the latest released version",
		Long:          selfUpdateLong,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runSelfUpdate(cmd.Context(), stdout, stderr, checkOnly, force)
		},
	}
	cmd.Flags().BoolVar(&checkOnly, "check-only", false,
		"exit 0 if up to date, exit 100 if a newer release exists; never download")
	cmd.Flags().BoolVar(&force, "force", false,
		"reinstall even when the local binary is already the latest version")
	return cmd
}

//nolint:gocyclo // self-update has many sequential phases; splitting hurts readability.
func runSelfUpdate(ctx context.Context, stdout, stderr io.Writer, checkOnly, force bool) error {
	log := logx.New(stdout, stderr)

	current := strings.TrimSpace(version.Get())
	if current == "" || current == "dev" {
		log.Error("self-update: cannot self-update a dev build (no version baked in)")
		return clihelpers.ErrFailure
	}
	current = strings.TrimPrefix(current, "v")

	tctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()
	rel, err := release.FetchLatest(tctx)
	if err != nil {
		return fmt.Errorf("%w: %w", clihelpers.ErrFailure, err)
	}
	latest := strings.TrimPrefix(rel.TagName, "v")

	log.Infof("%s %s", log.Bold("current version :"), current)
	log.Infof("%s %s", log.Bold("latest release  :"), latest)

	if !force && latest == current {
		log.Success("already up to date")
		return nil
	}
	if checkOnly {
		// stdout: keeps the line on the same stream as the version labels
		// so grep-on-stdout scripts keep working. Exit code 100 is still
		// the canonical "newer available" signal.
		log.Infof("newer release %s available; rerun without --check-only to install", latest)
		// Cancel before os.Exit (gocritic exitAfterDefer).
		cancel()
		os.Exit(ExitNewerAvailable) //nolint:gocritic // cancel() above runs the only pending defer.
	}

	assetName := fmt.Sprintf("cocoon-%s-%s", runtime.GOOS, runtime.GOARCH)
	assetURL := rel.AssetURL(assetName)
	sumsURL := rel.AssetURL("SHA256SUMS")
	if assetURL == "" {
		return fmt.Errorf("%w: release asset %q not found in %s", clihelpers.ErrFailure, assetName, rel.TagName)
	}
	if sumsURL == "" {
		return fmt.Errorf("%w: SHA256SUMS not found in %s", clihelpers.ErrFailure, rel.TagName)
	}

	tmp, err := os.MkdirTemp("", "cocoon-update-")
	if err != nil {
		return fmt.Errorf("%w: mktemp: %w", clihelpers.ErrFailure, err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	binPath := filepath.Join(tmp, assetName)
	sumsPath := filepath.Join(tmp, "SHA256SUMS")
	// Progressf writes to stderr (transient, not data) so stdout-grep
	// scripts see only the stable version / success output.
	log.Progressf("downloading %s...", assetName)
	if err = downloadFile(ctx, assetURL, binPath); err != nil {
		return fmt.Errorf("%w: download %s: %w", clihelpers.ErrFailure, assetName, err)
	}
	if err = downloadFile(ctx, sumsURL, sumsPath); err != nil {
		return fmt.Errorf("%w: download SHA256SUMS: %w", clihelpers.ErrFailure, err)
	}

	expected, err := readChecksum(sumsPath, assetName)
	if err != nil {
		return fmt.Errorf("%w: %w", clihelpers.ErrFailure, err)
	}
	actual, err := sha256File(binPath)
	if err != nil {
		return fmt.Errorf("%w: %w", clihelpers.ErrFailure, err)
	}
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("%w: checksum mismatch (got %s, want %s)", clihelpers.ErrFailure, actual, expected)
	}

	if err = os.Chmod(binPath, 0o755); err != nil {
		return fmt.Errorf("%w: chmod: %w", clihelpers.ErrFailure, err)
	}

	selfPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("%w: locate self: %w", clihelpers.ErrFailure, err)
	}
	if err = atomicReplace(binPath, selfPath); err != nil {
		return fmt.Errorf("%w: replace %s: %w", clihelpers.ErrFailure, selfPath, err)
	}

	log.Successf("updated cocoon to %s at %s", latest, selfPath)
	return nil
}

func downloadFile(ctx context.Context, url, dst string) error {
	tctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(tctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("get %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("%w: download %s: %s", release.ErrHTTPStatus, url, resp.Status)
	}
	f, err := os.Create(dst) //nolint:gosec // dst is mktemp-d temporary path.
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	defer func() { _ = f.Close() }()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("write %s: %w", dst, err)
	}
	return nil
}

func readChecksum(sumsPath, asset string) (string, error) {
	data, err := os.ReadFile(sumsPath) //nolint:gosec // sumsPath is mktemp-d temporary path.
	if err != nil {
		return "", fmt.Errorf("read SHA256SUMS: %w", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		if strings.TrimPrefix(fields[1], "*") == asset {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("%w: %s", errAssetMissing, asset)
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path) //nolint:gosec // path is mktemp-d temporary asset.
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash %s: %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// atomicReplace falls back to copy+chmod when src and dst are on
// different filesystems (os.Rename's EXDEV).
func atomicReplace(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	in, err := os.Open(src) //nolint:gosec // src is mktemp-d temporary path.
	if err != nil {
		return fmt.Errorf("open src: %w", err)
	}
	defer func() { _ = in.Close() }()
	tmp := dst + ".cocoon-update.tmp"
	out, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755) //nolint:gosec
	if err != nil {
		return fmt.Errorf("create %s: %w", tmp, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("copy: %w", err)
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("close %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename %s -> %s: %w", tmp, dst, err)
	}
	return nil
}
