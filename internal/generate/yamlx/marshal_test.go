package yamlx_test

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/sukekyo26/cocoon/internal/generate/yamlx"
)

func TestMarshalBlockStyle(t *testing.T) {
	t.Parallel()
	v := map[string]any{
		"services": map[string]any{
			"dev": map[string]any{"image": "ubuntu:24.04"},
		},
	}
	out, err := yamlx.Marshal(v)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got := string(out)
	if !strings.HasSuffix(got, "\n") {
		t.Fatalf("expected trailing newline, got %q", got)
	}
	if strings.Contains(got, "{") {
		t.Fatalf("expected block style, got flow: %q", got)
	}
}

func TestMarshalIndent(t *testing.T) {
	t.Parallel()
	type svc struct {
		Image string   `yaml:"image"`
		Args  []string `yaml:"args"`
	}
	out, err := yamlx.Marshal(map[string]svc{"dev": {Image: "u", Args: []string{"a", "b"}}})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	want := "dev:\n  image: u\n  args:\n    - a\n    - b\n"
	if string(out) != want {
		t.Fatalf("indent mismatch:\nwant %q\ngot  %q", want, string(out))
	}
}

func TestQuotedIfSpecial(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in    string
		style yaml.Style
	}{
		{"plain", 0},
		{"${VAR}", yaml.DoubleQuotedStyle},
		{"a:b", yaml.DoubleQuotedStyle},
		{"3000:3000", yaml.DoubleQuotedStyle},
		{"hello", 0},
		{"a,b", yaml.DoubleQuotedStyle},
	}
	for _, tc := range cases {
		n := yamlx.QuotedIfSpecial(tc.in)
		if n.Style != tc.style {
			t.Errorf("%q: style = %v, want %v", tc.in, n.Style, tc.style)
		}
	}
}

func TestQuotedNode(t *testing.T) {
	t.Parallel()
	n := yamlx.Quoted("x")
	out, err := yamlx.Marshal(n)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(out) != "\"x\"\n" {
		t.Fatalf("Quoted serialisation: got %q", string(out))
	}
}

func TestPlainNode(t *testing.T) {
	t.Parallel()
	n := yamlx.Plain("hello")
	if n.Kind != yaml.ScalarNode || n.Value != "hello" || n.Style != 0 {
		t.Errorf("Plain = %+v", n)
	}
}

func TestIntNode(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   int
		want string
	}{{0, "0"}, {42, "42"}, {-1, "-1"}}
	for _, tc := range cases {
		n := yamlx.Int(tc.in)
		if n.Tag != "!!int" || n.Value != tc.want {
			t.Errorf("Int(%d) = %+v, want value=%q", tc.in, n, tc.want)
		}
	}
}

func TestBoolNode(t *testing.T) {
	t.Parallel()
	if n := yamlx.Bool(true); n.Value != "true" {
		t.Errorf("Bool(true) value = %q", n.Value)
	}
	if n := yamlx.Bool(false); n.Value != "false" {
		t.Errorf("Bool(false) value = %q", n.Value)
	}
}

func TestMapNode(t *testing.T) {
	t.Parallel()
	m := yamlx.Map(
		yamlx.Pair{Key: "image", Value: yamlx.Plain("ubuntu")},
		yamlx.Pair{Key: "ports", Value: yamlx.Seq(yamlx.Int(80), yamlx.Int(443))},
	)
	if m.Kind != yaml.MappingNode {
		t.Errorf("Map kind = %v, want MappingNode", m.Kind)
	}
	if len(m.Content) != 4 {
		t.Errorf("len(content) = %d, want 4 (2 pairs × 2)", len(m.Content))
	}
	out, err := yamlx.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got := string(out)
	if !strings.Contains(got, "image: ubuntu") || !strings.Contains(got, "ports:") {
		t.Errorf("unexpected serialisation: %q", got)
	}
}

func TestSeqNode(t *testing.T) {
	t.Parallel()
	s := yamlx.Seq(yamlx.Plain("a"), yamlx.Plain("b"))
	if s.Kind != yaml.SequenceNode {
		t.Errorf("Seq kind = %v", s.Kind)
	}
	if len(s.Content) != 2 {
		t.Errorf("len(content) = %d, want 2", len(s.Content))
	}
}
