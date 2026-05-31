//nolint:dupl // each catalog file shares the same boilerplate by design.
package i18n

func init() {
	register(LangEN, messagesEN_cliHelp)
	register(LangJA, messagesJA_cliHelp)
}

// helpTemplateEN mirrors cobra v1.10's defaultHelpTemplate verbatim so
// the English `--help` layout stays byte-identical to upstream.
const helpTemplateEN = `{{with (or .Long .Short)}}{{. | trimTrailingWhitespaces}}

{{end}}{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}`

// usageTemplateEN mirrors cobra v1.10's defaultUsageTemplate verbatim.
// Keep the structure (placeholders, conditionals, whitespace) identical;
// usageTemplateJA below only swaps the literal section headers.
const usageTemplateEN = `Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

Available Commands:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

Additional Commands:{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`

// usageTemplateJA preserves usageTemplateEN's structure (conditionals,
// placeholders, whitespace) and only swaps the literal section headers
// and the trailing hint line.
const usageTemplateJA = `使い方:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

エイリアス:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

例:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

サブコマンド:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

その他のコマンド:{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

フラグ:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

グローバルフラグ:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

その他のヘルプトピック:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

各サブコマンドの詳細は "{{.CommandPath}} [command] --help" で確認できます。{{end}}
`

// versionTemplateFull keeps cocoon's existing "<version>\n" style, which
// predates this i18n pass and is shared across en/ja.
const versionTemplateFull = "{{.Version}}\n"

