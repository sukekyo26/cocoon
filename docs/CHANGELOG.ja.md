# Changelog

cocoon の主要な変更を記録します。フォーマットは
[Keep a Changelog](https://keepachangelog.com/ja/1.0.0/) に準拠し、
バージョニングは [Semantic Versioning](https://semver.org/lang/ja/) に従います。

## [Unreleased]

### 削除

- **BREAKING**: `cocoon config` 名詞グループを削除 (`get` / `list` / `volumes` / `plugin-get` / `plugin-list` / `plugin-volumes` / `plugins-table` / `validate-workspace` / `validate-plugins` / `has-section` / `list-sidecars` / `dump-devcontainer` / `dump-repositories` / `repositories` / `format-repositories`)。これらは v0.1.0 で全廃された bash entry-point スクリプト用の低レベル TOML アクセサで、cocoon 内部では既に未使用。`cocoon config` でスクレイプしていた外部スクリプトは専用の TOML パーサ (`tomlq` / `taplo` や小さな Go / Python ヘルパ) に切り替えてください。

## [0.1.0] - 2026-05-09

### 追加

- `cocoon init` を追加。サービス名・ユーザー名・OS・OS バージョン・ログインシェル・マウント範囲・devcontainer 出力切替・エイリアスバンドル・apt カテゴリ・プラグインを対話で選んで `workspace.toml` を生成。
- 非対話用フラグ (`--yes`, `--service-name`, `--username`, `--os`, `--os-version`, `--shell`, `--mount-root`, `--devcontainer`, `--no-devcontainer`, `--apt-categories`, `--plugins`, `--alias-bundles`, `--force`) を追加。CI やスクリプトから TTY なしで `cocoon init` を駆動可能。
- 生成 `workspace.toml` にローカライズされたインラインコメントと 20 個のコメントアウト済セクション雛形を追加し、ファイル内で機能を発見できるようにする。
- `cocoon gen` を追加。`workspace.toml` から `.devcontainer/{Dockerfile, docker-compose.yml, docker-entrypoint.sh, .env, devcontainer.json}` を生成。
- `cocoon plugin` 名詞グループを追加 (`list` / `show` / `add` / `remove` / `pin` / `scaffold` の 6 サブコマンド)。20 プラグインの埋め込みカタログと `LayeredFS` (project > user > embedded) による上書きをサポート。
- `cocoon config` 名詞グループを追加 (`get` / `list` / `volumes` / `plugin-get` / `plugin-list` / `plugin-volumes` / `plugins-table` / `validate-workspace` / `validate-plugins` / `has-section` / `list-sidecars` / `dump-devcontainer` / `dump-repositories` / `repositories` / `format-repositories`)。
- `cocoon self-update` を追加。GitHub リリースからのダウンロード、SHA256 検証、atomic rename による差し替えに対応。
- `cocoon version` を追加。
- `cocoon init` で選択できる apt カテゴリ 10 種 (`text-editors`, `vcs`, `utilities`, `compression`, `build`, `search`, `network`, `monitoring`, `python3`, `json-yaml`) を追加。
- `cocoon init` で選択できるエイリアスバンドル 3 種 (`git`, `ls`, `docker`) を追加。`[container.shell] aliases` にマージされる。
- Dockerfile heredoc によるシェル rc 注入を追加。`[container.shell] env` と `aliases` がイメージビルド時に `~/.bashrc` / `~/.zshrc` / `~/.config/fish/config.fish` へ直接反映される。
- `COMPOSE_PROJECT_NAME` をプロジェクトディレクトリの basename から導出するように変更。docker compose の namespace がホストディレクトリと一致する。
- 国際化 (英語 / 日本語) カタログを追加。CLI プロンプト・エラーメッセージ・`workspace.toml` インラインコメントすべてを `WORKSPACE_LANG` / `LC_ALL` / `LC_MESSAGES` / `LANG` で切替可能。

[Unreleased]: https://github.com/sukekyo26/cocoon/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/sukekyo26/cocoon/releases/tag/v0.1.0
