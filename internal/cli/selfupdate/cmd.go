package selfupdatecli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/cocoon/internal/cli/clihelpers"
	"github.com/sukekyo26/cocoon/internal/i18n"
	"github.com/sukekyo26/cocoon/internal/logx"
	"github.com/sukekyo26/cocoon/internal/release"
	"github.com/sukekyo26/cocoon/internal/version"
)

// ErrInstallDirReadOnly is returned when the parent directory of the
// running binary is not writable by the current uid. Callers can
// identify the class via errors.Is and surface a remediation hint
// (reinstall to $HOME/.local/bin, or rerun with sudo).
var ErrInstallDirReadOnly = errors.New("install directory is not writable")

const (
	// apiTimeout caps the GitHub Releases lookup. The payload is a small
	// JSON blob, so a short deadline keeps the read-only / --check-only
	// path fast-failing when GitHub is unreachable.
	apiTimeout = 30 * time.Second
	// downloadTimeout caps each asset transfer (the ~12MB binary and the
	// SHA256SUMS file). It is deliberately far more generous than
	// apiTimeout: a slow-but-working connection (corporate proxy, throttled
	// link) needs more than 30s to pull 12MB, and tripping the deadline
	// mid-download surfaces as `context deadline exceeded`.
	downloadTimeout = 3 * time.Minute
	// ExitNewerAvailable is the exit code `--check-only` uses to signal a
	// newer release is available without downloading it.
	ExitNewerAvailable = 100
)

func NewCommand(stdout, stderr io.Writer) *cobra.Command {
	cat := i18n.New(i18n.Detect())
	var checkOnly, force bool
	cmd := &cobra.Command{
		Use:           "self-update",
		Short:         cat.Msg("cmd_self_update_short"),
		Long:          cat.Msg("cmd_self_update_long"),
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runSelfUpdate(cmd.Context(), stdout, stderr, checkOnly, force)
		},
	}
	cmd.Flags().BoolVar(&checkOnly, "check-only", false, cat.Msg("flag_self_update_check_only_usage"))
	cmd.Flags().BoolVar(&force, "force", false, cat.Msg("flag_self_update_force_usage"))
	return cmd
}

