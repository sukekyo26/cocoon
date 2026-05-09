// Package tmplx wraps text/template with a common FuncMap so all generators
// share the same template helpers (shell quoting, etc.) without re-declaring
// them.
package tmplx

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/sukekyo26/cocoon/internal/generate/shellx"
)

// BaseFuncs returns the FuncMap registered for every template parsed via
// MustParse. Generator-specific helpers can be layered on top with Extend.
func BaseFuncs() template.FuncMap {
	return template.FuncMap{
		"shellQuote": shellx.ShellQuote,
	}
}

// Extend returns a copy of base with extra entries merged in. extra wins on
// key conflicts, allowing a generator to override a base helper if needed.
func Extend(base, extra template.FuncMap) template.FuncMap {
	out := make(template.FuncMap, len(base)+len(extra))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

// MustParse parses body as a template named name, registering BaseFuncs plus
// any extra functions. It panics on parse failure (suitable for package-level
// var initialisation).
func MustParse(name, body string, extra template.FuncMap) *template.Template {
	funcs := BaseFuncs()
	if len(extra) > 0 {
		funcs = Extend(funcs, extra)
	}
	return template.Must(template.New(name).Funcs(funcs).Parse(body))
}

// Render executes t against data and returns the result as a string.
func Render(t *template.Template, data any) (string, error) {
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("tmplx: execute %q: %w", t.Name(), err)
	}
	return buf.String(), nil
}
