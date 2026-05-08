package setup

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/sukekyo26/cocoon/internal/dockersock"
)

type defaultGIDDetector struct{}

// Detect resolves the docker socket GID via stat or /etc/group.
func (defaultGIDDetector) Detect() (int, error) { return detectDockerGID() }

const etcGroupPath = "/etc/group"

func detectDockerGID() (int, error) {
	for _, p := range dockersock.CandidatePaths() {
		if gid, err := socketFileGID(p); err == nil {
			return gid, nil
		}
	}
	return dockerGroupGIDFromFile(etcGroupPath)
}

func socketFileGID(path string) (int, error) {
	info, err := os.Stat(path) //nolint:gosec // path is from dockersock.CandidatePaths().
	if err != nil {
		return 0, fmt.Errorf("%w: stat %s: %w", ErrDockerGID, path, err)
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return int(stat.Gid), nil
	}
	return 0, fmt.Errorf("%w: stat_t unavailable", ErrDockerGID)
}

func dockerGroupGIDFromFile(path string) (int, error) {
	f, err := os.Open(path) //nolint:gosec // caller passes etcGroupPath or a test path.
	if err != nil {
		return 0, fmt.Errorf("%w: open %s: %w", ErrDockerGID, path, err)
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		parts := strings.SplitN(sc.Text(), ":", 4)
		if len(parts) >= 3 && parts[0] == "docker" {
			gid, err := strconv.Atoi(parts[2])
			if err == nil {
				return gid, nil
			}
		}
	}
	if err := sc.Err(); err != nil {
		return 0, fmt.Errorf("%w: scan %s: %w", ErrDockerGID, path, err)
	}
	return 0, fmt.Errorf("%w: docker group not found in %s", ErrDockerGID, path)
}