//nolint:gocyclo,gocognit,funlen // self-update has many sequential phases; splitting hurts readability.
func runSelfUpdate(ctx context.Context, stdout, stderr io.Writer, checkOnly, force bool) error {
	log := logx.New(stdout, stderr)
	cat := i18n.New(i18n.Detect())

	current := strings.TrimSpace(version.Get())
	if current == "" || current == "dev" {
		log.Error(cat.Msg("selfupdate_dev_build"))
		return clihelpers.ErrFailure
	}
	current = strings.TrimPrefix(current, "v")

	// --check-only is read-only by design — skip the install-dir preflight
	// so users without write access (binary in a root-owned dir like
	// /usr/local/bin) can still query the latest release without sudo.
	// For the install path, resolve self path and verify the parent dir is
	// writable before any network I/O so a permission problem fails fast
	// (saves a 12MB download + SHA256 check) and the error carries a
	// remediation hint.
	var selfPath string
	if !checkOnly {
		var err error
		selfPath, err = executablePath()
		if err != nil {
			return fmt.Errorf("%w: locate self: %w", clihelpers.ErrFailure, err)
		}
		if err = checkInstallDirWritable(selfPath); err != nil {
			if errors.Is(err, ErrInstallDirReadOnly) {
				return fmt.Errorf("%w: %w\n%s", clihelpers.ErrFailure, err, installDirHint(selfPath))
			}
			return fmt.Errorf("%w: %w", clihelpers.ErrFailure, err)
		}
	}

	tctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()
	rel, err := fetchLatest(tctx)
	if err != nil {
		return fmt.Errorf("%w: %w", clihelpers.ErrFailure, err)
	}
	latest := strings.TrimPrefix(rel.TagName, "v")

	log.Infof("%s %s", log.Bold(cat.Msg("selfupdate_label_current")), current)
	log.Infof("%s %s", log.Bold(cat.Msg("selfupdate_label_latest")), latest)

	if !force && latest == current {
		log.Success(cat.Msg("selfupdate_up_to_date"))
		return nil
	}
	if checkOnly {
		// stdout: keeps the line on the same stream as the version labels
		// so grep-on-stdout scripts keep working. Exit code 100 is still
		// the canonical "newer available" signal.
		log.Info(cat.Msg("selfupdate_newer_available", latest))
		// Cancel before os.Exit (gocritic exitAfterDefer).
		cancel()
		osExit(ExitNewerAvailable) //nolint:gocritic // cancel() above runs the only pending defer.
		return nil
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
	log.Progress(cat.Msg("selfupdate_downloading", assetName))
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

	if err = atomicReplace(binPath, selfPath); err != nil {
		// preflight already caught the common "root-owned dir" case; this
		// branch handles late-arriving EACCES (immutable bit, SELinux MAC,
		// preflight↔rename race) by reusing the same hint so the user
		// still sees actionable remediation rather than a raw syscall error.
		if errors.Is(err, fs.ErrPermission) {
			return fmt.Errorf("%w: replace %s: %w\n%s", clihelpers.ErrFailure, selfPath, err, installDirHint(selfPath))
		}
		return fmt.Errorf("%w: replace %s: %w", clihelpers.ErrFailure, selfPath, err)
	}

	log.Success(cat.Msg("selfupdate_updated", latest, selfPath))
	return nil
}

func downloadFile(ctx context.Context, url, dst string) error {
	tctx, cancel := context.WithTimeout(ctx, downloadTimeout)
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
	return "", fmt.Errorf("%w: %s", ErrAssetMissing, asset)
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

// fetchLatest / executablePath / osExit are test seams (same pattern as
// renameFn below). They let runSelfUpdate be exercised end-to-end without
// a real GitHub Releases roundtrip or terminating the test process.
// Production always points at the std library / package impls.
var (
	fetchLatest    = release.FetchLatest
	executablePath = os.Executable
	osExit         = os.Exit
)

// renameFn is a test seam: it lets the EXDEV copy fallback in
// atomicReplace be exercised deterministically without a second real
// filesystem. Production always uses os.Rename.
var renameFn = os.Rename

// checkInstallDirWritable verifies the parent of selfPath is writable
// without doing the full network/checksum round-trip. It does a real
// create-and-remove test rather than guessing from mode bits, so RO
// mounts / immutable attributes / SELinux MAC / NFS root_squash are all
// caught. Returns ErrInstallDirReadOnly on EACCES so callers can attach
// a remediation hint specific to the "binary lives in a root-owned dir"
// case; other I/O errors (including Close/Remove of the probe file)
// propagate verbatim so a half-written probe never gets silently left
// behind — the helper's contract is "no artefacts on return".
func checkInstallDirWritable(selfPath string) error {
	dir := filepath.Dir(selfPath)
	f, err := os.CreateTemp(dir, ".cocoon-update-preflight-")
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			return fmt.Errorf("%w: %s", ErrInstallDirReadOnly, dir)
		}
		return fmt.Errorf("preflight write check in %s: %w", dir, err)
	}
	name := f.Name()
	if cerr := f.Close(); cerr != nil {
		_ = os.Remove(name)
		return fmt.Errorf("preflight close %s: %w", name, cerr)
	}
	if rerr := os.Remove(name); rerr != nil {
		return fmt.Errorf("preflight cleanup %s: %w", name, rerr)
	}
	return nil
}

// installDirHint returns a remediation message naming the binary's
// location and the elevated invocation the user needs to retry. Kept
// minimal (no alternative install paths) so the message stays focused
// on the one action that always works.
func installDirHint(selfPath string) string {
	return fmt.Sprintf(
		"  cocoon binary lives at %s. self-update needs write access to its parent dir.\n"+
			"  rerun with elevated privileges: sudo %s self-update",
		selfPath, selfPath,
	)
}

// atomicReplace falls back to copy+chmod when src and dst are on
// different filesystems (os.Rename's EXDEV).
func atomicReplace(src, dst string) error {
	if err := renameFn(src, dst); err == nil {
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
	if err := renameFn(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename %s -> %s: %w", tmp, dst, err)
	}
	return nil
}
