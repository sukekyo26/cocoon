//nolint:dupl // each catalog file shares the same boilerplate by design.
package i18n

func init() {
	register(LangEN, messagesEN_initTemplates)
	register(LangJA, messagesJA_initTemplates)
}

// messagesEN_initTemplates holds the commented-out section templates that
// `cocoon init` inserts into the generated cocoon.toml so users can
// discover opt-in features without leaving the file. Each value is the
// raw `# ...` block (no trailing newline — the renderer appends `\n\n`).
//
//nolint:gochecknoglobals,revive // catalog tables are file-scoped by design.
var messagesEN_initTemplates = map[string]string{
	// [container.*] extras (rendered immediately after [container]).
	"init_toml_template_container_resources": "# [container.resources] — override compose resource defaults (Docker uses unset = unlimited).\n" +
		"#   Tune shm_size, pids_limit, cpus, memory.\n" +
		"# [container.resources]\n" +
		"# shm_size = \"2gb\"\n" +
		"# pids_limit = 2048",
	"init_toml_template_container_hosts": "# [container.hosts] — extra /etc/hosts entries.\n" +
		"#   Map hostnames to IP addresses or \"host-gateway\" (= the host).\n" +
		"# [container.hosts]\n" +
		"# \"db.local\"     = \"host-gateway\"\n" +
		"# \"corp.example\" = \"10.0.0.42\"",
	"init_toml_template_container_dns": "# [container.dns] — custom DNS resolvers and search domains.\n" +
		"#   Useful for corporate DNS or auto-suffixing short hostnames.\n" +
		"# [container.dns]\n" +
		"# servers = [\"10.0.0.53\", \"1.1.1.1\"]\n" +
		"# search  = [\"corp.example.com\"]",
	"init_toml_template_container_sysctls": "# [container.sysctls] — kernel parameters (e.g. for Elasticsearch).\n" +
		"#   Values can be int or string; both are forwarded to docker compose.\n" +
		"# [container.sysctls]\n" +
		"# \"vm.max_map_count\" = 262144",
	"init_toml_template_container_capabilities": "# [container.capabilities] — Linux capabilities to add or drop.\n" +
		"#   SYS_PTRACE is needed by debuggers (delve, gdb, lldb).\n" +
		"# [container.capabilities]\n" +
		"# add  = [\"SYS_PTRACE\"]\n" +
		"# drop = [\"AUDIT_WRITE\"]",
	"init_toml_template_container_security_opt": "# [container.security_opt] — Compose security_opt: seccomp / apparmor / no_new_privileges.\n" +
		"#   \"unconfined\" relaxes sandboxing; no_new_privileges blocks setuid escalation.\n" +
		"#   Note: no_new_privileges = true also disables sudo (the image grants the user passwordless sudo).\n" +
		"# [container.security_opt]\n" +
		"# seccomp           = \"unconfined\"\n" +
		"# no_new_privileges = true",
	"init_toml_template_container_sudo": "# [container.sudo] — sudo password policy (alternative to passwordless sudo / no_new_privileges).\n" +
		"#   mode = \"password\" requires a password for sudo, read at build time from\n" +
		"#   .devcontainer/.env.local (SUDO_PASSWORD=...) via a Docker build secret (gitignored, never\n" +
		"#   committed). A missing/empty SUDO_PASSWORD fails the build. Mutually exclusive with\n" +
		"#   no_new_privileges above. Omit for passwordless sudo (the default).\n" +
		"# [container.sudo]\n" +
		"# mode = \"password\"",
	"init_toml_template_container_skel": "# [[container.skel]] — seed dotfiles into the new user's home (via /etc/skel).\n" +
		"#   source: relative to the workspace root (build context); target: relative to /etc/skel.\n" +
		"# [[container.skel]]\n" +
		"# source = \".cocoon/skel/example.bashrc\"\n" +
		"# target = \".bashrc\"",

	// [container] flat-field extras (rendered right after the [container]
	// keys, before any [container.*] subtable so an uncommented line lands
	// in [container]).
	"init_toml_template_container_docker_socket": "# docker_socket — bind-mount /var/run/docker.sock so an in-container docker\n" +
		"#   client (e.g. the docker-cli plugin) can reach the host daemon.\n" +
		"# docker_socket = true",
	"init_toml_template_container_group_add": "# group_add — extra supplementary groups (name or numeric GID) the container user joins.\n" +
		"#   docker_socket = true additionally appends the host docker group.\n" +
		"# group_add = [\"audio\", \"dialout\"]",
	"init_toml_template_container_devices": "# devices — map host devices into the container (HOST:CONTAINER[:rwm]).\n" +
		"# devices = [\"/dev/dri:/dev/dri\"]",
	"init_toml_template_container_ipc": "# ipc — IPC namespace mode; \"host\" exposes a large shared-memory segment (ML workloads).\n" +
		"# ipc = \"host\"",
	"init_toml_template_container_gpus": "# gpus — request GPU access. Only \"all\" is currently supported.\n" +
		"# gpus = \"all\"",

	// [plugins.methods] (rendered immediately after [plugins], before
	// [plugins.options] because picking a method may change the upstream
	// URL used to pick a version).
	"init_toml_template_plugins_methods": "# [plugins.methods] — for plugins that declare multiple [install.methods],\n" +
		"#   pick which install path to use. Plugins with a single declared method\n" +
		"#   ignore this section.\n" +
		"# [plugins.methods]\n" +
		"# <plugin-id> = \"<method-name>\"",

	// [plugins.options] (rendered immediately after [plugins.methods]).
	"init_toml_template_plugins_options": "# [plugins.options] — extra per-plugin knobs for the enabled plugins above.\n" +
		"#   Pin versions inline in the enable array (e.g. \"go=1.22.5\", \"starship=latest\");\n" +
		"#   run `cocoon lock` to freeze \"latest\" and record checksums in cocoon.lock.\n" +
		"#   Use this table only for [install.extra_versions] knobs (e.g. android-sdk's\n" +
		"#   api_level / build_tools).\n" +
		"# [plugins.options]\n" +
		"# android-sdk = { api_level = \"35\", build_tools = \"35.0.0\" }",

	// [apt.*] extras (rendered immediately after [apt]).
	"init_toml_template_apt_mirror": "# [apt.mirror] — rewrite upstream apt URLs to a regional mirror.\n" +
		"#   Often a multi-fold first-build speedup. Match your [container].os family.\n" +
		"# [apt.mirror]\n" +
		"# url = \"http://jp.archive.ubuntu.com/ubuntu/\"",
	"init_toml_template_apt_proxy": "# [apt.proxy] — corporate HTTP/HTTPS proxy for apt-get.\n" +
		"#   Written to /etc/apt/apt.conf.d/95proxy at image build time.\n" +
		"# [apt.proxy]\n" +
		"# http  = \"http://proxy.corp:8080\"\n" +
		"# https = \"http://proxy.corp:8080\"",
	"init_toml_template_apt_sources": "# [[apt.sources]] — third-party apt repositories with signed-by GPG keys.\n" +
		"#   The key is fetched from key_url and dearmored under /etc/apt/keyrings.\n" +
		"# [[apt.sources]]\n" +
		"# name       = \"fish-stable\"\n" +
		"# suite      = \"noble\"\n" +
		"# components = [\"main\"]\n" +
		"# url        = \"https://download.opensuse.org/repositories/shells:fish:release:3/xUbuntu_24.04/\"\n" +
		"# key_url    = \"https://download.opensuse.org/repositories/shells:fish:release:3/xUbuntu_24.04/Release.key\"",

	// Top-level extras (rendered at the end of the file).
	"init_toml_template_ports": "# [ports] — host ports to forward into the container.\n" +
		"#   Short form (\"3000:3000\") or long-form table; ranges and protocol/mode supported.\n" +
		"# [ports]\n" +
		"# forward = [\"3000:3000\", \"5432:5432\"]",
	"init_toml_template_volumes": "# [volumes] — extra named volumes mapped under the container's home.\n" +
		"#   Persists state across container recreations. Format: <volume-name> = <path inside container>.\n" +
		"# [volumes]\n" +
		"# my-data = \"/home/${USERNAME}/.my-tool\"",
	"init_toml_template_env": "# [env] — environment variables passed into the container.\n" +
		"#   ${VAR} resolves against the host's env at `cocoon gen` time.\n" +
		"# [env]\n" +
		"# OPENAI_API_KEY = \"${OPENAI_API_KEY}\"\n" +
		"# DEBUG          = \"1\"",
	"init_toml_template_mounts": "# [[mounts]] — extra bind mounts from the host into the container.\n" +
		"#   readonly = true is safer when sharing credentials like SSH keys.\n" +
		"# [[mounts]]\n" +
		"# source   = \"~/.ssh\"\n" +
		"# target   = \"/home/${USERNAME}/.ssh\"\n" +
		"# readonly = true",
	"init_toml_template_home_files": "# [home_files] — files persisted via per-file bind mounts.\n" +
		"#   Each path is relative to ~. Bind-mount the host's ~/.gitconfig to share git identity.\n" +
		"# [home_files]\n" +
		"# files = [\".gitconfig\", \".claude.json\"]",
	"init_toml_template_locale": "# [locale] — container timezone and language.\n" +
		"#   timezone defaults to the host's; lang must be a UTF-8 locale (e.g. \"ja_JP.UTF-8\").\n" +
		"# [locale]\n" +
		"# timezone = \"Asia/Tokyo\"\n" +
		"# lang     = \"ja_JP.UTF-8\"",
	"init_toml_template_certificates": "# [certificates] — opt in to TLS certificate auto-bake from ~/.cocoon/certs/.\n" +
		"#   When enable = true, `cocoon gen` wires the host directory into the build via\n" +
		"#   docker-compose's additional_contexts and the Dockerfile's RUN --mount=type=bind,\n" +
		"#   so any *.crt / *.cer files placed there land in the container's trust store at build time.\n" +
		"#   Useful when the build must trust a private or self-signed CA. Default off → no cert wiring\n" +
		"#   in any generated artifact.\n" +
		"# [certificates]\n" +
		"# enable = true",
	"init_toml_template_dockerfile": "# [dockerfile] — inject custom Dockerfile fragments at well-defined hook points.\n" +
		"#   pre_user_setup runs before useradd; post_plugins runs after plugin install.<category>.sh's.\n" +
		"# [dockerfile]\n" +
		"# pre_user_setup = \"\"\"RUN apt-get install -y my-extra-pkg\"\"\"\n" +
		"# post_plugins   = \"\"\"RUN echo done\"\"\"",
	"init_toml_template_services": "# [services.<name>] — sidecar services on the same Compose network (e.g. postgres, redis).\n" +
		"#   Reachable from the dev container by service name. Multiple sidecars allowed.\n" +
		"# [services.postgres]\n" +
		"# image       = \"postgres:16-alpine\"\n" +
		"# environment = { POSTGRES_PASSWORD = \"dev\" }\n" +
		"# ports       = [\"5432:5432\"]",
	"init_toml_template_devcontainer": "# [devcontainer.*] — VS Code Dev Container customizations merged into devcontainer.json.\n" +
		"#   Ignored when [workspace] devcontainer = false. Common: VS Code extensions list.\n" +
		"# [devcontainer.customizations.vscode]\n" +
		"# extensions = [\n" +
		"#     \"ms-azuretools.vscode-docker\",\n" +
		"#     \"eamodio.gitlens\",\n" +
		"# ]",
	"init_toml_template_code_workspace": "# [code_workspace] — VS Code .code-workspace file generated by `cocoon gen workspace`.\n" +
		"#   Opt-in: only emitted when you run the subcommand. Output goes to the project root\n" +
		"#   (next to cocoon.toml, not under .devcontainer/), so `code <name>.code-workspace`\n" +
		"#   opens it directly. folders[].path supports \"~\" expansion and is relativized\n" +
		"#   against the directory the .code-workspace file is written to (default: cocoon.toml\n" +
		"#   directory; overridable with `cocoon gen workspace --output <dir>`), so \"~/.claude\"\n" +
		"#   resolves to a relative path that VS Code can traverse upward.\n" +
		"# [code_workspace]\n" +
		"# name = \"my-stack\"                                       # output file basename (default: project dir basename)\n" +
		"# folders = [\n" +
		"#     { path = \".\" },                                     # project itself\n" +
		"#     { path = \"~/.claude\" },                             # home subdirectory; name auto-derived\n" +
		"#     { path = \"../sibling-repo\" },                       # sibling project\n" +
		"#     { path = \"~/.config/nvim\", name = \"Neovim\" },     # explicit display name override\n" +
		"# ]\n" +
		"# [code_workspace.settings]\n" +
		"# \"editor.tabSize\" = 2\n" +
		"# [code_workspace.extensions]\n" +
		"# recommendations = [\"golang.go\", \"ms-azuretools.vscode-docker\"]",
}

