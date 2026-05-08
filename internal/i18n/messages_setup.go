package i18n

func init() {
	register(LangEN, messagesEN_setup)
	register(LangJA, messagesJA_setup)
}

// messagesEN_setup mirrors locale/en.sh keys setup_*.
//
//nolint:gochecknoglobals,revive // catalog tables are file-scoped by design.
var messagesEN_setup = map[string]string{
	"setup_unknown_arg":          "Unknown argument: %s",
	"setup_inside_container":     "This script cannot be run from inside a container",
	"setup_header_regenerate":    "Regenerate from workspace.toml",
	"setup_service_info":         "Service: %s",
	"setup_username_info":        "Username: %s",
	"setup_plugins_info":         "Plugins: %s",
	"setup_header_generate":      "Generate Dockerfile for the dev container",
	"setup_service_default":      "Service name: %s (default)",
	"setup_username_current":     "Username: %s (current user)",
	"setup_prompt_service_name":  "Enter container service name: ",
	"setup_prompt_username":      "Enter container username: ",
	"setup_prompt_os":            "Select base OS:",
	"setup_prompt_os_version":    "Select OS version:",
	"setup_os_default":           "Base OS: %s (default)",
	"setup_os_preserved":         "Base OS: %s (preserved)",
	"setup_os_version_default":   "OS version: %s (default)",
	"setup_os_version_preserved": "OS version: %s (preserved)",
	"setup_header_software":      "Software Installation Selection",
	"setup_plugin_enabled":       "  %s: enabled (default)",
	"setup_plugin_skipped":       "  %s: skipped",
	"setup_select_plugins":       "Select plugins to install:",
	"setup_port_default":         "Forward ports: %s (preserved)",
	"setup_port_none":            "Forward ports: none",
	"setup_header_port":          "Port Configuration",
	"setup_prompt_port":          "Forward ports (comma-separated, Enter to skip): ",
	"setup_invalid_port":         "Invalid port number: %s (must be 1-65535)",
	"setup_gen_workspace_toml":   "Generating workspace.toml...",
	"setup_header_certs":         "Custom CA Certificates Detected",
	"setup_certs_will_install":   "The following certificates will be installed from certs/:",
	"setup_gen_env":              "Generating .env...",
	"setup_gen_all":              "Generating workspace files (Dockerfile, docker-compose.yml, .devcontainer/, shell rc fragment)...",
	"setup_created_shellrc":      "Created config/%s from example",
	"setup_home_files":           "Ensuring host files for [home_files]:",
	"setup_home_files_touched":   "  - touched %s",
	"setup_home_files_inside_container": "ERROR: [home_files] is configured but `wsd setup` is running inside a container.\n" +
		"       The host-side touch step would create files inside the container's filesystem\n" +
		"       (a different mount namespace) instead of on the host, leaving every bind-mount\n" +
		"       source missing on the actual host.\n" +
		"       Run `wsd setup` (or `bash setup-docker.sh`) from the host (WSL2 / macOS / Linux)\n" +
		"       where the home-directory files actually need to persist, then come back into\n" +
		"       the container.",
	"setup_docker_gid_failed":   "Failed to detect Docker GID",
	"setup_docker_gid_hint":     "Tried: /var/run/docker.sock, rootless socket, docker group\nIf docker group does not exist: sudo groupadd docker && sudo usermod -aG docker $USER && newgrp docker",
	"setup_detected_docker_gid": "Detected Docker GID: %s",
	"setup_complete":            "=== Setup Complete ===",
	"setup_result_service":      "Container service name: %s",
	"setup_result_username":     "Username: %s",
	"setup_result_os":           "Base OS: %s",
	"setup_result_os_version":   "OS version: %s",
	"setup_result_uid_gid":      "UID/GID: %s/%s (automatically detected)",
	"setup_result_docker_gid":   "Docker GID: %s (automatically detected)",
	"setup_result_plugins":      "Enabled plugins:",
	"setup_result_plugin_item":  "  - %s: Yes",
	"setup_result_certs":        "  - Custom CA Certificates: Yes (from certs/)",
	"setup_result_port":         "Port forwarding: %s",
	"setup_result_port_none":    "Port forwarding: none",
	"setup_result_files":        "Generated files:",
	"setup_build_hint":          "You can build the Docker image with the following command:",
	"setup_start_hint":          "To start the container:",
	"setup_access_hint":         "To access the container:",
	"setup_stop_hint":           "To stop the container:",
	"setup_reconfig_hint":       "To reconfigure:",
}

