package i18n

// User-facing error text, localized at the CLI boundary via clihelpers.LocError
// and config.ValidationError. err_* keys carry the cocoon-authored message;
// err_ctx_* are short technical context labels (en == ja) prepended to a
// wrapped stdlib cause. err_field_* are workspace.toml validation messages
// keyed by the config Accumulator.

func init() {
	register(LangEN, cliErrEN)
	register(LangJA, cliErrJA)
}

var cliErrEN = map[string]string{}

var cliErrJA = map[string]string{}