// messagesEN_cliHelp holds cobra `--help` output for every cocoon
// subcommand: Short / Long / Example, Flag usages, and the full help /
// usage / version templates. Keys use a `cmd_<name>_*` /
// `flag_<cmd>_<flag>_usage` / `help_*` / `usage_*` / `version_*` namespace.
//
//nolint:gochecknoglobals,revive // catalog tables are file-scoped by design.
var messagesEN_cliHelp = map[string]string{
	"help_template_full":    helpTemplateEN,
	"usage_template_full":   usageTemplateEN,
	"version_template_full": versionTemplateFull,

	"flag_global_help_usage":    "help for %s",
	"flag_global_version_usage": "version for %s",

	// cocoon (root)
	"cmd_root_short": "Project-aware container workspace generator",
	"cmd_root_long": `cocoon — project-aware container workspace generator

Run cocoon from any project directory to read its workspace.toml and
materialize a Dev Container or plain docker-compose stack tailored to
that repository.`,

	// cocoon version
	"cmd_version_short": "Print the cocoon binary version",

	// Hidden `<cmd> help` alias attached via clihelpers.AttachHelpAlias.
	"cmd_help_alias_short": "Show usage for this command",

	// cocoon init
	"cmd_init_short": "Create workspace.toml in the current directory",
	"cmd_init_long": `cocoon init — generate workspace.toml in the current directory

Asks (when running interactively) for the container service name, the
inside-the-container username, the base image / version, the mount range,
whether to emit .devcontainer/devcontainer.json, and which categories
of common apt packages to install. service_name and username have no
default — you must type them — because cocoon refuses to bake either
the cwd basename or your host $USER into a file you may commit.

The interactive flow asks each question on its own screen. Empty
service-name / username are rejected on submission. shift+tab does
not navigate back across questions — re-run cocoon init to fix an
earlier answer. (Each prompt being its own form sidesteps a class of
viewport-sizing bugs in huh's multi-Group + OptionsFunc combination
that caused the cursor indicator to stay pinned while options
scrolled under it.)

Use --yes plus --service-name / --username (both required when --yes
is set) and any of --image / --image-version / --shell / --mount-root /
--dir / --devcontainer / --apt-categories / --plugins / --alias-bundles
to drive non-interactively from CI.`,
	"flag_init_yes_usage":             "skip optional prompts; --service-name and --username then required",
	"flag_init_service_name_usage":    "compose service name (required with --yes)",
	"flag_init_username_usage":        "in-container user (required with --yes)",
	"flag_init_image_usage":           "base image: %s",
	"flag_init_image_version_usage":   "base image tag — any well-formed Docker tag is accepted; --image must also be set",
	"flag_init_shell_usage":           "container login shell: %s (default: bash)",
	"flag_init_mount_root_usage":      `mount range: "." (cwd, default) or ".." (parent)`,
	"flag_init_dir_usage":             `container workdir parent under /home/<user>/ (default "workspace"; slashes allowed for nested paths, e.g. "work/myproject")`,
	"flag_init_devcontainer_usage":    "force-enable .devcontainer/devcontainer.json output",
	"flag_init_no_devcontainer_usage": "skip .devcontainer/devcontainer.json output",
	"flag_init_certificates_usage":    "force-enable [certificates] auto-bake from ~/.cocoon/certs/",
	"flag_init_no_certificates_usage": "skip the [certificates] section (default off)",
	"flag_init_secure_usage":          "preset [container.security_opt] no_new_privileges = true (disables in-container sudo)",
	"flag_init_no_secure_usage":       "leave no_new_privileges unset; in-container sudo stays available (default)",
	"flag_init_image_path_fix_usage": "force the language-image PATH/install-prefix auto-injection " +
		"(requires --image=node|python|golang|rust|denoland/deno; default on for those)",
	"flag_init_no_image_path_fix_usage": "skip the language-image PATH/install-prefix auto-injection " +
		"(requires --image=node|python|golang|rust|denoland/deno)",
	"flag_init_apt_categories_usage": "comma-separated apt category IDs (skips the multi-select prompt)",
	"flag_init_plugins_usage":        "comma-separated plugin IDs to enable (skips the plugin multi-select prompt)",
	"flag_init_plugin_versions_usage": "comma-separated <id>=<ref> pins for version_capable plugins " +
		"(each <id> must also appear in --plugins)",
	"flag_init_plugin_methods_usage": "comma-separated <id>=<method> picks for plugins that declare multiple " +
		"[install.methods] (each <id> must also appear in --plugins; <method> must be a declared key)",
	"flag_init_alias_bundles_usage": "comma-separated shell-alias bundle IDs " +
		"(skips the bundles multi-select prompt; e.g. git,ls)",
	"flag_init_ports_usage": "comma-separated docker-compose short-form port mappings — " +
		"[HOST_IP:][HOST:]CONTAINER[/PROTOCOL]; numeric ranges (N-M) and tcp|udp are accepted " +
		"(e.g. 3000,3000-3005,8000:8000,127.0.0.1:5432:5432/tcp,6060:6060/udp); skips the ports prompt",
	"flag_init_force_usage": "overwrite an existing workspace.toml",

	// cocoon gen
	"cmd_gen_short": "Generate .devcontainer/ artifacts from workspace.toml",
	"cmd_gen_long": `cocoon gen — generate .devcontainer/{Dockerfile, docker-compose.yml, devcontainer.json}

Discovers workspace.toml from the current directory (walking parent
directories until a .git boundary or $HOME), assembles the layered
plugin catalog (embedded < user < project), and writes the generated
artifacts under .devcontainer/. Plugin install scripts are inlined into
the generated Dockerfile, so the build needs no external context
beyond the project tree.

After generation, start the container yourself:

  docker compose -f .devcontainer/docker-compose.yml up -d

…or open the project in VS Code and pick "Reopen in Container".`,
	"flag_gen_workspace_usage": "path to workspace.toml (default: discovered from cwd)",
	"flag_gen_output_usage":    "project root to write generated artifacts under (default: directory of workspace.toml)",

	// cocoon gen workspace
	"cmd_gen_workspace_short": "Generate <name>.code-workspace at the project root from workspace.toml",
	"cmd_gen_workspace_long": `cocoon gen workspace — generate a VS Code .code-workspace file

Reads [code_workspace] from workspace.toml and writes <name>.code-workspace
at the project root (not under .devcontainer/). Folder paths are "~"-expanded
and relativized against the directory the .code-workspace file is written to
(the workspace.toml directory by default, or --output when set), so an entry
like "~/.claude" resolves to a relative path that VS Code can traverse from
that location.

Use --folder to add ad-hoc folders without editing workspace.toml; flag
entries are appended after the declarative [code_workspace].folders list.
A folder name is auto-derived from the basename of the resolved path; pass
"<path>=<name>" via --folder to override (e.g. --folder ~/.claude=Claude).

Examples:

  cocoon gen workspace
  cocoon gen workspace --folder ~/.config/nvim
  cocoon gen workspace --folder ~/.claude=Claude --name my-stack`,
	"flag_gen_workspace_workspace_usage": "path to workspace.toml (default: discovered from cwd)",
	"flag_gen_workspace_output_usage":    "project root to write under (default: directory of workspace.toml)",
	"flag_gen_workspace_name_usage": "output file basename without extension " +
		"(default: [code_workspace].name or project directory basename)",
	"flag_gen_workspace_folder_usage": `extra folder, appended after [code_workspace].folders; ` +
		`pass "<path>=<name>" to override the auto-derived name`,

	// cocoon plugin (parent)
	"cmd_plugin_short": "Inspect and author cocoon plugins (list / show / pin / scaffold)",
	"cmd_plugin_long": `cocoon plugin — inspect and author cocoon plugins

Subcommands:
  list       list every plugin available in the layered view (project > user > embedded)
  show       print the resolved manifest for one plugin id
  pin        print a workspace.toml [plugins.versions.<id>] block
  scaffold   create a new <id>/ directory from a template

To use a plugin, add its id to [plugins].enable in workspace.toml — the
embedded catalog is picked up automatically. To customise an embedded
plugin, the supported workflow is "cocoon plugin scaffold <new-id>" and
adapting the logic. If you have a clone of the cocoon source repo (or an
unpacked source tarball), copying the embedded source from
internal/plugin/catalog/<id>/ into ~/.cocoon/plugins/<id>/ is a shortcut;
single-binary installs do not include the embedded source on disk.`,

	// cocoon plugin list
	"cmd_plugin_list_short": "List available plugins with their source (embedded / user / project)",
	"cmd_plugin_list_long": `cocoon plugin list — show every available plugin

The list combines the embedded catalog with optional user (` + "`~/.cocoon/plugins`" + `)
and project (` + "`<project>/.cocoon/plugins`" + `) overlays. Same-id directories are
not merged; the highest-priority layer wins (project > user > embedded). The
SOURCE column shows which layer each id is currently served from.`,
	"flag_plugin_list_source_usage": "only show this layer (%s, %s, %s)",

	// cocoon plugin show
	"cmd_plugin_show_short": "Print the resolved manifest for a single plugin",
	"cmd_plugin_show_long": `cocoon plugin show — print the resolved plugin manifest for <id>

Resolves <id> through the same project > user > embedded layered view as
` + "`cocoon plugin list`" + `, prints the metadata, install hints, apt packages,
and the source layer it was read from.`,

	// cocoon plugin pin
	"cmd_plugin_pin_short": "Emit inline-table pin lines for workspace.toml (stdout, or in-place with --write)",
	"cmd_plugin_pin_long": `cocoon plugin pin — emit pin lines for [plugins.versions] (and optionally [plugins.methods])

By default a ` + "`<id> = { pin = \"<ref>\" }`" + ` line is printed to stdout for you
to paste under the [plugins.versions] section in workspace.toml. With
--write the line is upserted in place (inserted, or the existing
<id> = { ... } line is replaced); comments and blank lines outside the
target line are preserved verbatim.

Use the --amd64-checksum / --arm64-checksum flags when the upstream
release ships per-arch SHA256 sums you want the install script to
verify. Plugins that declare verify = "pgp" in plugin.toml verify
downloads against a bundled signature instead; passing checksum flags
to those plugins is rejected.

Pass --method <name> for plugins that declare two or more entries under
[install.methods] in their plugin.toml — the pin then writes (or prints)
both lines together: ` + "`<id> = \"<method>\"`" + ` for [plugins.methods] and
the inline-table line for [plugins.versions]. Checksums are workspace-
scoped (not per-method); when switching methods, refresh
--amd64-checksum / --arm64-checksum so the install script's SHA256
verification still matches the new artifact.`,
	"flag_plugin_pin_amd64_checksum_usage": "sha256 of the amd64 artifact (optional)",
	"flag_plugin_pin_arm64_checksum_usage": "sha256 of the arm64 artifact (optional)",
	"flag_plugin_pin_method_usage": "install method name; only meaningful for plugins that declare [install.methods]. " +
		"When set, the command pins both [plugins.methods] and [plugins.versions] in a " +
		"single workspace.toml read-write. When omitted, only [plugins.versions] is updated " +
		"(the existing [plugins.methods] entry — or the plugin's default_method — stays in effect).",
	"flag_plugin_pin_write_usage": "upsert the inline-table line in workspace.toml (auto-discovered from cwd)",

	// cocoon plugin scaffold
	"cmd_plugin_scaffold_cmd_short": "Create a new <id>/ plugin directory " +
		"(default <workspace>/.cocoon/plugins; --plugins-dir overrides)",
	"cmd_plugin_scaffold_cmd_long": `cocoon plugin scaffold — create a new <id>/ directory under the project plugins overlay

By default the new directory is created under <workspace>/.cocoon/plugins/<id>/,
auto-discovered from the nearest workspace.toml. Pass --plugins-dir <path> to
override (the path is taken as-is, joined with <id>).

The new directory contains a plugin.toml declaring a single
[install.methods.<category>] entry and an install.<category>.sh skeleton
matching the chosen template — installer (curl|bash), binary (single
binary download), apt (apt repo / .deb), or archive (multi-file
extract). With --with-install-user a second install_user.sh hook is
emitted (kept plugin-scoped, not per-method).`,
	"flag_plugin_scaffold_cmd_plugins_dir_usage":       "output directory (default: <workspace>/.cocoon/plugins, auto-discovered from workspace.toml)",
	"flag_plugin_scaffold_cmd_name_usage":              `display name (e.g. "GitHub CLI")`,
	"flag_plugin_scaffold_cmd_description_usage":       "short description (no URL)",
	"flag_plugin_scaffold_cmd_url_usage":               "upstream URL (e.g. https://github.com/owner/repo)",
	"flag_plugin_scaffold_cmd_default_usage":           "mark plugin enabled by default",
	"flag_plugin_scaffold_cmd_requires_root_usage":     "install script runs as root",
	"flag_plugin_scaffold_cmd_version_capable_usage":   "generate $PIN / $CHECKSUM_* boilerplate",
	"flag_plugin_scaffold_cmd_template_usage":          "install method category: installer | binary | apt | archive (catalog-canonical names — see docs/plugins.md)",
	"flag_plugin_scaffold_cmd_with_install_user_usage": "also generate install_user.sh",
	"flag_plugin_scaffold_cmd_non_interactive_usage":   "skip interactive prompts; require all fields above",
	"flag_plugin_scaffold_cmd_force_usage":             "overwrite <plugins-dir>/<id>/ if it already exists",

	// cocoon self-update
	"cmd_self_update_short": "Replace this binary with the latest released version",
	"cmd_self_update_long": `cocoon self-update — replace the current binary with the latest release

Hits the GitHub Releases API for sukekyo26/cocoon, compares the
release tag against the build's compiled-in version string, and on
update downloads the matching cocoon-<os>-<arch> asset under SHA-256
verification before atomically replacing this executable.

Exit codes:
  0   already up to date, or replacement succeeded
  100 (only with --check-only) a newer version exists
  1   any other failure`,
	"flag_self_update_check_only_usage": "exit 0 if up to date, exit 100 if a newer release exists; never download",
	"flag_self_update_force_usage":      "reinstall even when the local binary is already the latest version",

	// cobra auto-generated `completion` subtree
	"cmd_completion_short": "Generate the autocompletion script for the specified shell",
	"cmd_completion_long": `Generate the autocompletion script for cocoon for the specified shell.
See each sub-command's help for details on how to use the generated script.
`,
	"cmd_completion_bash_short":       "Generate the autocompletion script for bash",
	"cmd_completion_zsh_short":        "Generate the autocompletion script for zsh",
	"cmd_completion_fish_short":       "Generate the autocompletion script for fish",
	"cmd_completion_powershell_short": "Generate the autocompletion script for powershell",

	// cobra auto-registered `help` subcommand
	"cmd_help_subcommand_short": "Help about any command",
	"cmd_help_subcommand_long": `Help provides help for any command in the application.
Simply type %s help [path to command] for full details.`,
}

