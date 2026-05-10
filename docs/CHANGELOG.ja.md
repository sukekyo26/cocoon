# Changelog

cocoon の主要な変更を記録します。フォーマットは
[Keep a Changelog](https://keepachangelog.com/ja/1.0.0/) に準拠し、
バージョニングは [Semantic Versioning](https://semver.org/lang/ja/) に従います。

## [Unreleased]

### 追加

- コンテナ内 `/home/<user>/.cocoon` に named volume `cocoon` をマウント。ユーザー個人のシェル設定をコンテナリビルドを跨いで永続化する。コンテナの rc (bash / zsh / fish) が起動時に `~/.cocoon/.shellrc` (fish は `~/.cocoon/.shellrc.fish`) を自動 source するので、コンテナ内で編集した内容は `docker compose down && up --build` を跨いでも残る (リセットは `down -v` のみ)。
- `cocoon init --plugin-versions=<id>=<ref>,...` を追加。1 コマンドで `[plugins] enable` と `[plugins.versions]` の両方を出力できる。各 `<id>` は `--plugins` に含まれ、かつ `version_capable` である必要があり、重複は不可。これまで `cocoon plugin pin` の出力を手で貼り付けていた運用を置き換える。
- `cocoon plugin pin --write` を追加。`workspace.toml` の `[plugins.versions.<id>]` ブロックを直接挿入・置換する。行ベースのミューテータが対象ブロック外のコメント・空行を保持するため、既存ファイルを安全に編集できる。`--write` 無しの stdout-only 動作はデフォルトのまま。`[plugins.versions]` 直下に任意の key 代入 (例: `<id> = "..."` や `<id> = { ... }`) がある場合は重複ブロック追加を避けるため usage error で停止する。
- `~/.cocoon/certs/*.crt` に置いた TLS 証明書を build 時にコンテナイメージへ自動取り込みする。compose 側で `additional_contexts: cocoon_user_certs: ${HOME}/.cocoon/certs` を宣言し、Dockerfile 側で `RUN --mount=type=bind,from=cocoon_user_certs ...` により他の apt 操作より前に trust store へマージする。これにより Zscaler 等の TLS インターセプトが行われる corp ネットワーク環境でも build が成立する。RUN 内部はシェルレベルの条件分岐になっているため、ユーザー証明書の有無にかかわらず Dockerfile の生成内容は同一。
- 生成 `.devcontainer/devcontainer.json` に `initializeCommand: "mkdir -p ${HOME}/.cocoon/certs"` を追加。VS Code Dev Containers ユーザーは cocoon バイナリ無しでも、ホスト側にディレクトリが build 前に自動作成される。`docker compose build` を直接実行するユーザー (CI 等) はこのフックを通らないため、初回のみホスト側で `mkdir -p ~/.cocoon/certs` を実行する必要がある。
- 生成された `.devcontainer/Dockerfile` / `docker-compose.yml` / `devcontainer.json` は、証明書の有無に関わらず常に同一。チームで commit して共有可能。実際にインストールされる証明書は各開発者の `~/.cocoon/certs/` の状態で決まる。

### 変更

- `cocoon gen` がプラグインカタログを `~/.cocoon/cache/build-context/` に展開する処理を廃止。有効化された各プラグインの `install.sh` (および存在すれば `install_user.sh`) は生成 `.devcontainer/Dockerfile` 内へシングルクオートの bash heredoc で直接埋め込まれ、`docker-compose.yml` から `additional_contexts: plugins:` も削除した。これによりビルドはプロジェクトツリー以外を必要とせず、ホストでも dev コンテナ内でも同じように `cocoon gen` を実行できる (従来はキャッシュがホスト `$HOME` 配下に置かれる前提のためビルドは必ずホストで行う必要があった)。残存する `~/.cocoon/cache/build-context/` ディレクトリは再作成されないので、不要なら `rm -rf ~/.cocoon/cache/build-context` で手動削除できる。
- **BREAKING**: `cocoon plugin scaffold` の `--plugins-dir` デフォルトを `./plugins` から `<workspace>/.cocoon/plugins` (`workspace.toml` から自動検出) に変更。`--plugins-dir` 未指定かつ cocoon プロジェクト外で実行した場合は `./plugins/<id>/` に黙って書き込む代わりに actionable error で停止する。明示的に上書きするには `--plugins-dir <path>` を渡す。
- **BREAKING**: TLS 証明書自動取り込みの参照元を `<project>/certs/*.crt` から `~/.cocoon/certs/*.crt` に変更。移行手順: `mkdir -p ~/.cocoon/certs && mv ./certs/*.crt ~/.cocoon/certs/`、続いて `cocoon gen` を再実行。プロジェクト直下の `certs/` ディレクトリはもはやスキャンされない。

### 修正

- 生成される `docker-compose.yml` の `[workspace] mount_root` 解決を修正。docker-compose は bind mount の相対パスを compose ファイルがあるディレクトリ (`.devcontainer/`) 基準で解決するため、従来の出力は 1 段浅かった。`mount_root = ".."` ではプロジェクトルートしかマウントされず兄弟リポジトリが見えていなかったし、`mount_root = "."` では `.devcontainer/` 自身がマウントされていた。両ケースとも `..` を 1 段足した形で出力されるようになり、本来の対象ディレクトリにマウントされるようになった。
- `install.sh` を持たず `[install.env]` のみを定義したプラグイン (env-only プラグイン) で `ENV` ディレクティブが生成 Dockerfile から silently drop されていた問題を修正。env ブロックを独立したスニペットとして出力し、env 変数が確実にイメージに反映されるようにした。
- カタログプラグイン `claude-code` / `copilot-cli` が `[install.env]` で `~/.local/bin` を `PATH` に追加するように修正。これにより、`uv` 等の他プラグインに依存することなくインストールされた CLI が対話シェルから即時利用可能になる。
- カタログプラグイン `go` に `build-essential` (gcc / make) の apt インストールを追加。これにより cgo ビルドや native 依存ツールの `go install` がそのまま動作する。

### ドキュメント

- `docs/commands.md` の plugin セクションを「目的・実行例・落とし穴」付きで全面増補。先頭にレイヤード FS (project > user > embedded) の説明と `add → 編集 → 有効化 → gen` の典型ワークフローを追加。

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
