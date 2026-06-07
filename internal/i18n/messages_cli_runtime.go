package i18n

// Runtime (non-help, non-error) output for the self-update / lock / plugin
// subcommands and the init wizard's screen-reader fallback. Help text lives in
// messages_cli_help.go; error text is carried by its own keys elsewhere.

func init() {
	register(LangEN, cliRuntimeEN)
	register(LangJA, cliRuntimeJA)
}

var cliRuntimeEN = map[string]string{
	// self-update.
	"selfupdate_dev_build":       "self-update: cannot self-update a dev build (no version baked in)",
	"selfupdate_label_current":   "current version :",
	"selfupdate_label_latest":    "latest release  :",
	"selfupdate_up_to_date":      "already up to date",
	"selfupdate_newer_available": "newer release %s available; rerun without --check-only to install",
	"selfupdate_downloading":     "downloading %s...",
	"selfupdate_updated":         "updated cocoon to %s at %s",

	// lock.
	"lock_nothing_to_lock":           "no lockable plugins; nothing to lock",
	"lock_up_to_date":                "%s is up to date (%d plugin(s))",
	"lock_reused":                    "Reused %s %s",
	"lock_locked":                    "Locked %s %s",
	"lock_wrote":                     "Wrote %s (%d plugin(s))",
	"lock_ignoring_malformed":        "ignoring malformed %s (%v); regenerating from scratch",
	"lock_skipped_sourceless_latest": "Skipped %s: its latest version cannot be determined automatically, so \"latest\" cannot be locked; cocoon gen installs the latest at build time (not reproducible). To lock it, set a version in [plugins].enable: \"%s=<version>\"",

	// plugin pin.
	"plugin_pin_updated_enable":          "Updated %s: [plugins].enable %q",
	"plugin_pin_updated_method":          "Updated %s: [plugins.methods] %s = %q",
	"plugin_pin_snippet_enable_header":   "# Add (or update) this entry in the [plugins].enable array in your config file:",
	"plugin_pin_snippet_header":          "# Add the following to your config file:",
	"plugin_pin_snippet_enable_section":  "# In the [plugins].enable array:",
	"plugin_pin_snippet_methods_section": "# Under [plugins.methods]:",

	// init wizard accessible (screen-reader) fallback.
	"init_accessible_choose":   "Choose by number or type a tag: ",
	"init_accessible_empty":    "empty input not accepted",
	"init_accessible_type_tag": "Or type any tag directly.",
}

var cliRuntimeJA = map[string]string{
	// self-update.
	"selfupdate_dev_build":       "self-update: 開発ビルドは自己更新できません (バージョンが埋め込まれていません)",
	"selfupdate_label_current":   "現在のバージョン :",
	"selfupdate_label_latest":    "最新リリース     :",
	"selfupdate_up_to_date":      "すでに最新です",
	"selfupdate_newer_available": "新しいリリース %s が利用可能です。インストールするには --check-only なしで再実行してください",
	"selfupdate_downloading":     "%s をダウンロードしています...",
	"selfupdate_updated":         "cocoon を %s に更新しました (%s)",

	// lock.
	"lock_nothing_to_lock":           "lock 可能なプラグインがありません。lock する対象がありません",
	"lock_up_to_date":                "%s は最新です (%d 個のプラグイン)",
	"lock_reused":                    "再利用しました %s %s",
	"lock_locked":                    "ロックしました %s %s",
	"lock_wrote":                     "%s を書き出しました (%d 個のプラグイン)",
	"lock_ignoring_malformed":        "破損した %s を無視します (%v)。最初から再生成します",
	"lock_skipped_sourceless_latest": "%s をスキップ: 最新バージョンを自動で特定できないため \"latest\" を lock できません。cocoon gen がビルド時に最新を導入します（再現性なし）。lock するには [plugins].enable でバージョンを指定してください: \"%s=<version>\"",

	// plugin pin.
	"plugin_pin_updated_enable":          "%s を更新しました: [plugins].enable %q",
	"plugin_pin_updated_method":          "%s を更新しました: [plugins.methods] %s = %q",
	"plugin_pin_snippet_enable_header":   "# 設定ファイルの [plugins].enable 配列に次のエントリを追加（または更新）してください:",
	"plugin_pin_snippet_header":          "# 設定ファイルに次を追加してください:",
	"plugin_pin_snippet_enable_section":  "# [plugins].enable 配列に:",
	"plugin_pin_snippet_methods_section": "# [plugins.methods] の下に:",

	// init wizard accessible (screen-reader) fallback.
	"init_accessible_choose":   "番号で選択するかタグを入力してください: ",
	"init_accessible_empty":    "空の入力は受け付けられません",
	"init_accessible_type_tag": "または任意のタグを直接入力してください。",
}
