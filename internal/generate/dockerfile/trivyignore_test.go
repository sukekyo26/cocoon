package dockerfile_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// trivyIgnoreFile is the repo-root suppression list the trivy-* recipes in
// justfile pass via --ignorefile, and which the E2E workflows rely on to keep
// the generated Dockerfile's two intentional misconfigs from failing CI.
const trivyIgnoreFile = ".trivyignore.yaml"

type trivyIgnore struct {
	Misconfigurations []struct {
		ID        string `yaml:"id"`
		Statement string `yaml:"statement"`
	} `yaml:"misconfigurations"`
	Vulnerabilities []struct {
		ID string `yaml:"id"`
	} `yaml:"vulnerabilities"`
}

func loadTrivyIgnore(t *testing.T) trivyIgnore {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(repoRoot(t), trivyIgnoreFile))
	if err != nil {
		t.Fatalf("read %s: %v", trivyIgnoreFile, err)
	}
	var parsed trivyIgnore
	if uerr := yaml.Unmarshal(raw, &parsed); uerr != nil {
		t.Fatalf("parse %s: %v", trivyIgnoreFile, uerr)
	}
	return parsed
}

// TestTrivyIgnore_SuppressionsAreReviewed pins the suppression set so a third
// entry cannot be added without editing this test — silencing a Trivy finding
// is a security decision that has to be argued in review, not slipped in.
func TestTrivyIgnore_SuppressionsAreReviewed(t *testing.T) {
	t.Parallel()

	// Every ID here is justified in the file's own statement field; see
	// .trivyignore.yaml for the reasoning.
	want := map[string]bool{
		"AVD-DS-0002": true, // last USER is root (entrypoint drops privileges)
		"AVD-DS-0026": true, // no HEALTHCHECK (interactive dev container)
	}

	parsed := loadTrivyIgnore(t)
	got := make(map[string]bool, len(parsed.Misconfigurations))
	for _, m := range parsed.Misconfigurations {
		got[m.ID] = true
		if strings.TrimSpace(m.Statement) == "" {
			t.Errorf("suppression %s has no statement explaining why it is intentional", m.ID)
		}
	}

	for id := range want {
		if !got[id] {
			t.Errorf("expected suppression %s is missing; if the generator no longer trips it, drop it from want too", id)
		}
	}
	for id := range got {
		if !want[id] {
			t.Errorf("unreviewed suppression %s: add it to want here once the review agreed it is intentional", id)
		}
	}

	// The image CVE scan is report-only, so a vulnerability entry would only
	// hide rows from a report that gates nothing.
	if len(parsed.Vulnerabilities) != 0 {
		t.Errorf("got %d vulnerability suppressions, want 0: the image scan does not gate, so there is nothing to suppress", len(parsed.Vulnerabilities))
	}
}

// TestTrivyIgnore_DS0002PremiseStillHolds guards the reasoning behind the
// AVD-DS-0002 suppression rather than the suppression itself. It is justified
// only because the Dockerfile ends as root so docker-entrypoint.sh can remap
// the container user's UID/GID onto the host owner before dropping privileges.
// If the generator ever stops emitting that shape, the statement in
// .trivyignore.yaml becomes false and the suppression must go — this test
// fails first and says so.
func TestTrivyIgnore_DS0002PremiseStillHolds(t *testing.T) {
	t.Parallel()

	// The golden snapshot is kept byte-identical to generator output by
	// TestGenerate_Snapshot, so reading it here is equivalent to generating.
	raw, err := os.ReadFile(filepath.Join("testdata", "snapshot.expected"))
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	lines := strings.Split(string(raw), "\n")

	userRe := regexp.MustCompile(`^USER\s+(\S+)`)
	lastUser, lastUserLine := "", -1
	for i, line := range lines {
		if m := userRe.FindStringSubmatch(line); m != nil {
			lastUser, lastUserLine = m[1], i
		}
	}
	if lastUser != "root" {
		t.Fatalf("last USER is %q, want \"root\": the AVD-DS-0002 suppression in %s is now unjustified — remove it", lastUser, trivyIgnoreFile)
	}

	// The trailing root stage exists to install the entrypoint that drops
	// privileges. Without that entrypoint the container really would run as
	// root and the suppression would be hiding a genuine finding.
	tail := strings.Join(lines[lastUserLine:], "\n")
	if !strings.Contains(tail, "ENTRYPOINT") || !strings.Contains(tail, "docker-entrypoint.sh") {
		t.Errorf("no docker-entrypoint.sh ENTRYPOINT after the final USER root; the AVD-DS-0002 suppression in %s assumes the entrypoint drops privileges", trivyIgnoreFile)
	}
}
