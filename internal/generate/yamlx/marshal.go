// Package yamlx wraps gopkg.in/yaml.v3 with helpers that emit:
//   - Block style with a 2-space indent.
//   - Sequences indented relative to their parent key.
//   - Strings as plain (unquoted) scalars by default, double-quoted only when
//     the value contains a YAML-special character listed in [SpecialChars].
//
// Generators construct documents from [Map], [Seq], [Plain], [Quoted],
// [QuotedIfSpecial], [Int], and [Bool] so the resulting *yaml.Node carries
// the desired scalar style.
package yamlx

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// SpecialChars enumerates the runes that, when present in a scalar string,
// force [QuotedIfSpecial] to emit a double-quoted form rather than a plain
// scalar.
const SpecialChars = ":{}#&*!|>%@`'\"[]?,\n"

// Marshal serialises v as YAML using a 2-space indent and block style and
// returns the trailing-newline-terminated byte slice.
func Marshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		_ = enc.Close()
		return nil, fmt.Errorf("yamlx: encode: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("yamlx: close encoder: %w", err)
	}
	return buf.Bytes(), nil
}

// Quoted returns a *yaml.Node representing s as a double-quoted scalar.
// Use this for values that must always be quoted regardless of content.
func Quoted(s string) *yaml.Node {
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: s,
		Style: yaml.DoubleQuotedStyle,
	}
}

// Plain returns a *yaml.Node representing s as a plain (unquoted) scalar.
func Plain(s string) *yaml.Node {
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: s,
	}
}

// QuotedIfSpecial returns a double-quoted node when s contains any character
// in [SpecialChars], otherwise a plain node.
func QuotedIfSpecial(s string) *yaml.Node {
	if strings.ContainsAny(s, SpecialChars) {
		return Quoted(s)
	}
	return Plain(s)
}

// Int returns a plain integer scalar node.
func Int(i int) *yaml.Node {
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!int",
		Value: fmt.Sprintf("%d", i),
	}
}

// Bool returns a plain boolean scalar node ("true" / "false").
func Bool(b bool) *yaml.Node {
	v := "false"
	if b {
		v = "true"
	}
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!bool",
		Value: v,
	}
}

// Pair is a (key, value) entry for Map. Keys are emitted as plain scalars.
type Pair struct {
	Key   string
	Value *yaml.Node
}

// Map builds an ordered mapping node from the given pairs.
func Map(pairs ...Pair) *yaml.Node {
	content := make([]*yaml.Node, 0, len(pairs)*2)
	for _, p := range pairs {
		content = append(content, Plain(p.Key), p.Value)
	}
	return &yaml.Node{Kind: yaml.MappingNode, Content: content}
}

// Seq builds a sequence node from the given items.
func Seq(items ...*yaml.Node) *yaml.Node {
	return &yaml.Node{Kind: yaml.SequenceNode, Content: items}
}
