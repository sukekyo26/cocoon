package tmplx_test

import (
	"strings"
	"testing"
	"text/template"

	"github.com/sukekyo26/cocoon/internal/generate/tmplx"
)

func TestRender(t *testing.T) {
	t.Parallel()
	tmpl := tmplx.MustParse("greet", `hello {{ .Name }}`, nil)
	got, err := tmplx.Render(tmpl, map[string]string{"Name": "world"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got != "hello world" {
		t.Errorf("got %q, want %q", got, "hello world")
	}
}

func TestBaseFuncsShellQuote(t *testing.T) {
	t.Parallel()
	tmpl := tmplx.MustParse("q", `{{ shellQuote .V }}`, nil)
	got, err := tmplx.Render(tmpl, map[string]string{"V": "with space"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got != `'with space'` {
		t.Errorf("got %q, want %q", got, `'with space'`)
	}
}

func TestExtendOverride(t *testing.T) {
	t.Parallel()
	extra := template.FuncMap{
		"shellQuote": func(s string) string { return "X" + s + "X" },
	}
	tmpl := tmplx.MustParse("ov", `{{ shellQuote "y" }}`, extra)
	got, err := tmplx.Render(tmpl, nil)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got != "XyX" {
		t.Errorf("override failed: got %q", got)
	}
}

func TestWhitespaceTrim(t *testing.T) {
	t.Parallel()
	const body = `start
{{- with .X }}
{{ . }}
{{- end }}
end`
	tmpl := tmplx.MustParse("ws", body, nil)
	empty, err := tmplx.Render(tmpl, map[string]string{"X": ""})
	if err != nil {
		t.Fatalf("Render empty: %v", err)
	}
	if !strings.Contains(empty, "start\nend") {
		t.Errorf("empty placeholder did not collapse: %q", empty)
	}
	full, err := tmplx.Render(tmpl, map[string]string{"X": "MID"})
	if err != nil {
		t.Fatalf("Render full: %v", err)
	}
	if !strings.Contains(full, "start\nMID\nend") {
		t.Errorf("non-empty placeholder rendered wrong: %q", full)
	}
}
