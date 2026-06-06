// Package i18n holds the message catalog used for localized CLI output.
//
// It mirrors locale/{en,ja}.sh so that Go subcommands print the same
// strings as the legacy bash entry points. Keys are added per-PR as
// individual subcommands are ported; a key missing from the active
// language falls back to English, then to the key itself.
package i18n

import (
	"fmt"
	"os"
	"strings"
)

// Lang identifies a supported message catalog.
type Lang string

const (
	// LangEN is the English catalog (default/fallback).
	LangEN Lang = "en"
	// LangJA is the Japanese catalog.
	LangJA Lang = "ja"
)

// Detect picks a Lang from environment variables, mirroring lib/i18n.sh.
//
// Priority: WORKSPACE_LANG > LC_ALL > LC_MESSAGES > LANG. Any value that
// starts with "ja" selects Japanese; anything else falls back to English.
func Detect() Lang {
	for _, name := range []string{"WORKSPACE_LANG", "LC_ALL", "LC_MESSAGES", "LANG"} {
		v := os.Getenv(name)
		if v == "" {
			continue
		}
		lower := strings.ToLower(v)
		switch {
		case strings.HasPrefix(lower, "ja"):
			return LangJA
		case strings.HasPrefix(lower, "en"):
			return LangEN
		}
	}
	return LangEN
}

// Localizer is implemented by error values that can render themselves in a
// chosen language. The binary boundary (cmd/cocoon/main.go) renders any error
// in the chain that implements it; everything else falls back to Error().
type Localizer interface {
	Localize(*Catalog) string
}

// Catalog renders messages for a single language.
type Catalog struct {
	lang Lang
}

// Lang reports the catalog's active language.
func (c *Catalog) Lang() Lang { return c.lang }

// enCatalog is the cached English catalog used as the always-readable fallback
// (logs, %v, Error()). New(LangEN) only stores the language tag; the message
// tables are read lazily at Msg time, so building it at package-init is safe.
//
//nolint:gochecknoglobals // immutable cached fallback catalog.
var enCatalog = New(LangEN)

// English returns the cached English catalog.
func English() *Catalog { return enCatalog }

// New returns a Catalog for the given language. Unknown languages fall
// back to English.
func New(lang Lang) *Catalog {
	if _, ok := messages[lang]; !ok {
		lang = LangEN
	}
	return &Catalog{lang: lang}
}

// Msg returns the formatted message for key. Missing keys fall back to
// English; a key absent from both tables is returned verbatim so the
// caller still sees something useful.
func (c *Catalog) Msg(key string, args ...any) string {
	tmpl := lookup(c.lang, key)
	if len(args) == 0 {
		return tmpl
	}
	return fmt.Sprintf(tmpl, args...)
}

func lookup(lang Lang, key string) string {
	if tab, ok := messages[lang]; ok {
		if v, ok := tab[key]; ok {
			return v
		}
	}
	if v, ok := messages[LangEN][key]; ok {
		return v
	}
	return key
}

// messages aggregates all language tables. Each *_messages.go file
// contributes to one inner map via init().
var messages = map[Lang]map[string]string{
	LangEN: {},
	LangJA: {},
}

func register(lang Lang, table map[string]string) {
	dst := messages[lang]
	for k, v := range table {
		dst[k] = v
	}
}