// messagesJA_initTemplates mirrors messagesEN_initTemplates in Japanese.
// Section identifiers themselves stay English (they are the TOML schema)
// — only the surrounding prose is localized.
//
//nolint:gochecknoglobals,revive // catalog tables are file-scoped by design.
var messagesJA_initTemplates = map[string]string{
	// [container.*] extras
	"init_toml_template_container_resources": "# [container.resources] — Compose のリソース上限を上書き (未設定 = 無制限)。\n" +
		"#   shm_size / pids_limit / cpus / memory を指定。\n" +
		"# [container.resources]\n" +
		"# shm_size = \"2gb\"\n" +
		"# pids_limit = 2048",
	"init_toml_template_container_hosts": "# [container.hosts] — /etc/hosts 追加エントリ。\n" +
		"#   ホスト名 → IP アドレス or \"host-gateway\" (= ホスト機) のマップ。\n" +
		"# [container.hosts]\n" +
		"# \"db.local\"     = \"host-gateway\"\n" +
		"# \"corp.example\" = \"10.0.0.42\"",
	"init_toml_template_container_dns": "# [container.dns] — カスタム DNS リゾルバと検索ドメイン。\n" +
		"#   社内 DNS や短名の自動補完に。\n" +
		"# [container.dns]\n" +
		"# servers = [\"10.0.0.53\", \"1.1.1.1\"]\n" +
		"# search  = [\"corp.example.com\"]",
	"init_toml_template_container_sysctls": "# [container.sysctls] — カーネルパラメータ (例: Elasticsearch 向け)。\n" +
		"#   値は int か string、どちらも docker compose に渡される。\n" +
		"# [container.sysctls]\n" +
		"# \"vm.max_map_count\" = 262144",
	"init_toml_template_container_capabilities": "# [container.capabilities] — 追加 / 剥奪する Linux capabilities。\n" +
		"#   SYS_PTRACE はデバッガ (delve / gdb / lldb) で必要。\n" +
		"# [container.capabilities]\n" +
		"# add  = [\"SYS_PTRACE\"]\n" +
		"# drop = [\"AUDIT_WRITE\"]",
	"init_toml_template_container_security_opt": "# [container.security_opt] — Compose の security_opt (seccomp / apparmor / no_new_privileges)。\n" +
		"#   \"unconfined\" でサンドボックス緩和、no_new_privileges で setuid 権限昇格を遮断。\n" +
		"#   注意: no_new_privileges = true は sudo も無効化する (イメージはユーザーに passwordless sudo を付与)。\n" +
		"# [container.security_opt]\n" +
		"# seccomp           = \"unconfined\"\n" +
		"# no_new_privileges = true",
	"init_toml_template_container_sudo": "# [container.sudo] — sudo のパスワード方針 (passwordless sudo / no_new_privileges の代替)。\n" +
		"#   mode = \"password\" で sudo にパスワードを要求。値はビルド時に .devcontainer/.env.local の\n" +
		"#   SUDO_PASSWORD=... から Docker build secret として読む (gitignore 対象・非コミット)。\n" +
		"#   SUDO_PASSWORD が未設定/空ならビルド失敗。上の no_new_privileges とは排他。\n" +
		"#   省略で passwordless sudo (既定)。\n" +
		"# [container.sudo]\n" +
		"# mode = \"password\"",
	"init_toml_template_container_skel": "# [[container.skel]] — 新規ユーザーのホームに dotfiles を配置 (/etc/skel 経由)。\n" +
		"#   source はワークスペースルート相対 (build context)、target は /etc/skel 相対。\n" +
		"# [[container.skel]]\n" +
		"# source = \".cocoon/skel/example.bashrc\"\n" +
		"# target = \".bashrc\"",

	// [container] 直下のフラットフィールド (＝[container] キーの直後、
	// [container.*] サブテーブルより前に出力。コメントを外した行が
	// [container] に入るようにするため)。
	"init_toml_template_container_docker_socket": "# docker_socket — /var/run/docker.sock をバインドマウントし、コンテナ内の\n" +
		"#   docker クライアント (例: docker-cli プラグイン) がホスト daemon に到達できるようにする。\n" +
		"# docker_socket = true",
	"init_toml_template_container_group_add": "# group_add — コンテナユーザーが参加する補助グループ (グループ名 または 数値 GID)。\n" +
		"#   docker_socket = true のときはホストの docker グループも自動で追加される。\n" +
		"# group_add = [\"audio\", \"dialout\"]",
	"init_toml_template_container_devices": "# devices — ホストのデバイスをコンテナにマップ (HOST:CONTAINER[:rwm])。\n" +
		"# devices = [\"/dev/dri:/dev/dri\"]",
	"init_toml_template_container_ipc": "# ipc — IPC 名前空間モード。\"host\" は大きな共有メモリセグメントを与える (ML 用途)。\n" +
		"# ipc = \"host\"",
	"init_toml_template_container_gpus": "# gpus — GPU アクセスを要求。現状 \"all\" のみサポート。\n" +
		"# gpus = \"all\"",

	// [plugins.methods]（[plugins] の直後、[plugins.options] の前に出力。
	// method の切替で上流 URL が変わる場合があるため、version より先に置く）。
	"init_toml_template_plugins_methods": "# [plugins.methods] — 複数の [install.methods] を持つプラグインで、どの方式を使うか指定。\n" +
		"#   方式を 1 つしか持たないプラグインはこのセクションを無視。\n" +
		"# [plugins.methods]\n" +
		"# <plugin-id> = \"<method-name>\"",

	// [plugins.options]
	"init_toml_template_plugins_options": "# [plugins.options] — 上で有効化したプラグインの追加設定。\n" +
		"#   バージョンは enable 配列にインラインで指定（例 \"go=1.22.5\", \"starship=latest\"）。\n" +
		"#   `cocoon lock` で \"latest\" を凍結し checksum を cocoon.lock に記録。\n" +
		"#   このテーブルは [install.extra_versions] の項目（例 android-sdk の\n" +
		"#   api_level / build_tools）専用。\n" +
		"# [plugins.options]\n" +
		"# android-sdk = { api_level = \"35\", build_tools = \"35.0.0\" }",

	// [apt.*] extras
	"init_toml_template_apt_mirror": "# [apt.mirror] — 上流 apt URL を地域ミラーに書き換え。\n" +
		"#   初回ビルドが大幅高速化。[container].os ファミリと合わせる。\n" +
		"# [apt.mirror]\n" +
		"# url = \"http://jp.archive.ubuntu.com/ubuntu/\"",
	"init_toml_template_apt_proxy": "# [apt.proxy] — apt-get の HTTP/HTTPS プロキシ。\n" +
		"#   イメージビルド時に /etc/apt/apt.conf.d/95proxy へ書かれる。\n" +
		"# [apt.proxy]\n" +
		"# http  = \"http://proxy.corp:8080\"\n" +
		"# https = \"http://proxy.corp:8080\"",
	"init_toml_template_apt_sources": "# [[apt.sources]] — 第三者 apt リポジトリ (signed-by GPG キー方式)。\n" +
		"#   key_url からキーを取得し /etc/apt/keyrings/ に dearmor して配置。\n" +
		"# [[apt.sources]]\n" +
		"# name       = \"fish-stable\"\n" +
		"# suite      = \"noble\"\n" +
		"# components = [\"main\"]\n" +
		"# url        = \"https://download.opensuse.org/repositories/shells:fish:release:3/xUbuntu_24.04/\"\n" +
		"# key_url    = \"https://download.opensuse.org/repositories/shells:fish:release:3/xUbuntu_24.04/Release.key\"",

	// Top-level extras
	"init_toml_template_ports": "# [ports] — コンテナへ転送するホストポート。\n" +
		"#   short (\"3000:3000\") か long-form テーブル、範囲やプロトコル/モードも可。\n" +
		"# [ports]\n" +
		"# forward = [\"3000:3000\", \"5432:5432\"]",
	"init_toml_template_volumes": "# [volumes] — コンテナホーム配下に追加でマウントする named volume。\n" +
		"#   コンテナ再作成をまたいで状態保持。書式: <ボリューム名> = <コンテナ内パス>。\n" +
		"# [volumes]\n" +
		"# my-data = \"/home/${USERNAME}/.my-tool\"",
	"init_toml_template_env": "# [env] — コンテナへ渡す環境変数。\n" +
		"#   ${VAR} は `cocoon gen` 実行時にホストの環境変数を解決。\n" +
		"# [env]\n" +
		"# OPENAI_API_KEY = \"${OPENAI_API_KEY}\"\n" +
		"# DEBUG          = \"1\"",
	"init_toml_template_mounts": "# [[mounts]] — ホストからコンテナへの追加バインドマウント。\n" +
		"#   SSH 鍵等の共有では readonly = true が安全。\n" +
		"# [[mounts]]\n" +
		"# source   = \"~/.ssh\"\n" +
		"# target   = \"/home/${USERNAME}/.ssh\"\n" +
		"# readonly = true",
	"init_toml_template_home_files": "# [home_files] — 単一ファイル bind mount で永続化。\n" +
		"#   各パスは ~ 相対。ホストの ~/.gitconfig を bind して git identity を共有。\n" +
		"# [home_files]\n" +
		"# files = [\".gitconfig\", \".claude.json\"]",
	"init_toml_template_locale": "# [locale] — コンテナのタイムゾーンと言語。\n" +
		"#   timezone のデフォルトはホスト準拠、lang は UTF-8 ロケール (例: \"ja_JP.UTF-8\")。\n" +
		"# [locale]\n" +
		"# timezone = \"Asia/Tokyo\"\n" +
		"# lang     = \"ja_JP.UTF-8\"",
	"init_toml_template_certificates": "# [certificates] — ~/.cocoon/certs/ からコンテナイメージへ TLS 証明書を自動取り込み (opt-in)。\n" +
		"#   enable = true のとき `cocoon gen` が docker-compose の additional_contexts と Dockerfile の\n" +
		"#   RUN --mount=type=bind を配線し、ホスト側ディレクトリの *.crt / *.cer がビルド時にトラストストアへ\n" +
		"#   マージされる。プライベート CA / 自己署名 CA を信頼させたいときに有効化。\n" +
		"#   デフォルト off → 生成物に cert 関連の配線は一切乗らない。\n" +
		"# [certificates]\n" +
		"# enable = true",
	"init_toml_template_dockerfile": "# [dockerfile] — Dockerfile の所定フックポイントにカスタムフラグメントを注入。\n" +
		"#   pre_user_setup は useradd の前、post_plugins はプラグイン install.<category>.sh の後に実行。\n" +
		"# [dockerfile]\n" +
		"# pre_user_setup = \"\"\"RUN apt-get install -y my-extra-pkg\"\"\"\n" +
		"# post_plugins   = \"\"\"RUN echo done\"\"\"",
	"init_toml_template_services": "# [services.<name>] — 同じ Compose ネットワーク上のサイドカー (例: postgres, redis)。\n" +
		"#   dev コンテナからサービス名で到達可。複数定義可。\n" +
		"# [services.postgres]\n" +
		"# image       = \"postgres:16-alpine\"\n" +
		"# environment = { POSTGRES_PASSWORD = \"dev\" }\n" +
		"# ports       = [\"5432:5432\"]",
	"init_toml_template_devcontainer": "# [devcontainer.*] — VS Code Dev Container 用に devcontainer.json へマージされる設定。\n" +
		"#   [workspace] devcontainer = false のときは無視。よくあるのは VS Code 拡張リスト。\n" +
		"# [devcontainer.customizations.vscode]\n" +
		"# extensions = [\n" +
		"#     \"ms-azuretools.vscode-docker\",\n" +
		"#     \"eamodio.gitlens\",\n" +
		"# ]",
	"init_toml_template_code_workspace": "# [code_workspace] — `cocoon gen workspace` が生成する VS Code .code-workspace ファイル。\n" +
		"#   opt-in。サブコマンドを実行したときだけ生成される。出力先はプロジェクトルート\n" +
		"#   (cocoon.toml と同階層、.devcontainer/ 配下ではない) なので `code <name>.code-workspace`\n" +
		"#   でそのまま開ける。folders[].path は \"~\" 展開 + `.code-workspace` を書き出すディレクトリ\n" +
		"#   (既定は cocoon.toml と同階層、`cocoon gen workspace --output <dir>` で上書き可) 起点の\n" +
		"#   相対化に対応するので、\"~/.claude\" のようなエントリも VS Code が上方向に辿れる相対パスへ解決される。\n" +
		"# [code_workspace]\n" +
		"# name = \"my-stack\"                                       # 出力ファイル名 (デフォルト: プロジェクトディレクトリ basename)\n" +
		"# folders = [\n" +
		"#     { path = \".\" },                                     # プロジェクト自身\n" +
		"#     { path = \"~/.claude\" },                             # ホーム配下; name は basename から自動導出\n" +
		"#     { path = \"../sibling-repo\" },                       # 兄弟プロジェクト\n" +
		"#     { path = \"~/.config/nvim\", name = \"Neovim\" },     # 表示名を明示\n" +
		"# ]\n" +
		"# [code_workspace.settings]\n" +
		"# \"editor.tabSize\" = 2\n" +
		"# [code_workspace.extensions]\n" +
		"# recommendations = [\"golang.go\", \"ms-azuretools.vscode-docker\"]",
}
