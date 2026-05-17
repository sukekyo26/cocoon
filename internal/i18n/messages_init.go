//nolint:dupl // each catalog file shares the same boilerplate by design.
package i18n

func init() {
	register(LangEN, messagesEN_init)
	register(LangJA, messagesJA_init)
}

// messagesEN_init holds prompt / status strings emitted by `cocoon init`
// and `cocoon gen`. Keys are deliberately scoped with `init_` / `gen_`
// prefixes so they do not collide with the legacy `setup_*` table.
//
//nolint:gochecknoglobals,revive // catalog tables are file-scoped by design.
var messagesEN_init = map[string]string{
	// init prompts
	"init_prompt_service_name":          "Service name",
	"init_desc_service_name":            "Compose service name (e.g. \"my-api\").",
	"init_err_service_name_fmt":         "must start with a lowercase letter; only lowercase letters, digits, _ and - allowed",
	"init_prompt_username":              "Username",
	"init_desc_username":                "In-container user account (e.g. \"dev\").",
	"init_err_username_fmt":             "must start with a lowercase letter or _; only lowercase letters, digits, _ and - allowed",
	"init_err_required":                 "required — please enter a value",
	"init_prompt_image":                 "Base image",
	"init_desc_image":                   "Container base image (DockerHub canonical name). Pick ubuntu/debian for plain Linux, or a language-runtime image (node, python, golang, rust, denoland/deno) to skip an apt install step. Picking golang or rust disables the matching cocoon plugin to prevent double-install.",
	"init_prompt_image_version":         "%s version",
	"init_desc_image_version":           "Pulled as FROM %s:<version> in the generated Dockerfile.",
	"init_prompt_image_version_static":  "Image version",
	"init_desc_image_version_static":    "Type a tag, or press Tab to cycle through curated suggestions (e.g. 1.26.3-bookworm). Any well-formed tag the upstream registry publishes is accepted. Rules: first character is a letter, digit, or underscore; trailing characters add `.` and `-`; no slash, no colon.",
	"init_option_other_manual_input":    "Other (manual input)",
	"init_err_image_version_fmt":        "must be a plain Docker tag — first character alnum or underscore; trailing characters may add `.` and `-`; no slash or colon",
	"init_prompt_shell":                 "Login shell",
	"init_desc_shell":                   "Container login shell. bash is the cocoon default; zsh / fish drive shellrc generation differently.",
	"init_prompt_mount_root":            "Mount range",
	"init_desc_mount_root":              "How much of your filesystem should be visible inside the container?",
	"init_option_mount_cwd":             "Just this project (.)",
	"init_option_mount_parent":          "Parent directory — sibling repos visible (..)",
	"init_prompt_devcontainer":          "Generate .devcontainer/devcontainer.json for VS Code Dev Containers?",
	"init_desc_devcontainer":            "Says yes if you ever open this repo in VS Code Dev Containers; harmless otherwise.",
	"init_prompt_certificates":          "Bake user TLS certificates from ~/.cocoon/certs/ into the container image?",
	"init_desc_certificates":            "Enable when your build needs a corp CA inside the container (Zscaler, internal proxies, etc.). When off, no cert wiring lands in the generated artifacts.",
	"init_confirm_yes":                  "Yes",
	"init_confirm_no":                   "No",
	"init_prompt_apt":                   "Select common apt packages to install",
	"init_desc_apt":                     "Pre-checked categories are installed by default; uncheck what you do not need.",
	"init_prompt_plugins":               "Select plugins to enable",
	"init_desc_plugins":                 "Space toggles, Enter confirms. Pre-checked plugins are enabled by default.",
	"init_err_plugin_unknown_fmt":       "unknown plugin %q (run `cocoon plugin list` for the catalog)",
	"init_err_plugin_conflict_fmt":      "%s conflicts with %s — pick one",
	"init_err_plugin_load_fmt":          "load plugin catalog: %s",
	"init_prompt_plugin_version":        "%s version",
	"init_desc_plugin_version":          "LATEST = newest at build time. Pick Other to pin (e.g. 1.23.4). cocoon does NOT verify the pin exists — confirm via the URL below.",
	"init_err_plugin_pin_fmt":           "must be a plain version pin — first character alnum or underscore; trailing characters may add `.` and `-`; no slash or colon",
	"init_prompt_plugin_method":         "%s install method",
	"init_desc_plugin_method":           "This plugin declares more than one install method (e.g. official installer vs. direct binary). Pick the one that fits your environment — the choice is saved to [plugins.methods] in workspace.toml.",
	"init_prompt_alias_bundles":         "Select shell alias bundles",
	"init_desc_alias_bundles":           "Pre-canned alias sets merged into [container.shell].aliases. All start unchecked — opt in only what you want.",
	"init_err_alias_bundle_unknown_fmt": "unknown alias bundle %q",
	"init_prompt_ports":                 "Port forwards (comma-separated; leave blank to skip)",
	"init_desc_ports":                   "docker-compose short form: e.g. 3000 | 8000:8000 | 127.0.0.1:5432:5432/tcp | 3000-3005:3000-3005 | 6060:6060/udp. Press Enter with no input to defer (the commented [ports] template stays for later opt-in).",
	"init_err_port_invalid_fmt":         "%q is not a valid port short form (examples: 3000, 8000:8000, 127.0.0.1:8001:8001, 3000-3005:3000-3005, 6060:6060/udp; ports must be in [1, 65535])",
	// workspace.toml inline comments (rendered into the generated file)
	"init_toml_header": "# workspace.toml — cocoon configuration (generated by `cocoon init`)\n# Edit freely; re-run `cocoon gen` to regenerate .devcontainer/.\n",
	"init_toml_section_workspace": "# [workspace] — generation-wide knobs.\n" +
		"#   mount_root: how much of your filesystem to expose. \".\" = cwd only, \"..\" = parent (siblings visible).\n" +
		"#   devcontainer: emit .devcontainer/devcontainer.json for VS Code Reopen-in-Container.",
	"init_toml_section_container": "# [container] — image identity.\n" +
		"#   service_name: docker-compose `services:` <key>. Used by `docker compose exec <name>`.\n" +
		"#   username / image / image_version: in-container account and FROM <image>:<image_version>.\n" +
		"#   image candidates (DockerHub canonical names): ubuntu, debian, node, python, golang, rust, denoland/deno.",
	"init_toml_section_container_shell": "# [container.shell] — login shell + per-shell rc injection.\n" +
		"#   default: bash | zsh | fish. The generator picks ~/.bashrc / ~/.zshrc / ~/.config/fish/config.fish.\n" +
		"#   aliases / env: appended to the rc file inside the image at build time.\n" +
		"#   bash & zsh use POSIX syntax (alias k='v', export K=V); fish translation is automatic.\n" +
		"#\n" +
		"# env example (uncomment + edit):\n" +
		"#   env = { EDITOR = \"vim\", PAGER = \"less -R\" }\n" +
		"# Caveats:\n" +
		"#   - EDITOR=vim / nano needs the `text-editors` apt category enabled.\n" +
		"#   - EDITOR=code only works when launched from VS Code Dev Containers (the `code` shim is\n" +
		"#     injected by VS Code, not by cocoon).\n" +
		"#   - PAGER=less / less -R needs the `utilities` apt category (less is not in cocoon's minimal base).",
	"init_toml_section_plugins": "# [plugins] — enable cocoon plugins (run `cocoon plugin list` for the catalog).\n" +
		"#   Pin versions in [plugins.versions] when you need reproducible builds.",
	"init_toml_section_plugins_methods": "# [plugins.methods] — install method picked for plugins that declare multiple methods.\n" +
		"#   Plugins with a single declared method ignore this section.",
	"init_toml_section_plugins_versions": "# [plugins.versions] — pinned versions for the enabled plugins above.\n" +
		"#   checksum_amd64 / checksum_arm64 (64 lowercase hex chars) verify install tarballs — verify = \"checksum\" plugins only.",
	"init_toml_section_apt": "# [apt] — extra apt packages installed on top of cocoon's minimal base + selected categories.\n" +
		"#   Re-run `cocoon init --force` to change category checkboxes, or edit this list directly.",
	"init_toml_section_certificates": "# [certificates] — TLS certificate auto-bake from ~/.cocoon/certs/ on the host.\n" +
		"#   When enable = true the generators wire the host directory through to the build via\n" +
		"#   docker-compose's additional_contexts and the Dockerfile's RUN --mount=type=bind, so any\n" +
		"#   *.crt files placed there land in the container's trust store at build time. Default off.",
	"init_toml_section_ports": "# [ports] — host ports forwarded into the container.\n" +
		"#   Short form: [HOST_IP:][HOST:]CONTAINER[/PROTOCOL]. Ranges (3000-3005:3000-3005), UDP, and IPv6 [::1] binds are accepted.",
	// init result + next steps
	"init_wrote":             "wrote %s",
	"init_next_header":       "Next steps:",
	"init_next_step_gen":     "  1. cocoon gen",
	"init_next_step_compose": "  2. docker compose -f .devcontainer/docker-compose.yml up -d",
	"init_next_step_vscode":  `     (or open in VS Code → "Reopen in Container")`,
	// gen result + next steps
	"gen_wrote":               "wrote %s",
	"gen_next_header":         "To start the container:",
	"gen_next_step_compose":   "  docker compose -f .devcontainer/docker-compose.yml up -d",
	"gen_next_step_vscode":    `  (or open in VS Code → "Reopen in Container")`,
	"gen_next_step_manage":    "  ./.devcontainer/manage.sh -h    (clean up / rebuild this project's Docker resources)",
	"gen_certs_dir_created":   "created host directory %s (used as the cocoon_user_certs build context)",
	"gen_certs_notice_header": "Host TLS certificates:",
	"gen_certs_notice_path":   "  Drop *.crt files into ~/.cocoon/certs/ to bake corporate / private CAs into the container at build time.",
	"gen_certs_notice_team":   "  Team members who skip VS Code Dev Containers must run `mkdir -p ~/.cocoon/certs` once on their host (VS Code users get this auto-created via initializeCommand).",
	// gen home_files: host-side touch + notices
	"gen_home_file_touched":                 "created %s (empty, 0600) for [home_files] bind",
	"gen_home_files_notice_header":          "Host files for [home_files]:",
	"gen_home_files_notice_check":           "  Verify these files exist on the host before running `docker compose up`:",
	"gen_home_files_notice_item":            "    ~/%s",
	"gen_home_files_in_container_warning":   "WARNING: cocoon gen is running inside a container (/.dockerenv detected); [home_files] entries will be touched in this container's HOME, not the Docker host's. Run `cocoon gen` on the host before `docker compose up`.",
	"gen_home_files_is_directory":           "%s exists as a directory (likely auto-created by a previous `docker compose up` when the file was missing); remove it with `rm -rf %s` and re-run `cocoon gen`",
	"gen_docker_cli_without_socket_warning": "WARNING: the docker-cli plugin is enabled but [container].docker_socket is not enabled (unset or false); the in-container docker client has no daemon socket to reach, so `docker ...` will fail with \"cannot connect to the Docker daemon\". Add `docker_socket = true` under [container] in workspace.toml, or remove the docker-cli plugin. Ignore this if the container talks to a remote DOCKER_HOST.",
}

