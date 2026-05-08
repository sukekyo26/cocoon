package dockersock

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestIsSocketDetectsUnixSocket(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "test.sock")
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "unix", sockPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	if !isSocket(sockPath) {
		t.Errorf("isSocket(%s) = false, want true", sockPath)
	}
}

func TestIsSocketRejectsRegularFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "file")
	if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if isSocket(p) {
		t.Errorf("isSocket on regular file = true, want false")
	}
}

func TestIsSocketRejectsMissingPath(t *testing.T) {
	t.Parallel()
	if isSocket(filepath.Join(t.TempDir(), "no-such")) {
		t.Errorf("isSocket on missing path = true")
	}
}

func TestIsSocketRejectsDirectory(t *testing.T) {
	t.Parallel()
	if isSocket(t.TempDir()) {
		t.Errorf("isSocket on directory = true, want false")
	}
}