//nolint:gochecknoglobals,revive // catalog tables are file-scoped by design.
var messagesJA_setup = map[string]string{
	"setup_unknown_arg":          "不明な引数: %s",
	"setup_inside_container":     "このスクリプトはコンテナ内から実行できません",
	"setup_header_regenerate":    "workspace.toml から再生成",
	"setup_service_info":         "サービス: %s",
	"setup_username_info":        "ユーザー名: %s",
	"setup_plugins_info":         "プラグイン: %s",
	"setup_header_generate":      "開発コンテナ用 Dockerfile 生成",
	"setup_service_default":      "サービス名: %s (デフォルト)",
	"setup_username_current":     "ユーザー名: %s (現在のユーザー)",
	"setup_prompt_service_name":  "コンテナサービス名を入力: ",
	"setup_prompt_username":      "コンテナのユーザー名を入力: ",
	"setup_prompt_os":            "ベース OS を選択:",
	"setup_prompt_os_version":    "OS バージョンを選択:",
	"setup_os_default":           "ベース OS: %s (デフォルト)",
	"setup_os_preserved":         "ベース OS: %s (保持)",
	"setup_os_version_default":   "OS バージョン: %s (デフォルト)",
	"setup_os_version_preserved": "OS バージョン: %s (保持)",
	"setup_header_software":      "ソフトウェアインストール選択",
	"setup_plugin_enabled":       "  %s: 有効 (デフォルト)",
	"setup_plugin_skipped":       "  %s: スキップ",
	"setup_select_plugins":       "インストールするプラグインを選択:",
	"setup_port_default":         "フォワードポート: %s (保持)",
	"setup_port_none":            "フォワードポート: なし",
	"setup_header_port":          "ポート設定",
	"setup_prompt_port":          "フォワードポート (カンマ区切り、Enter でスキップ): ",
	"setup_invalid_port":         "無効なポート番号: %s (1-65535 の範囲で入力してください)",
	"setup_gen_workspace_toml":   "workspace.toml を生成中...",
	"setup_header_certs":         "カスタム CA 証明書を検出",
	"setup_certs_will_install":   "以下の証明書が certs/ からインストールされます:",
	"setup_gen_env":              ".env を生成中...",
	"setup_gen_all":              "ワークスペースファイルを生成中 (Dockerfile, docker-compose.yml, .devcontainer/, シェル rc フラグメント)...",
	"setup_created_shellrc":      "config/%s をサンプルから作成しました",
	"setup_home_files":           "[home_files] のホストファイルを確保中:",
	"setup_home_files_touched":   "  - %s を作成",
	"setup_home_files_inside_container": "エラー: [home_files] が設定されていますが `wsd setup` がコンテナ内で実行されています。\n" +
		"        ホスト側の touch 処理がコンテナ内のファイルシステム (別のマウント名前空間) に作用してしまい、\n" +
		"        ホスト側の bind マウントソースは存在しないままになります。\n" +
		"        `wsd setup` (または `bash setup-docker.sh`) は、実際にホームディレクトリのファイルを\n" +
		"        永続化したいホスト (WSL2 / macOS / Linux) 側で実行してください。実行後にコンテナへ\n" +
		"        入り直してください。",
	"setup_docker_gid_failed":   "Docker GID の検出に失敗しました",
	"setup_docker_gid_hint":     "試行: /var/run/docker.sock, rootless ソケット, docker グループ\ndocker グループが存在しない場合: sudo groupadd docker && sudo usermod -aG docker $USER && newgrp docker",
	"setup_detected_docker_gid": "Docker GID を検出: %s",
	"setup_complete":            "=== セットアップ完了 ===",
	"setup_result_service":      "コンテナサービス名: %s",
	"setup_result_username":     "ユーザー名: %s",
	"setup_result_os":           "ベース OS: %s",
	"setup_result_os_version":   "OS バージョン: %s",
	"setup_result_uid_gid":      "UID/GID: %s/%s (自動検出)",
	"setup_result_docker_gid":   "Docker GID: %s (自動検出)",
	"setup_result_plugins":      "有効なプラグイン:",
	"setup_result_plugin_item":  "  - %s: はい",
	"setup_result_certs":        "  - カスタム CA 証明書: はい (certs/ から)",
	"setup_result_port":         "ポートフォワーディング: %s",
	"setup_result_port_none":    "ポートフォワーディング: なし",
	"setup_result_files":        "生成されたファイル:",
	"setup_build_hint":          "Docker イメージは以下のコマンドでビルドできます:",
	"setup_start_hint":          "コンテナの起動:",
	"setup_access_hint":         "コンテナへのアクセス:",
	"setup_stop_hint":           "コンテナの停止:",
	"setup_reconfig_hint":       "再設定:",
}