// messagesJA_init mirrors messagesEN_init in Japanese. Untranslated keys
// (typically command-name examples like `cocoon gen`) intentionally keep
// the English form because the command name itself is not localized.
//
//nolint:gochecknoglobals,revive // catalog tables are file-scoped by design.
var messagesJA_init = map[string]string{
	// init prompts
	"init_prompt_service_name":          "サービス名",
	"init_desc_service_name":            "Compose のサービス名 (例: \"my-api\")。",
	"init_err_service_name_fmt":         "英小文字で始め、英小文字・数字・_・- のみ使用可です",
	"init_prompt_username":              "ユーザー名",
	"init_desc_username":                "コンテナ内ユーザーのアカウント名 (例: \"dev\")。",
	"init_err_username_fmt":             "英小文字または _ で始め、英小文字・数字・_・- のみ使用可です",
	"init_err_required":                 "必須項目です。値を入力してください",
	"init_prompt_image":                 "ベースイメージ",
	"init_desc_image":                   "コンテナのベースイメージ (DockerHub の正式名称)。Linux のみなら ubuntu/debian を、言語ランタイム入りなら node / python / golang / rust / denoland/deno を選ぶと apt 1 ステップ省ける。golang / rust を選んだ場合は同名の cocoon プラグインが無効化される（二重インストール回避）。",
	"init_prompt_image_version":         "%s のバージョン",
	"init_desc_image_version":           "生成される Dockerfile の FROM %s:<version> に展開されます。",
	"init_prompt_image_version_static":  "イメージのバージョン",
	"init_option_other_manual_input":    "その他 (手動入力)",
	"init_err_image_version_fmt":        "Docker タグの形式である必要があります (先頭は英数字または `_`、2 文字目以降は `.` / `-` も可、スラッシュ・コロン禁止)",
	"init_desc_image_version_static":    "タグを入力するか、Tab キーで推奨候補を循環できます (例: 1.26.3-bookworm)。上流レジストリが公開している正しい形式のタグなら自由に入力可。形式: 先頭は英数字または `_`、2 文字目以降は `.` / `-` も可、スラッシュ・コロン禁止。",
	"init_prompt_shell":                 "ログインシェル",
	"init_desc_shell":                   "コンテナ内のログインシェル。bash が cocoon のデフォルト。zsh / fish は shellrc 生成が分岐します。",
	"init_prompt_mount_root":            "マウント範囲",
	"init_desc_mount_root":              "コンテナ内に見せるファイルシステムの範囲を選んでください。",
	"init_option_mount_cwd":             "このプロジェクトのみ (.)",
	"init_option_mount_parent":          "親ディレクトリ — 兄弟リポジトリも見える (..)",
	"init_prompt_devcontainer":          ".devcontainer/devcontainer.json を VS Code Dev Containers 用に生成しますか？",
	"init_desc_devcontainer":            "VS Code Dev Containers で開く可能性があれば Yes。そうでなくても害はありません。",
	"init_prompt_certificates":          "~/.cocoon/certs/ の TLS 証明書をコンテナイメージに取り込みますか？",
	"init_desc_certificates":            "Zscaler や社内プロキシなど、build 中に corp CA を信頼させる必要がある場合に Yes。off の場合は生成物に cert 関連の配線は一切乗りません。",
	"init_confirm_yes":                  "はい",
	"init_confirm_no":                   "いいえ",
	"init_prompt_apt":                   "インストールする apt パッケージのカテゴリを選択",
	"init_desc_apt":                     "プリチェック済みのカテゴリがデフォルトでインストールされます。不要なものはチェックを外してください。",
	"init_prompt_plugins":               "有効化するプラグインを選択",
	"init_desc_plugins":                 "スペースでトグル、Enter で確定します。プリチェック済みのプラグインがデフォルトで有効になります。",
	"init_err_plugin_unknown_fmt":       "未知のプラグイン %q (`cocoon plugin list` で一覧を確認してください)",
	"init_err_plugin_conflict_fmt":      "%s と %s は併用できません — どちらか一方を選んでください",
	"init_err_plugin_load_fmt":          "プラグインカタログの読み込みに失敗: %s",
	"init_prompt_plugin_version":        "%s のバージョン",
	"init_desc_plugin_version":          "LATEST はビルド時に最新版を取得。固定するなら「その他 (手動入力)」で入力 (例: 1.23.4)。実在検証はしないので下記 URL で確認を。",
	"init_err_plugin_pin_fmt":           "バージョン pin 形式である必要があります (先頭は英数字または `_`、2 文字目以降は `.` / `-` も可、スラッシュ・コロン禁止)",
	"init_prompt_plugin_method":         "%s のインストール方式",
	"init_desc_plugin_method":           "このプラグインは複数のインストール方式（例: 公式インストーラ / バイナリ直接ダウンロード）を提供しています。環境に合うものを 1 つ選択してください — 選択は workspace.toml の [plugins.methods] に保存されます。",
	"init_prompt_alias_bundles":         "シェルエイリアスバンドルを選択",
	"init_desc_alias_bundles":           "プリセットの alias セットを [container.shell].aliases にマージします。初期チェックは全部 OFF — 欲しいものだけ選んでください。",
	"init_err_alias_bundle_unknown_fmt": "未知のエイリアスバンドル %q",
	"init_prompt_ports":                 "ポートフォワード設定 (カンマ区切り、空 Enter でスキップ)",
	"init_desc_ports":                   "docker-compose の short form: 例 3000 | 8000:8000 | 127.0.0.1:5432:5432/tcp | 3000-3005:3000-3005 | 6060:6060/udp。何も入力せず Enter で見送り（後から有効化できるよう [ports] のコメント雛形は残ります）。",
	"init_err_port_invalid_fmt":         "%q はポート指定として無効です (例: 3000, 8000:8000, 127.0.0.1:8001:8001, 3000-3005:3000-3005, 6060:6060/udp。ポート番号は [1, 65535] の範囲)",
	// workspace.toml inline comments (rendered into the generated file)
	"init_toml_header": "# workspace.toml — cocoon 設定 (cocoon init で生成)\n# 自由に編集してください。cocoon gen で .devcontainer/ を再生成します。\n",
	"init_toml_section_workspace": "# [workspace] — 生成全体の挙動。\n" +
		"#   mount_root: コンテナへ見せるホスト範囲。\".\" = cwd のみ、\"..\" = 親ディレクトリ（兄弟リポも見える）。\n" +
		"#   devcontainer: VS Code Reopen-in-Container 用の devcontainer.json を生成するか。",
	"init_toml_section_container": "# [container] — イメージの素性。\n" +
		"#   service_name: docker-compose の `services:` <キー>。`docker compose exec <名前>` で使う。\n" +
		"#   username / image / image_version: コンテナ内ユーザー名と FROM <image>:<image_version>。\n" +
		"#   image 候補 (DockerHub 正式名称): ubuntu, debian, node, python, golang, rust, denoland/deno。",
	"init_toml_section_container_shell": "# [container.shell] — ログインシェル + シェル別 rc 注入。\n" +
		"#   default: bash | zsh | fish。生成系が ~/.bashrc / ~/.zshrc / ~/.config/fish/config.fish を選ぶ。\n" +
		"#   aliases / env: イメージビルド時に rc ファイルへ追記される。\n" +
		"#   bash と zsh は POSIX 記法 (alias k='v', export K=V)、fish は自動翻訳。\n" +
		"#\n" +
		"# env の設定例（コメントアウトを外して編集）:\n" +
		"#   env = { EDITOR = \"vim\", PAGER = \"less -R\" }\n" +
		"# 注意:\n" +
		"#   - EDITOR=vim / nano は apt カテゴリ `text-editors` の有効化が前提。\n" +
		"#   - EDITOR=code は VS Code Dev Containers で開いたときのみ動く（`code` シムは\n" +
		"#     VS Code が注入するもので、cocoon は関与しない）。\n" +
		"#   - PAGER=less / less -R は apt カテゴリ `utilities` が前提（less は cocoon の最小ベースに含まれない）。",
	"init_toml_section_plugins": "# [plugins] — cocoon プラグインの有効化（一覧は `cocoon plugin list`）。\n" +
		"#   再現性が必要なら [plugins.versions] でバージョン固定。",
	"init_toml_section_plugins_methods": "# [plugins.methods] — 複数のインストール方式を提供するプラグインに対する選択。\n" +
		"#   方式を 1 つしか持たないプラグインはこのセクションを無視。",
	"init_toml_section_plugins_versions": "# [plugins.versions] — 上で有効化したプラグインに対するバージョン固定。\n" +
		"#   verify = \"checksum\" のプラグインは checksum_amd64 / checksum_arm64（64 文字小文字 hex）で install tarball を検証できる。",
	"init_toml_section_certificates": "# [certificates] — ホスト側 ~/.cocoon/certs/ の TLS 証明書をコンテナイメージに自動取り込み (opt-in)。\n" +
		"#   enable = true のときジェネレータが docker-compose の additional_contexts と Dockerfile の\n" +
		"#   RUN --mount=type=bind を配線し、ホスト側 *.crt が build 時にトラストストアへマージされる。\n" +
		"#   デフォルト off。",
	"init_toml_section_apt": "# [apt] — cocoon の最小ベース + 選択カテゴリに追加する apt パッケージ。\n" +
		"#   カテゴリのチェックを変えるなら `cocoon init --force` を再実行、または直接このリストを編集。",
	"init_toml_section_ports": "# [ports] — コンテナへフォワードするホストポート。\n" +
		"#   short form: [HOST_IP:][HOST:]CONTAINER[/PROTOCOL]。範囲 (3000-3005:3000-3005)、UDP、IPv6 [::1] バインドも可。",
	// init result + next steps
	"init_wrote":             "%s を書き出しました",
	"init_next_header":       "次のステップ:",
	"init_next_step_gen":     "  1. cocoon gen",
	"init_next_step_compose": "  2. docker compose -f .devcontainer/docker-compose.yml up -d",
	"init_next_step_vscode":  `     (または VS Code で「Reopen in Container」を実行)`,
	// gen result + next steps
	"gen_wrote":               "%s を書き出しました",
	"gen_next_header":         "コンテナを起動するには:",
	"gen_next_step_compose":   "  docker compose -f .devcontainer/docker-compose.yml up -d",
	"gen_next_step_vscode":    `  (または VS Code で「Reopen in Container」を実行)`,
	"gen_next_step_manage":    "  ./.devcontainer/manage.sh -h    (このプロジェクトの Docker リソースの掃除 / リビルド)",
	"gen_certs_dir_created":   "ホスト側ディレクトリ %s を作成しました (cocoon_user_certs ビルドコンテキストとして使用)",
	"gen_certs_notice_header": "ホスト TLS 証明書:",
	"gen_certs_notice_path":   "  社内 / プライベート CA を取り込む場合は `.crt` を `~/.cocoon/certs/` に置いてください。build 時に自動取り込みされます。",
	"gen_certs_notice_team":   "  VS Code Dev Containers を使わないチームメンバーは初回のみホストで `mkdir -p ~/.cocoon/certs` の実行が必要です (VS Code 経由は initializeCommand で自動)。",
	// gen home_files: host-side touch + notices
	"gen_home_file_touched":                 "[home_files] バインド用に %s を作成しました (空ファイル, 0600)",
	"gen_home_files_notice_header":          "[home_files] のホスト側ファイル:",
	"gen_home_files_notice_check":           "  `docker compose up` を実行する前に、ホストで次のファイルが存在することを確認してください:",
	"gen_home_files_notice_item":            "    ~/%s",
	"gen_home_files_in_container_warning":   "警告: cocoon gen がコンテナ内 (/.dockerenv を検出) で実行されています。[home_files] エントリは Docker ホストではなくこのコンテナの HOME に対して touch されます。`docker compose up` の前にホスト側で `cocoon gen` を実行してください。",
	"gen_home_files_is_directory":           "%s がディレクトリとして存在します (以前の `docker compose up` がファイル不在時に自動作成した可能性)。`rm -rf %s` で削除してから `cocoon gen` を再実行してください",
	"gen_docker_cli_without_socket_warning": "警告: docker-cli プラグインは有効ですが [container].docker_socket が有効化されていません (未設定または false)。コンテナ内の docker クライアントが接続できる daemon ソケットが無いため、`docker ...` は \"cannot connect to the Docker daemon\" で失敗します。workspace.toml の [container] に `docker_socket = true` を追加するか、docker-cli プラグインを外してください。リモートの DOCKER_HOST に接続する構成ならこの警告は無視して構いません。",
}
