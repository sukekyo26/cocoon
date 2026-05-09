package devcontainerjson

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// orderedMap is a string-keyed map that preserves insertion order when
// serialised to JSON. Used to keep the devcontainer.json output identical
// to Python json.dumps over an insertion-ordered dict.
type orderedMap struct {
	keys   []string
	values map[string]any
}

func newOrderedMap() *orderedMap {
	return &orderedMap{keys: nil, values: map[string]any{}}
}

func (m *orderedMap) set(k string, v any) {
	if _, ok := m.values[k]; !ok {
		m.keys = append(m.keys, k)
	}
	m.values[k] = v
}

func (m *orderedMap) get(k string) (any, bool) {
	v, ok := m.values[k]
	return v, ok
}

// MarshalJSON emits keys in insertion order. Each value is encoded with
// SetEscapeHTML(false) so that '<', '>', '&', and non-ASCII characters
// pass through verbatim (matches Python json.dumps(ensure_ascii=False)).
func (m *orderedMap) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range m.keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		kb, err := json.Marshal(k)
		if err != nil {
			return nil, fmt.Errorf("orderedMap: key: %w", err)
		}
		buf.Write(kb)
		buf.WriteByte(':')
		vb, err := marshalNoHTMLEscape(m.values[k])
		if err != nil {
			return nil, err
		}
		buf.Write(vb)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

func marshalNoHTMLEscape(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, fmt.Errorf("orderedMap: value: %w", err)
	}
	b := buf.Bytes()
	if len(b) > 0 && b[len(b)-1] == '\n' {
		b = b[:len(b)-1]
	}
	return b, nil
}
