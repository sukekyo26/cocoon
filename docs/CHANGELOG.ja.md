# Changelog

cocoon の主要な変更を記録します。フォーマットは
[Keep a Changelog](https://keepachangelog.com/ja/1.0.0/) に準拠し、
バージョニングは [Semantic Versioning](https://semver.org/lang/ja/) に従います。

## [Unreleased]

### 追加

- `[container].image` の選択肢を従来の `ubuntu` / `debian` に加えて 5 種の言語ランタイム公式イメージに拡張: `node` (`26-bookworm-slim` / `24-bookworm-slim` / `22-bookworm-slim`)、`python` (`3.14-slim-bookworm` / `3.13-slim-bookworm` / `3.12-slim-bookworm`)、`go` (`1.26-bookworm` / `1.26.3-bookworm` / `1.25-bookworm` / `1.24-bookworm`)、`rust` (`1.95-bookworm` / `1.94-bookworm` / `1.93-bookworm`)、`deno` (`debian-2.7.14` / `debian-2.6.10` / `debian-2.5.7`)。すべて Debian (bookworm) ベースなので既存の apt ベースのプラグインカタログがそのまま機能します。deno は vendor 公式の `denoland/deno`、他の 4 つは Docker official `library/<image>` に展開されます。言語ランタイムイメージを選ぶと該当言語の apt インストール 1 ステップが省ける代わりに FROM レイヤーが少し大きくなります。
- `cocoon init` のインタラクティブピッカーが 7 種のイメージを提示し、選んだイメージごとの推奨候補がバージョン選択肢として並びます。非対話パスは `--image <id>` / `--image-version <tag>`。
- `cocoon init` のバージョン選択を、推奨候補を Tab キーで循環できる **1 画面テキスト入力** に変更。候補を Tab で送るか、任意の正しい形式のタグを直接入力できます (例: `golang:1.26.4-bookworm` を新パッチ公開日にすぐ pin)。非対話パス (`--image-version <tag>`) も同様の集合を受理。

### 変更

- `image_version` の whitelist 厳格チェックを撤廃。Docker タグ文字集合 (英数字・ドット・アンダースコア・ハイフン、スラッシュ / コロン禁止) の形式チェックだけに緩和し、上流レジストリが公開するパッチ・新マイナーをすぐに pin できるようにしました (例: `golang:1.26.4-bookworm`)。`SupportedImageVersions` は `cocoon init` の推奨候補と `docs/configuration.md` の推奨タグ表に位置づけが変わります。タグがレジストリに実在するかは `docker pull` (ビルド時) に委ねます。
- **BREAKING**: `[container].os` / `os_version` を `[container].image` / `image_version` にリネームしました。サポート対象が 2 種の Linux ディストロから 7 種のイメージに広がったため「OS」という名前は実態と乖離しています。マイグレーション: `os = "ubuntu"` → `image = "ubuntu"`、`os_version = "26.04"` → `image_version = "26.04"`。旧キーを残した `workspace.toml` を読み込ませると validator がリライト例入りの fail-fast エラーを出すので、ファイル単位で 1 回の置換で済みます。
- **BREAKING**: `cocoon init` の `--os` / `--os-version` フラグを `--image` / `--image-version` にリネーム。旧フラグの alias は提供しないので、CI で旧フラグを固定指定している箇所は置換が必要です。
- **BREAKING**: `[container].ubuntu_version` (v0.2.0 で deprecated 化していたもの) を撤去。strict TOML パーサが unknown key として reject します。マイグレーション: `image = "ubuntu"` / `image_version = "..."` に直接リライト (v0.2 の `ubuntu_version` → `os` / `os_version` の中間段階は経由しなくて構いません)。
- **BREAKING**: 生成 `Dockerfile` の ARG 名 `OS_IMAGE` / `OS_VERSION` と、対応する `.env` / docker-compose の補間キーを `IMAGE` / `IMAGE_VERSION` にリネーム。cocoon の外側でこれらに依存していた箇所 (例: `docker build --build-arg OS_IMAGE=...` を自分で叩いていた等) は併せて更新してください。
- `image = "go"` と `[plugins].enable = ["go"]` の併用、および `image = "rust"` と `[plugins].enable = ["rust"]` の併用を validation エラーで reject するようにしました。go プラグインは `/usr/local/go` をベースイメージごと上書き、rust プラグインは `$HOME/.cargo/bin` を PATH 先頭に挿入してベースを死蔵させるため、両方有効にすると docker-build 時間を浪費するだけで実行時の挙動は変わりません。ベースイメージかプラグインのどちらか一方を選んでください。エラーメッセージには具体的なリライト案が含まれます。

## [0.2.0] - 2026-05-11

### 追加

- `install.sh` に `COCOON_API_BASE` (デフォルト `https://api.github.com`) と `COCOON_RELEASE_BASE` (デフォルト `https://github.com`) の上書き入力を追加。GitHub Enterprise Server やローカルミラー経由でも公開インストーラを利用可能。
- `install.sh` に `COCOON_API_TOKEN` を追加。GitHub API 呼び出し時に Bearer トークンとして送られ、anonymous 60 req/hour のレート制限 (CI のように同一 runner IP プールを共有する環境でぶつかりやすい) を回避できる。`curl ... | sh` を 1 回だけ手で叩く一般ユーザは設定不要。
- コンテナ内 `/home/<user>/.cocoon` に named volume `cocoon` をマウント。ユーザー個人のシェル設定をコンテナリビルドを跨いで永続化する。コンテナの rc (bash / zsh / fish) が起動時に `~/.cocoon/.shellrc` (fish は `~/.cocoon/.shellrc.fish`) を自動 source するので、コンテナ内で編集した内容は `docker compose down && up --build` を跨いでも残る (リセットは `down -v` のみ)。
- `cocoon init --plugin-versions=<id>=<ref>,...` を追加。1 コマンドで `[plugins] enable` と `[plugins.versions]` の両方を出力できる。各 `<id>` は `--plugins` に含まれ、かつ `version_capable` である必要があり、重複は不可。これまで `cocoon plugin pin` の出力を手で貼り付けていた運用を置き換える。
- `cocoon plugin pin --write` を追加。`workspace.toml` の `[plugins.versions.<id>]` ブロックを直接挿入・置換する。行ベースのミューテータが対象ブロック外のコメント・空行を保持するため、既存ファイルを安全に編集できる。`--write` 無しの stdout-only 動作はデフォルトのまま。`[plugins.versions]` 直下に任意の key 代入 (例: `<id> = "..."` や `<id> = { ... }`) がある場合は重複ブロック追加を避けるため usage error で停止する。
- `workspace.toml` に新セクション `[certificates]` を追加。`enable = true` のとき `~/.cocoon/certs/*.crt` を build 時にコンテナイメージへ自動取り込み、デフォルト (セクション不在 or `enable = false`) では生成された `Dockerfile` / `docker-compose.yml` / `devcontainer.json` に **cert 関連の配線が一切乗らない** (additional_contexts も RUN --mount=type=bind も initializeCommand も SSL_CERT_FILE ENV も出ない)。社内 CA を扱わないチームは corp-CA 機構ゼロの成果物を commit できる。有効時は compose 側で `additional_contexts: cocoon_user_certs: ${HOME:?…}/.cocoon/certs`、Dockerfile 側で `RUN --mount=type=bind,from=cocoon_user_certs …` により他の apt 操作より前に trust store へマージするため、Zscaler 等の TLS インターセプトが行われる corp ネットワーク環境でも build が成立する。
- `cocoon init --certificates` / `--no-certificates` フラグ + 対話プロンプトで上記セクションを切り替え可能。有効化時は生成 `workspace.toml` に `[certificates] enable = true` が出力され、無効時はセクション省略 + コメントテンプレートで後から有効化する手順を案内する。
- 生成 `.devcontainer/devcontainer.json` の `initializeCommand: "mkdir -p ${HOME:?…}/.cocoon/certs"` は `[certificates]` 有効時のみ出力される。有効化済みワークスペースの VS Code Dev Containers ユーザーは cocoon バイナリ無しでもホスト側ディレクトリが build 前に自動作成される。`docker compose build` を直接実行するユーザー (CI 等) はこのフックを通らないため、初回のみホスト側で `mkdir -p ~/.cocoon/certs` を実行する必要がある。
- `cocoon gen` 実行時にホスト側 `~/.cocoon/certs/` (パーミッション 0700) を不在なら自動作成し、社内 / プライベート CA の `.crt` 配置先を案内するメッセージを出力する — ただし `[certificates] enable = true` のときのみ。無効ワークスペースではホスト側への副作用も notice 出力も発生しない。
- 生成 compose の `additional_contexts` と devcontainer の `initializeCommand` で `${HOME:?…}` パラメータ展開を使用。`HOME` 未設定環境では明示的なエラーで fail-fast し、`/.cocoon/certs` への path collapse による sub debug が発生しないようにした。

### 変更

- `cocoon plugin scaffold` の対話プロンプト「install_user.sh も生成する?」に複数段落の説明を追加。root + user 分割の趣旨、付けるべきケース／付けないケース、典型例として starship を提示する。EN / JA 両プロンプトカタログを更新。
- `cocoon gen` がプラグインカタログを `~/.cocoon/cache/build-context/` に展開する処理を廃止。有効化された各プラグインの `install.sh` (および存在すれば `install_user.sh`) は生成 `.devcontainer/Dockerfile` 内へシングルクオートの bash heredoc で直接埋め込まれ、`docker-compose.yml` から `additional_contexts: plugins:` も削除した。これによりビルドはプロジェクトツリー以外を必要とせず、ホストでも dev コンテナ内でも同じように `cocoon gen` を実行できる (従来はキャッシュがホスト `$HOME` 配下に置かれる前提のためビルドは必ずホストで行う必要があった)。残存する `~/.cocoon/cache/build-context/` ディレクトリは再作成されないので、不要なら `rm -rf ~/.cocoon/cache/build-context` で手動削除できる。
- **BREAKING**: `cocoon plugin scaffold` の `--plugins-dir` デフォルトを `./plugins` から `<workspace>/.cocoon/plugins` (`workspace.toml` から自動検出) に変更。`--plugins-dir` 未指定かつ cocoon プロジェクト外で実行した場合は `./plugins/<id>/` に黙って書き込む代わりに actionable error で停止する。明示的に上書きするには `--plugins-dir <path>` を渡す。
- **BREAKING**: TLS 証明書自動取り込みの参照元を `<project>/certs/*.crt` から `~/.cocoon/certs/*.crt` に変更し、機能自体を `[certificates] enable = true` による opt-in 化 (デフォルト off)。移行手順: `workspace.toml` に `[certificates]\nenable = true` を追加し、`mkdir -p ~/.cocoon/certs && mv ./certs/*.crt ~/.cocoon/certs/` を実行、続いて `cocoon gen` を再実行。プロジェクト直下の `certs/` ディレクトリはもはやスキャンされず、ワークスペースが opt-in しない限り cert 配線も生成されない。

### 修正

- `install.sh` が macOS 上で動作するように修正。これまで darwin サポートを宣言していたにもかかわらず Linux coreutils の `sha256sum` を直接呼び出していたため、macOS では `curl ... | sh` が `missing required tool: sha256sum` で停止していた。`sha256sum` が無い環境では `shasum -a 256` にフォールバックする。
- `[install].build_args` を `install.sh` と `install_user.sh` で対称に扱うよう修正。これまでジェネレータは `ARG <name>` 行を `install.sh` の RUN の直前にしか出力していなかったため、`install_user.sh` のみのプラグイン（`install.sh` なし）+ `build_args` の組み合わせでは `${<name>}` がビルド時に空文字に展開されていた。修正後はプラグインごとに 1 回 `ARG <name>` を「先に走る hook」の直前に出力し、両 hook の per-RUN env prefix から build-arg 値を参照できるようにした。ARG のスコープは stage 全体なので 1 回の宣言で両 RUN をカバーでき、両 hook を持つプラグインで重複宣言は発生しない。
- 生成される `docker-compose.yml` の `[workspace] mount_root` 解決を修正。docker-compose は bind mount の相対パスを compose ファイルがあるディレクトリ (`.devcontainer/`) 基準で解決するため、従来の出力は 1 段浅かった。`mount_root = ".."` ではプロジェクトルートしかマウントされず兄弟リポジトリが見えていなかったし、`mount_root = "."` では `.devcontainer/` 自身がマウントされていた。両ケースとも `..` を 1 段足した形で出力されるようになり、本来の対象ディレクトリにマウントされるようになった。
- `install.sh` を持たず `[install.env]` のみを定義したプラグイン (env-only プラグイン) で `ENV` ディレクティブが生成 Dockerfile から silently drop されていた問題を修正。env ブロックを独立したスニペットとして出力し、env 変数が確実にイメージに反映されるようにした。
- カタログプラグイン `claude-code` / `copilot-cli` が `[install.env]` で `~/.local/bin` を `PATH` に追加するように修正。これにより、`uv` 等の他プラグインに依存することなくインストールされた CLI が対話シェルから即時利用可能になる。
- カタログプラグイン `go` に `build-essential` (gcc / make) の apt インストールを追加。これにより cgo ビルドや native 依存ツールの `go install` がそのまま動作する。

### ドキュメント

- `docs/plugins.md` (英語) と `docs/plugins.ja.md` (日本語) を新設。プラグイン作成者向けの単一ソースとして、3 層 LayeredFS / `plugin.toml` 全フィールド表 / `install.sh` と `install_user.sh` の使い分け（判断マトリクス + starship 実例 + fzf / oh-my-zsh / miniconda の仮想例）/ install スクリプトに渡される環境変数 / バージョン pin の契約 / catalog ツアー / トラブルシューティングをまとめた。`plugin-authoring` スキル (SKILL.md) は agent 向け作業手順のみに絞り、仕様面はすべて新ドキュメントへ委譲。
- `docs/commands.md` の plugin セクションを「目的・実行例・落とし穴」付きで全面増補。先頭にレイヤード FS (project > user > embedded) の説明を置き、`docs/plugins.md` への作成者向けクロスリンクを追加。従来の `add → 編集 → 有効化 → gen` ワークフロー記述は「`[plugins].enable` に id を並べる; カスタマイズしたければ cp -r or scaffold」に置き換えた。
- `README.md` と `docs/README.ja.md` を全面書き直し。機能列挙ではなく「Docker / docker-compose を書きたくない人向け」という想定ユーザー宣言と Before/After 比較を冒頭に置く構成に変更した。両 README と `docs/{architecture,configuration,commands,plugins}.{md,ja.md}` の冒頭に v0.x alpha 警告ブロックを追加し、どのドキュメント入口からも開発段階の注意事項が見えるようにした。冒頭バッジが MIT License を示しているため、両 README 末尾の `## License` セクションは削除した。
- `docs/commands.{md,ja.md}` 末尾に「削除済みコマンド」節を追加。撤去された `cocoon config` ノウングループと `cocoon plugin add` / `cocoon plugin remove` を 1 行ずつの移行案内付きで列挙し、古いコマンドを探して辿り着いた読者がすぐに代替手段を見つけられるようにした。

### 削除

- **BREAKING**: `cocoon plugin remove` サブコマンドを削除。実装は `os.RemoveAll` の薄いラッパーで `rm -rf <overlay>` と完全に等価だった。移行手順: `cocoon plugin remove <id> --scope user` を `rm -rf ~/.cocoon/plugins/<id>` (project スコープなら `<workspace>/.cocoon/plugins/<id>`) に置き換える。
- **BREAKING**: `cocoon plugin add` サブコマンドを削除。実装は埋め込みプラグインを書き込み可能な overlay にコピーするだけだったが、「add」という名前が「enable」と誤読されやすかった (LayeredFS のおかげで `[plugins].enable` に id を並べるだけで埋め込みカタログは有効化される)。移行手順: 埋め込みプラグインを使うだけなら `workspace.toml` の `[plugins].enable` に id を列挙する。改変したい場合のサポート手順は `cocoon plugin scaffold <new-id>` で新規 id を作りロジックを移植すること。cocoon リポジトリのクローン（または GitHub Release のソース tarball）を持っているなら `cp -r internal/plugin/catalog/<id> ~/.cocoon/plugins/<id>/` が近道だが、単体バイナリでインストールした場合は embedded ソースがディスク上に存在しないためこの近道は使えない。`add` が呼んでいた `plugin.Materialize` ヘルパも併せて削除。
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

[Unreleased]: https://github.com/sukekyo26/cocoon/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/sukekyo26/cocoon/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/sukekyo26/cocoon/releases/tag/v0.1.0
