package i18n

func init() {
	register(LangEN, messagesEN_plugin)
	register(LangJA, messagesJA_plugin)
}

//nolint:gochecknoglobals,revive // catalog tables are file-scoped by design.
var messagesEN_plugin = map[string]string{
	"plugin_scaffold_intro":            "Scaffold a new plugin under %s/%s.",
	"plugin_scaffold_prompt_name":      "Display name (e.g. \"GitHub CLI\")",
	"plugin_scaffold_prompt_desc":      "Description (no URL)",
	"plugin_scaffold_prompt_url":       "Upstream URL (e.g. https://github.com/owner/repo)",
	"plugin_scaffold_prompt_default":   "Mark plugin enabled by default?",
	"plugin_scaffold_prompt_root":      "Run the install script as root?",
	"plugin_scaffold_prompt_versioned": "Generate $PIN / $CHECKSUM_* boilerplate?",
	"plugin_scaffold_prompt_template":  "install method category",
	"plugin_scaffold_prompt_user_hook": "Also generate install_user.sh?",
	"plugin_scaffold_prompt_user_hook_desc": "install_user.sh runs as the unprivileged container user (USERNAME),\n" +
		"regardless of [install].requires_root.\n" +
		"\n" +
		"Use it when install.sh runs as root (requires_root=true) AND the\n" +
		"plugin needs to write to user-owned files (~/.bashrc,\n" +
		"~/.config/<tool>, ~/.local/share, ...) OR run a setup command as\n" +
		"the unprivileged user (`<tool> init`, `git clone ~/.<tool>`,\n" +
		"`conda init bash`, ...).\n" +
		"\n" +
		"Skip it when install.sh already runs as user (requires_root=false),\n" +
		"the plugin only needs ENV vars (use [install].env), or a single\n" +
		"rc-file line that fits in install.sh itself.\n" +
		"\n" +
		"Examples: yes -> starship (root binary + user rc edit).\n" +
		"          no  -> go, docker-cli, bun, uv, ... (most catalog plugins).",
	"plugin_scaffold_invalid_id":        "Invalid plugin id %q (must match ^[a-z][a-z0-9-]*$)",
	"plugin_scaffold_missing_id":        "plugin id is required",
	"plugin_scaffold_missing_flag":      "--non-interactive requires --%s",
	"plugin_scaffold_blank_name":        "--name must not be blank or whitespace-only",
	"plugin_scaffold_blank_description": "--description must not be blank or whitespace-only",
	"plugin_scaffold_blank_url":         "--url must not be blank or whitespace-only",
	"plugin_scaffold_invalid_url":       "--url must start with https:// and contain no whitespace (e.g. https://github.com/owner/repo)",
	"plugin_scaffold_unknown_template":  "unknown template %q (want installer, binary, apt, or archive — see docs/plugins.md)",
	"plugin_scaffold_binary_needs_ver":  "--template binary requires --version-capable",
	"plugin_scaffold_dir_exists":        "%s already exists; pass --force to overwrite",
	"plugin_scaffold_validation_failed": "generated plugin.toml failed strict validation; rolled back",
	"plugin_scaffold_done":              "OK: scaffolded %s (%d files)",
	"plugin_scaffold_no_plugins_dir": "scaffold needs a writable plugins dir.\n" +
		"  - run inside a cocoon project (a cocoon.toml or workspace.toml is discoverable from cwd), or\n" +
		"  - pass --plugins-dir <path> explicitly.",
}

//nolint:gochecknoglobals,revive // catalog tables are file-scoped by design.
var messagesJA_plugin = map[string]string{
	"plugin_scaffold_intro":            "%s/%s に新規プラグインを生成します。",
	"plugin_scaffold_prompt_name":      "表示名 (例: \"GitHub CLI\")",
	"plugin_scaffold_prompt_desc":      "説明 (URL は含めない)",
	"plugin_scaffold_prompt_url":       "上流 URL (例: https://github.com/owner/repo)",
	"plugin_scaffold_prompt_default":   "デフォルトで有効化する?",
	"plugin_scaffold_prompt_root":      "install スクリプトを root で実行?",
	"plugin_scaffold_prompt_versioned": "$PIN / $CHECKSUM_* の雛形を生成する?",
	"plugin_scaffold_prompt_template":  "インストール方式カテゴリ",
	"plugin_scaffold_prompt_user_hook": "install_user.sh も生成する?",
	"plugin_scaffold_prompt_user_hook_desc": "install_user.sh は [install].requires_root の値に関わらず、常に\n" +
		"非特権ユーザー (USERNAME) で実行される。\n" +
		"\n" +
		"必要なケース: install.sh が root で動く (requires_root=true) かつ、\n" +
		"プラグインが ~/.bashrc / ~/.config/<tool> / ~/.local/share/...\n" +
		"などユーザー所有ファイルに書き込む、もしくは `<tool> init` や\n" +
		"`git clone ~/.<tool>` / `conda init bash` のような初期化コマンドを\n" +
		"非特権ユーザーとして実行したい場合。\n" +
		"\n" +
		"不要なケース: install.sh が既に user 権限で動く (requires_root=false) /\n" +
		"ENV のみで完結する ([install].env で書ける) / 単一行の rc 追記なら\n" +
		"install.sh 内で済む。\n" +
		"\n" +
		"例: yes -> starship (root でバイナリ配置 → user で rc 追記)。\n" +
		"    no  -> go / docker-cli / bun / uv など (ほとんどのプラグイン)。",
	"plugin_scaffold_invalid_id":        "プラグイン ID %q が不正です (^[a-z][a-z0-9-]*$ に一致が必要)",
	"plugin_scaffold_missing_id":        "プラグイン ID は必須です",
	"plugin_scaffold_missing_flag":      "--non-interactive 時は --%s が必須です",
	"plugin_scaffold_blank_name":        "--name は空白だけにできません",
	"plugin_scaffold_blank_description": "--description は空白だけにできません",
	"plugin_scaffold_blank_url":         "--url は空白だけにできません",
	"plugin_scaffold_invalid_url":       "--url は https:// で始まり空白を含まない URL である必要があります (例: https://github.com/owner/repo)",
	"plugin_scaffold_unknown_template":  "未知のテンプレート %q (installer / binary / apt / archive のいずれかを指定 — 詳細は docs/plugins.ja.md)",
	"plugin_scaffold_binary_needs_ver":  "--template binary は --version-capable が必須です",
	"plugin_scaffold_dir_exists":        "%s は既に存在します。上書きするには --force を指定してください",
	"plugin_scaffold_validation_failed": "生成された plugin.toml が strict 検証に失敗したためロールバックしました",
	"plugin_scaffold_done":              "OK: %s を生成 (%d ファイル)",
	"plugin_scaffold_no_plugins_dir": "scaffold には書き込み可能なプラグインディレクトリが必要です。\n" +
		"  - cocoon プロジェクト内 (cwd から cocoon.toml または workspace.toml が見える場所) で実行するか、\n" +
		"  - --plugins-dir <path> を明示的に指定してください。",
}