//nolint:gochecknoglobals,revive // catalog tables are file-scoped by design.
var messagesJA_cliHelp = map[string]string{
	"help_template_full":    helpTemplateEN, // structure-only; no literal headers to translate
	"usage_template_full":   usageTemplateJA,
	"version_template_full": versionTemplateFull,

	"flag_global_help_usage":    "%s のヘルプを表示",
	"flag_global_version_usage": "%s のバージョンを表示",

	// cocoon (root)
	"cmd_root_short": "プロジェクト連動のコンテナ ワークスペース生成ツール",
	"cmd_root_long": `cocoon — プロジェクト連動のコンテナ ワークスペース生成ツール

プロジェクトディレクトリで cocoon を実行すると workspace.toml を読み取り、
そのリポジトリ専用の Dev Container または docker-compose スタックを生成します。`,

	// cocoon version
	"cmd_version_short": "cocoon バイナリのバージョンを表示",

	// Hidden `<cmd> help` alias attached via clihelpers.AttachHelpAlias.
	"cmd_help_alias_short": "このコマンドの使い方を表示",

	// cocoon init
	"cmd_init_short": "カレントディレクトリに workspace.toml を生成",
	"cmd_init_long": `cocoon init — カレントディレクトリに workspace.toml を生成

対話実行時はサービス名・コンテナ内ユーザー名・ベースイメージ/バージョン・
マウント範囲・.devcontainer/devcontainer.json を生成するかどうか・
よく使う apt パッケージのカテゴリを順に質問します。
service_name と username にデフォルトは無く、必ず入力してください。
cocoon は cwd のディレクトリ名やホストの $USER を、コミットされ得る
ファイルに自動で焼き込みません。

対話フローは各質問を 1 画面ずつ表示します。service-name / username の
空入力は送信時に拒否されます。shift+tab で前の質問へ戻ることは
できません（前の回答を直すには cocoon init を再実行してください）。
各プロンプトを独立フォームにしているのは、huh の multi-Group +
OptionsFunc 組み合わせで発生する viewport サイジング起因のカーソル
固定問題を回避するためです。

CI など非対話実行では --yes に加えて --service-name / --username
（--yes 指定時は両方必須）と必要に応じて --image / --image-version /
--shell / --mount-root / --dir / --devcontainer / --apt-categories /
--plugins / --alias-bundles を渡します。`,
	"flag_init_yes_usage":             "任意プロンプトをスキップ（--service-name と --username が必須になります）",
	"flag_init_service_name_usage":    "Compose サービス名（--yes 指定時は必須）",
	"flag_init_username_usage":        "コンテナ内ユーザー名（--yes 指定時は必須）",
	"flag_init_image_usage":           "ベースイメージ: %s",
	"flag_init_image_version_usage":   "ベースイメージのタグ（任意の妥当な Docker タグを受け付け。--image も併せて指定が必要）",
	"flag_init_shell_usage":           "コンテナのログインシェル: %s（既定: bash）",
	"flag_init_mount_root_usage":      `マウント範囲: "."（cwd、既定）または ".."（親ディレクトリ）`,
	"flag_init_dir_usage":             `/home/<user>/ 配下のコンテナ作業ディレクトリ親（既定 "workspace"。"work/myproject" のように "/" 区切りでネスト可）`,
	"flag_init_devcontainer_usage":    ".devcontainer/devcontainer.json の生成を強制",
	"flag_init_no_devcontainer_usage": ".devcontainer/devcontainer.json の生成をスキップ",
	"flag_init_certificates_usage":    "~/.cocoon/certs/ からの [certificates] 自動取り込みを強制有効化",
	"flag_init_no_certificates_usage": "[certificates] セクションをスキップ（既定 off）",
	"flag_init_secure_usage":          "[container.security_opt] no_new_privileges = true を事前設定（コンテナ内 sudo を無効化）",
	"flag_init_no_secure_usage":       "no_new_privileges を設定しない。コンテナ内 sudo は引き続き利用可（既定）",
	"flag_init_image_path_fix_usage": "言語イメージの PATH / インストール先自動設定を強制有効化" +
		"（--image=node|python|golang|rust|denoland/deno が必須。それらは既定 on）",
	"flag_init_no_image_path_fix_usage": "言語イメージの PATH / インストール先自動設定をスキップ" +
		"（--image=node|python|golang|rust|denoland/deno が必須）",
	"flag_init_apt_categories_usage": "カンマ区切り apt カテゴリ ID（複数選択プロンプトをスキップ）",
	"flag_init_plugins_usage":        "有効化するプラグイン ID のカンマ区切り（プラグイン複数選択プロンプトをスキップ）",
	"flag_init_plugin_versions_usage": "version_capable プラグイン向けの <id>=<ref> ピンをカンマ区切りで指定" +
		"（各 <id> は --plugins にも含めること）",
	"flag_init_plugin_methods_usage": "複数の [install.methods] を持つプラグイン向けの <id>=<method> 選択をカンマ区切りで指定" +
		"（各 <id> は --plugins にも含めること。<method> は宣言済みキーであること）",
	"flag_init_alias_bundles_usage": "シェルエイリアス bundle ID のカンマ区切り" +
		"（bundles 複数選択プロンプトをスキップ。例: git,ls）",
	"flag_init_ports_usage": "docker-compose の short-form ポートマッピングをカンマ区切りで指定 — " +
		"[HOST_IP:][HOST:]CONTAINER[/PROTOCOL]。数値レンジ (N-M) と tcp|udp 可" +
		"（例: 3000,3000-3005,8000:8000,127.0.0.1:5432:5432/tcp,6060:6060/udp）。ports プロンプトをスキップ",
	"flag_init_force_usage": "既存の workspace.toml を上書き",

	// cocoon gen
	"cmd_gen_short": "workspace.toml から .devcontainer/ の成果物を生成",
	"cmd_gen_long": `cocoon gen — .devcontainer/{Dockerfile, docker-compose.yml, devcontainer.json} を生成

カレントディレクトリから親方向に workspace.toml を探索し（.git 境界または $HOME で停止）、
レイヤー化されたプラグインカタログ（embedded < user < project）を組み立てて、
生成物を .devcontainer/ 配下に書き出します。プラグインの install スクリプトは
生成済み Dockerfile に直接埋め込まれるため、ビルドにプロジェクトツリー外の
外部コンテキストは不要です。

生成後はコンテナを自分で起動してください:

  docker compose -f .devcontainer/docker-compose.yml up -d

…または VS Code でプロジェクトを開き「Reopen in Container」を選びます。`,
	"flag_gen_workspace_usage": "workspace.toml のパス（既定: cwd から探索）",
	"flag_gen_output_usage":    "生成物の出力先プロジェクトルート（既定: workspace.toml のあるディレクトリ）",

	// cocoon gen workspace
	"cmd_gen_workspace_short": "workspace.toml から <name>.code-workspace をプロジェクトルートに生成",
	"cmd_gen_workspace_long": `cocoon gen workspace — VS Code の .code-workspace ファイルを生成

workspace.toml の [code_workspace] を読み取り、<name>.code-workspace を
プロジェクトルート（.devcontainer/ 配下ではない）に書き出します。
folders[].path は "~" 展開され、.code-workspace の書き出し先ディレクトリ
（既定で workspace.toml のあるディレクトリ、--output 指定時はそのディレクトリ）を
基準に相対化されるため、"~/.claude" のようなエントリは VS Code が
その場所から辿れる相対パスに解決されます。

workspace.toml を編集せずに一時的なフォルダを追加したい場合は --folder を使います。
フラグ指定エントリは宣言済みの [code_workspace].folders の後ろに追記されます。
フォルダ名は解決されたパスの basename から自動導出されますが、
--folder "<path>=<name>" で上書きできます（例: --folder ~/.claude=Claude）。

例:

  cocoon gen workspace
  cocoon gen workspace --folder ~/.config/nvim
  cocoon gen workspace --folder ~/.claude=Claude --name my-stack`,
	"flag_gen_workspace_workspace_usage": "workspace.toml のパス（既定: cwd から探索）",
	"flag_gen_workspace_output_usage":    "出力先（既定: workspace.toml のあるディレクトリ）",
	"flag_gen_workspace_name_usage": "出力ファイルの basename（拡張子なし）" +
		"（既定: [code_workspace].name もしくはプロジェクトディレクトリの basename）",
	"flag_gen_workspace_folder_usage": `[code_workspace].folders の後ろに追加するフォルダ。` +
		`"<path>=<name>" 形式で自動導出名を上書き可`,

	// cocoon plugin (parent)
	"cmd_plugin_short": "cocoon プラグインを参照・作成 (list / show / pin / scaffold)",
	"cmd_plugin_long": `cocoon plugin — cocoon プラグインの参照・作成

サブコマンド:
  list       レイヤービュー (project > user > embedded) で参照可能な全プラグインを表示
  show       単一プラグイン ID の解決済みマニフェストを表示
  pin        workspace.toml の [plugins.versions.<id>] ブロックを出力
  scaffold   テンプレートから新しい <id>/ ディレクトリを作成

プラグインを使うには workspace.toml の [plugins].enable に id を追加します。
組み込みカタログは自動で読み込まれます。組み込みプラグインをカスタマイズしたい
場合は "cocoon plugin scaffold <new-id>" で新しい ID を作って中身を書き換える
のが想定フローです。cocoon ソースリポジトリの clone（または展開済みのソース
tarball）を持っている場合、internal/plugin/catalog/<id>/ から
~/.cocoon/plugins/<id>/ にコピーする近道もありますが、単一バイナリ配布版には
組み込みソースは同梱されていません。`,

	// cocoon plugin list
	"cmd_plugin_list_short": "利用可能なプラグインとその source (embedded / user / project) を表示",
	"cmd_plugin_list_long": `cocoon plugin list — 利用可能なプラグインを一覧表示

組み込みカタログに加えて、任意のユーザーオーバーレイ (` + "`~/.cocoon/plugins`" + `) と
プロジェクトオーバーレイ (` + "`<project>/.cocoon/plugins`" + `) を統合した一覧を表示します。
同じ id のディレクトリはマージされず、最も優先度の高いレイヤーが採用されます
(project > user > embedded)。SOURCE 列は各 id が現在どのレイヤーから読まれているか
を示します。`,
	"flag_plugin_list_source_usage": "指定したレイヤーのみ表示 (%s / %s / %s)",

	// cocoon plugin show
	"cmd_plugin_show_short": "単一プラグインの解決済みマニフェストを表示",
	"cmd_plugin_show_long": `cocoon plugin show — <id> の解決済みプラグインマニフェストを表示

` + "`cocoon plugin list`" + ` と同じ project > user > embedded のレイヤービューで
<id> を解決し、メタデータ / インストールヒント / apt パッケージ / 読み込み元の
source レイヤーを表示します。`,

	// cocoon plugin pin
	"cmd_plugin_pin_short": "workspace.toml 向けの inline-table pin 行を出力 (stdout、または --write で in-place)",
	"cmd_plugin_pin_long": `cocoon plugin pin — [plugins.versions] (および任意で [plugins.methods]) の pin 行を出力

既定では ` + "`<id> = { pin = \"<ref>\" }`" + ` 行が stdout に出力されるので、
workspace.toml の [plugins.versions] セクションに貼り付けてください。--write を
付けると in-place で upsert されます (新規挿入、または既存の <id> = { ... } 行を
置換)。対象行以外のコメントや空行は verbatim で保持されます。

上流リリースが arch ごとの SHA256 を配布していて、それを install スクリプトに
検証させたい場合は --amd64-checksum / --arm64-checksum を指定します。
plugin.toml に verify = "pgp" を宣言しているプラグインはバンドル署名で検証する
ため、これらのプラグインに checksum フラグを渡すと拒否されます。

plugin.toml の [install.methods] に複数エントリを宣言しているプラグインには
--method <name> を指定します。pin はその場合に
` + "`<id> = \"<method>\"`" + ` 行（[plugins.methods] 向け）と inline-table 行
（[plugins.versions] 向け）の両方をまとめて書き出します(または出力します)。
checksum は workspace スコープ（method 毎ではない）なので、method を切り替える
ときは install スクリプトの SHA256 検証が新成果物に合うよう
--amd64-checksum / --arm64-checksum も更新してください。`,
	"flag_plugin_pin_amd64_checksum_usage": "amd64 成果物の sha256（任意）",
	"flag_plugin_pin_arm64_checksum_usage": "arm64 成果物の sha256（任意）",
	"flag_plugin_pin_method_usage": "install メソッド名。[install.methods] を宣言したプラグインのみ意味を持ちます。" +
		"指定すると単一の workspace.toml 読み書きで [plugins.methods] と [plugins.versions] の両方を pin します。" +
		"省略時は [plugins.versions] のみ更新されます " +
		"（既存の [plugins.methods] エントリ、またはプラグインの default_method が引き続き有効）。",
	"flag_plugin_pin_write_usage": "workspace.toml の inline-table 行を upsert（cwd から自動探索）",

	// cocoon plugin scaffold
	"cmd_plugin_scaffold_cmd_short": "新しい <id>/ プラグインディレクトリを作成 " +
		"(既定: <workspace>/.cocoon/plugins、--plugins-dir で上書き)",
	"cmd_plugin_scaffold_cmd_long": `cocoon plugin scaffold — プロジェクトのプラグインオーバーレイ配下に新しい <id>/ ディレクトリを作成

既定では <workspace>/.cocoon/plugins/<id>/ 配下に新規ディレクトリが作られます
（最寄りの workspace.toml から自動探索）。--plugins-dir <path> で上書きすると、
パスはそのまま採用され <id> が結合されます。

新規ディレクトリには [install.methods.<category>] エントリを 1 つ宣言した
plugin.toml と、選択テンプレートに合った install.<category>.sh スケルトンが
含まれます。テンプレートは installer (curl|bash) / binary (単一バイナリ DL) /
apt (apt repo / .deb) / archive (複数ファイル展開) のいずれかです。
--with-install-user を付けるとさらに install_user.sh フック (プラグイン
スコープ・メソッド非依存) も生成されます。`,
	"flag_plugin_scaffold_cmd_plugins_dir_usage":       "出力先ディレクトリ（既定: workspace.toml から自動探索した <workspace>/.cocoon/plugins）",
	"flag_plugin_scaffold_cmd_name_usage":              `表示名（例: "GitHub CLI"）`,
	"flag_plugin_scaffold_cmd_description_usage":       "短い説明（URL は含めない）",
	"flag_plugin_scaffold_cmd_url_usage":               "上流の URL（例: https://github.com/owner/repo）",
	"flag_plugin_scaffold_cmd_default_usage":           "プラグインを既定で有効に",
	"flag_plugin_scaffold_cmd_requires_root_usage":     "install スクリプトを root で実行",
	"flag_plugin_scaffold_cmd_version_capable_usage":   "$PIN / $CHECKSUM_* の雛形を生成",
	"flag_plugin_scaffold_cmd_template_usage":          "install メソッドカテゴリ: installer | binary | apt | archive（カタログ正規名 — docs/plugins.md 参照）",
	"flag_plugin_scaffold_cmd_with_install_user_usage": "install_user.sh も生成",
	"flag_plugin_scaffold_cmd_non_interactive_usage":   "対話プロンプトをスキップ（上記フィールドの指定が必須）",
	"flag_plugin_scaffold_cmd_force_usage":             "<plugins-dir>/<id>/ が既に存在しても上書き",

	// cocoon self-update
	"cmd_self_update_short": "このバイナリを最新リリースで置き換える",
	"cmd_self_update_long": `cocoon self-update — 現在のバイナリを最新リリースで置き換える

sukekyo26/cocoon の GitHub Releases API に問い合わせ、リリースタグと
ビルドに焼き込まれたバージョン文字列を比較します。更新がある場合は
cocoon-<os>-<arch> アセットを SHA-256 検証付きでダウンロードし、
このバイナリをアトミックに置き換えます。

終了コード:
  0   既に最新、または置換成功
  100 (--check-only 時のみ) より新しいバージョンが存在
  1   その他の失敗`,
	"flag_self_update_check_only_usage": "最新なら exit 0、新しいリリースがあれば exit 100。ダウンロードはしない",
	"flag_self_update_force_usage":      "ローカルが最新でも再インストール",

	// cobra auto-generated `completion` subtree
	"cmd_completion_short": "指定したシェル向けの補完スクリプトを生成",
	"cmd_completion_long": `指定したシェル向けの cocoon 補完スクリプトを生成します。
生成スクリプトの使い方は各サブコマンドのヘルプを参照してください。
`,
	"cmd_completion_bash_short":       "bash 向けの補完スクリプトを生成",
	"cmd_completion_zsh_short":        "zsh 向けの補完スクリプトを生成",
	"cmd_completion_fish_short":       "fish 向けの補完スクリプトを生成",
	"cmd_completion_powershell_short": "powershell 向けの補完スクリプトを生成",

	// cobra auto-registered `help` subcommand
	"cmd_help_subcommand_short": "任意のコマンドのヘルプを表示",
	"cmd_help_subcommand_long": `アプリケーション内の任意のコマンドのヘルプを表示します。
詳細を見るには %s help [コマンドパス] と入力してください。`,
}
