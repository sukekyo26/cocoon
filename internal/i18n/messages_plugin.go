package i18n

func init() {
	register(LangEN, messagesEN_plugin)
	register(LangJA, messagesJA_plugin)
}

//nolint:gochecknoglobals,revive // catalog tables are file-scoped by design.
var messagesEN_plugin = map[string]string{
	"plugin_scaffold_intro":             "Scaffold a new plugin under %s/%s.",
	"plugin_scaffold_prompt_name":       "Display name (e.g. \"GitHub CLI\")",
	"plugin_scaffold_prompt_desc":       "Description (include upstream URL in parentheses)",
	"plugin_scaffold_prompt_default":    "Mark plugin enabled by default?",
	"plugin_scaffold_prompt_root":       "Run install.sh as root?",
	"plugin_scaffold_prompt_versioned":  "Generate $PIN / $CHECKSUM_* boilerplate?",
	"plugin_scaffold_prompt_template":   "install.sh template",
	"plugin_scaffold_prompt_user_hook":  "Also generate install_user.sh?",
	"plugin_scaffold_invalid_id":        "Invalid plugin id %q (must match ^[a-z][a-z0-9-]*$)",
	"plugin_scaffold_missing_id":        "plugin id is required",
	"plugin_scaffold_missing_flag":      "--non-interactive requires --%s",
	"plugin_scaffold_blank_name":        "--name must not be blank or whitespace-only",
	"plugin_scaffold_blank_description": "--description must not be blank or whitespace-only",
	"plugin_scaffold_desc_missing_url":  "--description must include an upstream URL in parentheses, e.g. \"(https://example.com)\"",
	"plugin_scaffold_desc_invalid":      "--description is invalid",
	"plugin_scaffold_unknown_template":  "unknown template %q (want curl-pipe, tarball, or generic)",
	"plugin_scaffold_tarball_needs_ver": "--template tarball requires --version-capable",
	"plugin_scaffold_dir_exists":        "%s already exists; pass --force to overwrite",
	"plugin_scaffold_validation_failed": "generated plugin.toml failed strict validation; rolled back",
	"plugin_scaffold_done":              "OK: scaffolded %s (%d files)",
	"plugin_scaffold_no_plugins_dir": "scaffold needs a writable plugins dir.\n" +
		"  - run inside a cocoon project (workspace.toml discoverable from cwd), or\n" +
		"  - pass --plugins-dir <path> explicitly.",
}

//nolint:gochecknoglobals,revive // catalog tables are file-scoped by design.
var messagesJA_plugin = map[string]string{
	"plugin_scaffold_intro":             "%s/%s に新規プラグインを生成します。",
	"plugin_scaffold_prompt_name":       "表示名 (例: \"GitHub CLI\")",
	"plugin_scaffold_prompt_desc":       "説明 (上流 URL を括弧内に含めること)",
	"plugin_scaffold_prompt_default":    "デフォルトで有効化する?",
	"plugin_scaffold_prompt_root":       "install.sh を root で実行?",
	"plugin_scaffold_prompt_versioned":  "$PIN / $CHECKSUM_* の雛形を生成する?",
	"plugin_scaffold_prompt_template":   "install.sh のテンプレート",
	"plugin_scaffold_prompt_user_hook":  "install_user.sh も生成する?",
	"plugin_scaffold_invalid_id":        "プラグイン ID %q が不正です (^[a-z][a-z0-9-]*$ に一致が必要)",
	"plugin_scaffold_missing_id":        "プラグイン ID は必須です",
	"plugin_scaffold_missing_flag":      "--non-interactive 時は --%s が必須です",
	"plugin_scaffold_blank_name":        "--name は空白だけにできません",
	"plugin_scaffold_blank_description": "--description は空白だけにできません",
	"plugin_scaffold_desc_missing_url":  "--description には上流 URL を括弧で囲んで含めてください (例: \"(https://example.com)\")",
	"plugin_scaffold_desc_invalid":      "--description の値が不正です",
	"plugin_scaffold_unknown_template":  "未知のテンプレート %q (curl-pipe, tarball, generic のいずれか)",
	"plugin_scaffold_tarball_needs_ver": "--template tarball は --version-capable が必須です",
	"plugin_scaffold_dir_exists":        "%s は既に存在します。上書きするには --force を指定してください",
	"plugin_scaffold_validation_failed": "生成された plugin.toml が strict 検証に失敗したためロールバックしました",
	"plugin_scaffold_done":              "OK: %s を生成 (%d ファイル)",
	"plugin_scaffold_no_plugins_dir": "scaffold には書き込み可能なプラグインディレクトリが必要です。\n" +
		"  - cocoon プロジェクト内 (cwd から workspace.toml が見える場所) で実行するか、\n" +
		"  - --plugins-dir <path> を明示的に指定してください。",
}
