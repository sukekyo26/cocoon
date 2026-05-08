package schema_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/sukekyo26/cocoon/internal/cli/schema"
)

//nolint:unparam // test helper returns both buffers; not all callers use both
func runCmd(args ...string) (stdout, stderr bytes.Buffer, err error) {
	cmd := schema.NewCommand(&stdout, &stderr)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return stdout, stderr, err
}

func TestGenerateValidJSON(t *testing.T) {
	t.Parallel()
	data, err := schema.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	var v map[string]any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !bytes.HasSuffix(data, []byte("\n")) {
		t.Errorf("output should end with newline")
	}
}

func TestGenerateForwardItemsIsOneOf(t *testing.T) {
	t.Parallel()
	data, err := schema.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	var root struct {
		Defs map[string]json.RawMessage `json:"$defs"`
	}
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if _, ok := root.Defs["PortMappingLong"]; !ok {
		t.Fatalf("missing $defs.PortMappingLong; defs=%v", keys(root.Defs))
	}

	var portsSpec struct {
		Properties struct {
			Forward struct {
				Type  string `json:"type"`
				Items struct {
					OneOf []map[string]any `json:"oneOf"`
				} `json:"items"`
			} `json:"forward"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(root.Defs["PortsSpec"], &portsSpec); err != nil {
		t.Fatalf("PortsSpec unmarshal: %v", err)
	}

	if portsSpec.Properties.Forward.Type != "array" {
		t.Errorf("forward.type = %q, want %q", portsSpec.Properties.Forward.Type, "array")
	}
	one := portsSpec.Properties.Forward.Items.OneOf
	if len(one) != 2 {
		t.Fatalf("forward.items.oneOf len = %d, want 2", len(one))
	}
	if one[0]["type"] != "string" {
		t.Errorf("oneOf[0].type = %v, want \"string\"", one[0]["type"])
	}
	pat, _ := one[0]["pattern"].(string) //nolint:errcheck // type assert ok-pattern.
	if pat == "" {
		t.Errorf("oneOf[0].pattern is empty")
	}
	ref, _ := one[1]["$ref"].(string) //nolint:errcheck // type assert ok-pattern.
	if ref != "#/$defs/PortMappingLong" {
		t.Errorf("oneOf[1].$ref = %q, want %q", ref, "#/$defs/PortMappingLong")
	}

	var long struct {
		Type                 string         `json:"type"`
		Required             []string       `json:"required"`
		AdditionalProperties any            `json:"additionalProperties"`
		Properties           map[string]any `json:"properties"`
	}
	if err := json.Unmarshal(root.Defs["PortMappingLong"], &long); err != nil {
		t.Fatalf("PortMappingLong unmarshal: %v", err)
	}
	if long.Type != "object" {
		t.Errorf("PortMappingLong.type = %q, want \"object\"", long.Type)
	}
	if len(long.Required) != 1 || long.Required[0] != "target" {
		t.Errorf("PortMappingLong.required = %v, want [target]", long.Required)
	}
	if got, want := long.AdditionalProperties, false; got != want {
		t.Errorf("PortMappingLong.additionalProperties = %v, want %v", got, want)
	}
	for _, k := range []string{"target", "published", "host_ip", "protocol", "mode"} {
		if _, ok := long.Properties[k]; !ok {
			t.Errorf("PortMappingLong missing property %q", k)
		}
	}
}

func keys(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestRunDumpStdout(t *testing.T) {
	t.Parallel()
	stdout, _, err := runCmd("dump")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if stdout.Len() == 0 {
		t.Errorf("stdout empty")
	}
}

func TestRunHelp(t *testing.T) {
	t.Parallel()
	stdout, _, err := runCmd("help")
	if err != nil {
		t.Fatalf("Run help: %v", err)
	}
	if !strings.Contains(stdout.String(), "dump") {
		t.Errorf("missing usage: %q", stdout.String())
	}
}

func TestRunMissingSubcommand(t *testing.T) {
	t.Parallel()
	_, stderr, err := runCmd()
	if !errors.Is(err, schema.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
	if !strings.Contains(stderr.String(), "dump") {
		t.Errorf("missing usage in stderr: %q", stderr.String())
	}
}

func TestRunUnknownSubcommand(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("bogus")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRunDumpInvalidFlag(t *testing.T) {
	t.Parallel()
	_, _, err := runCmd("dump", "--bogus")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRunDumpToOutputFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out := dir + "/sub/schema.json"
	if _, _, err := runCmd("dump", "--output", out); err != nil {
		t.Fatalf("Run: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("output file missing: %v", err)
	}
	var v map[string]any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Errorf("output not valid JSON: %v", err)
	}
}

func TestRunDumpToWriteDefaultPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(cwd); err != nil {
			t.Logf("restore cwd: %v", err)
		}
	})

	if _, _, err := runCmd("dump", "--write"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, err := os.Stat("schemas/workspace.schema.json"); err != nil {
		t.Errorf("default-path file missing: %v", err)
	}
}
