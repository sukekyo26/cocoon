package certificates_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sukekyo26/cocoon/internal/certificates"
)

func writeCert(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

const validBody = `-----BEGIN CERTIFICATE-----
MIIBIjANBg
-----END CERTIFICATE-----
`

func TestList_FiltersInvalid(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	certs := filepath.Join(root, "certs")
	if err := os.MkdirAll(certs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeCert(t, certs, "good.crt", validBody)
	writeCert(t, certs, "no-end.crt", "-----BEGIN CERTIFICATE-----\nbody\n")
	writeCert(t, certs, "wrong-header.crt", "PEM\n-----END CERTIFICATE-----\n")
	writeCert(t, certs, "wrong-ext.pem", validBody)
	writeCert(t, certs, "second.crt", validBody)

	got, err := certificates.List(root)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []string{"good.crt", "second.crt"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i, n := range got {
		if n != want[i] {
			t.Errorf("got[%d]=%q want=%q", i, n, want[i])
		}
	}
}

func TestList_NoCertsDir(t *testing.T) {
	t.Parallel()
	got, err := certificates.List(t.TempDir())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != nil {
		t.Fatalf("got %#v", got)
	}
	if certificates.Has(t.TempDir()) {
		t.Fatal("Has should be false on empty")
	}
}

func TestValidate_RejectsMissing(t *testing.T) {
	t.Parallel()
	if err := certificates.Validate(filepath.Join(t.TempDir(), "nope.crt")); err == nil {
		t.Fatal("expected error")
	}
}
