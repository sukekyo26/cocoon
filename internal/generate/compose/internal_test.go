package compose

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestScalarString(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   any
		want string
	}{
		{"string_passthrough", "hello", "hello"},
		{"int64", int64(42), "42"},
		{"int", 7, "7"},
		{"float_via_fmt", 1.5, "1.5"},
		{"nil_via_fmt", nil, "<nil>"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := scalarString(c.in); got != c.want {
				t.Errorf("scalarString(%v) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestAnyNode_BoolFloatSlice(t *testing.T) {
	t.Parallel()
	t.Run("bool_true", func(t *testing.T) {
		t.Parallel()
		n := anyNode(true)
		if n == nil || n.Kind != yaml.ScalarNode || n.Value != "true" {
			t.Errorf("bool true => %+v", n)
		}
	})
	t.Run("float64", func(t *testing.T) {
		t.Parallel()
		n := anyNode(1.5)
		if n == nil || n.Kind != yaml.ScalarNode {
			t.Errorf("float => %+v", n)
		}
	})
	t.Run("seq_of_strings", func(t *testing.T) {
		t.Parallel()
		n := anyNode([]any{"a", "b"})
		if n == nil || n.Kind != yaml.SequenceNode || len(n.Content) != 2 {
			t.Errorf("seq => %+v", n)
		}
	})
	t.Run("map", func(t *testing.T) {
		t.Parallel()
		n := anyNode(map[string]any{"k": "v"})
		if n == nil || n.Kind != yaml.MappingNode || len(n.Content) != 2 {
			t.Errorf("map => %+v", n)
		}
	})
	t.Run("unknown_quoted", func(t *testing.T) {
		t.Parallel()
		type unknown struct{ X int }
		n := anyNode(unknown{X: 7})
		if n == nil || n.Kind != yaml.ScalarNode {
			t.Errorf("unknown => %+v", n)
		}
	})
}

func TestSysctlNode(t *testing.T) {
	t.Parallel()
	t.Run("int", func(t *testing.T) {
		t.Parallel()
		n := sysctlNode(7)
		if n.Value != "7" {
			t.Errorf("int = %q", n.Value)
		}
	})
	t.Run("int64", func(t *testing.T) {
		t.Parallel()
		n := sysctlNode(int64(42))
		if n.Value != "42" {
			t.Errorf("int64 = %q", n.Value)
		}
	})
	t.Run("string", func(t *testing.T) {
		t.Parallel()
		n := sysctlNode("hello")
		if n.Value != "hello" {
			t.Errorf("string = %q", n.Value)
		}
	})
	t.Run("unknown_via_fmt", func(t *testing.T) {
		t.Parallel()
		n := sysctlNode(true)
		if n.Value != "true" {
			t.Errorf("bool = %q", n.Value)
		}
	})
}

func TestAnyMap_SortsKeys(t *testing.T) {
	t.Parallel()
	n := anyMap(map[string]any{"b": "B", "a": "A", "c": "C"})
	if n.Kind != yaml.MappingNode {
		t.Fatalf("kind = %v", n.Kind)
	}
	if len(n.Content) != 6 {
		t.Fatalf("expected 6 entries (3 pairs), got %d", len(n.Content))
	}
	keys := []string{n.Content[0].Value, n.Content[2].Value, n.Content[4].Value}
	if keys[0] != "a" || keys[1] != "b" || keys[2] != "c" {
		t.Errorf("keys = %v, want sorted", keys)
	}
}
