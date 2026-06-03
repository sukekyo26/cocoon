# Changelog

cocoon の主要な変更を記録します。フォーマットは
[Keep a Changelog](https://keepachangelog.com/ja/1.0.0/) に準拠し、
バージョニングは [Semantic Versioning](https://semver.org/lang/ja/) に従います。

## [Unreleased]

### 追加

- `cocoon lock` が、有効な `version_capable` プラグインのバージョン制約を
  ネットワーク越しに具体バージョン（と arch ごとの SHA256 checksum）へ解決し、
  workspace ルートに `cocoon.lock` を書き出します。これにより `cocoon gen` が
  再現可能なワークスペースを生成します。`"latest"` 制約は最新リリースへ凍結され、
  `"=x.y.z"` pin は checksum を記録します。`--check` は解決せずに lock が
  `workspace.toml` と一致するか検証（CI 用）、`--upgrade` は `"latest"` 制約を
  再解決します。上流が機械可読なバージョンを公開しないプラグイン（`aws-cli`,
  `android-sdk`, `flutter`）は exact バージョンへの pin が必要です。

### 変更

- **BREAKING**: `[plugins.versions]` のエントリは文字列のバージョン制約に
  なりました — `<id> = "=1.23.4"`（exact pin）または `<id> = "latest"`。
  従来のインラインテーブル形式（`<id> = { pin = "1.23.4" }`）は廃止されたので、
  各エントリを文字列に書き換えてください。追加のバージョン入力を持つプラグイン
  （例: android-sdk の `api_level` / `build_tools`）はテーブル形式を維持し、制約を
  `version` キーに置きます: `<id> = { version = "=…", api_level = "…" }`。旧 `pin`
  形式は読み込み時に移行ヒント付きで拒否されます。range 演算子（`>=`・`^`・`~`
  など）は非対応です。
- **BREAKING**: `cocoon plugin pin <id> <ref>` は文字列制約 `<id> = "=<ref>"` を
  書き込むようになりました（素の `<ref>` は exact pin、`latest` を渡すと最新
  リリースを追従）。`--amd64-checksum` / `--arm64-checksum` は受け付けません。

### 削除

- **BREAKING**: `workspace.toml` の `[plugins.versions]` で `checksum_amd64` /
  `checksum_arm64` キーは無効になりました。arch ごとの checksum は `cocoon.lock`
  に記録されます。これらを残した `workspace.toml` は移行ヒント付きで拒否されます。

## [0.13.0] - 2026-06-03

### 追加

- `cocoon init --sudo nopasswd|password|none`（および対応する対話プロンプト）が
  `--secure` / `--no-secure` フラグを置き換え、コンテナ内 sudo の方針を 1 つの
  3 値選択で指定できるようになりました。
  - `nopasswd`（既定）: パスワード不要の sudo。従来どおり。
  - `password`: sudo にパスワードを要求し、新フィールド `[container.sudo] mode =
    "password"` に記録します。パスワードはイメージの**ビルド時**に
    `.devcontainer/.env.local`（`SUDO_PASSWORD=...` の 1 行）から Docker build
    secret（`RUN --mount=type=secret`）として読み込みます。**平文**はイメージ層・
    ビルドキャッシュ・コンテナ環境変数・`docker inspect` のいずれにも残らず、
    `/etc/shadow` には派生ハッシュのみ書き込まれます（パスワードを持つ通常の
    Unix アカウントと同様）。`cocoon gen` は compose の `secrets:` 配線と
    `.env.local` を除外する
    `.devcontainer/.gitignore` を生成し、password モードなのに `.env.local` が
    無いか空なら警告します。`SUDO_PASSWORD` が未設定/空ならビルドは失敗し、
    passwordless へ暗黙にフォールバックしません。対話で `password` を選ぶと
    パスワードを尋ねて `.devcontainer/.env.local`（mode 0600・既存は上書きしない）
    を生成します。`--yes --sudo password` はモードのみ設定し、ファイル作成は
    ユーザーに委ねます。BuildKit が必要です（生成される Dockerfile が有効化済み）。
  - `none`: `[container.security_opt] no_new_privileges = true` を事前設定します
    （旧 `--secure` の挙動）。password モードとは排他です。

### 削除

- **BREAKING**: `cocoon init --secure` / `--no-secure` を削除し、統一された
  `cocoon init --sudo` フラグに置き換えました。スクリプトや CI では `--secure`
  を `--sudo none` に、`--no-secure` を `--sudo nopasswd` に置き換えてください
  （旧名のエイリアスは受け付けません）。`[container.security_opt]
  no_new_privileges` フィールド自体は不変なので、既存の `workspace.toml` は
  編集なしで読み込めます。

## [0.12.0] - 2026-06-02

### 追加

- `cocoon init --secure`（および対応する対話プロンプト）を追加しました。生成される
  `workspace.toml` に `[container.security_opt] no_new_privileges = true` を事前
  設定します。setuid 権限昇格を遮断して、未信頼コードや AI エージェントを実行する
  コンテナを硬化します。トレードオフとしてコンテナ内の `sudo` は no-op になります
  （root が要るときはホストから `docker exec -u root`）。UID/GID/DOCKER_GID の
  再マップは無影響です。デフォルト挙動は不変で、`--no-secure` を渡すかフラグを省略
  すれば passwordless `sudo` のままです。

### 変更

- `cocoon init` のベースイメージ既定を、`--image` / `--image-version` 省略時に
  `ubuntu:26.04` から `debian:12` (bookworm) へ変更しました。対話のイメージバージョン
  選択では debian の `12` を先頭 (推奨) に表示します。`ubuntu` を含む他のサポート
  イメージ・タグは引き続き選択できます。

### 修正

- `android-sdk` プラグインが、固定していた `openjdk-17-jdk-headless` ではなく
  `default-jdk-headless` をインストールするようにしました。Debian 13 (trixie) と
  Ubuntu 24.04+ は `openjdk-17-jdk-headless` をパッケージとして提供しておらず、
  従来これらのイメージでビルドに失敗していました。メタパッケージは各イメージの
  既定 JDK (bookworm は 17、trixie / Ubuntu 24.04+ は 21) に解決されます。

## [0.11.0] - 2026-06-01

### 追加

- `cocoon init` で選択できる apt カテゴリ `agent` (デフォルト OFF) を追加しました。
  `jq` / `yq` / `ripgrep` / `fd-find` / `tree` とシステム `python3` (+ `pip` /
  `venv`) を 1 つのチェックボックスにまとめた、AI エージェント運用向けのバンドルです。
  `search` (ripgrep, fd-find) と `utilities` (tree) のパッケージと重複しますが、
  併せて選択しても冪等です (各パッケージは 1 回だけインストールされます)。

### 削除

- **BREAKING**: apt カテゴリ `json-yaml` と `python3` を新カテゴリ `agent` に統合し、
  削除しました。移行方法: `--apt-categories json-yaml` / `--apt-categories python3`
  を `--apt-categories agent` に置き換えてください — 未知のカテゴリ id は usage error
  で即座に失敗します。`search` カテゴリ (`fzf` / `ripgrep` / `bat` / `fd-find`) は
  変更していないため、`ripgrep` / `fd-find` は `agent` バンドル全体を選ばずとも
  à la carte で入手できます。

## [0.10.2] - 2026-05-31

### 変更

- 生成される `.devcontainer/docker-compose.yml` が、すべての文字列値を
  ダブルクォートで出力するようになった（例: `command: "sleep infinity"`、
  `ipc: "host"`、`dns` の各エントリ、`environment` の値）。従来は YAML 特殊文字を
  含む値だけがクォートされ、クォート有無が混在していた。整形のみの変更で、
  パースもランタイム動作も同一（`${VAR}` 展開も従来どおり）。`cocoon gen` で
  再生成して追従。

### 修正

- host 側とコンテナ側でポート番号が異なる `[ports].forward` エントリ
  （例: `"30002:3000"`、long form `{ target = 5432, published = 15432 }`）について、
  `.devcontainer/devcontainer.json` の `forwardPorts` がコンテナ側ポートを
  出力するよう修正。従来は host 側ポートを出力しており、VS Code が誰も listen
  していないコンテナポートを転送しようとしていた。生成される
  `docker-compose.yml` の `ports:` マッピングは不変。既存の `.devcontainer/` は
  `cocoon gen` で再生成して追従させてください。

## [0.10.1] - 2026-05-31

### 修正

- 低速回線でバイナリのダウンロード中に `cocoon self-update` が
  `context deadline exceeded` で失敗する問題を修正。アセットのダウンロード
  （約 12MB のバイナリと `SHA256SUMS`）に、GitHub Releases 参照の 30 秒とは別の
  3 分のタイムアウトを設けたので、転送に 30 秒以上かかる場合でも API 用の短い
  デッドラインで打ち切られず完了できる。

## [0.10.0] - 2026-05-31

### 追加

- 新しい `azure-cli` プラグインを追加。[Azure CLI](https://learn.microsoft.com/cli/azure)
  （`az`）を Microsoft 公式の apt リポジトリ（`packages.microsoft.com`）から、
  Microsoft の署名鍵で検証してインストールする。Microsoft はリポジトリをリリース
  コードネーム単位で公開しているため、未公開のコードネームのベース（例: Debian 13）
  では、壊れたビルドにせず明確なメッセージで早期に失敗する。`~/.azure` を永続化し、
  `az login` の状態がコンテナ再ビルド後も残る。
- 新しい `gcloud` プラグインを追加。[Google Cloud CLI](https://cloud.google.com/sdk)
  （`gcloud`, `gsutil`, `bq`）を Google 公式の apt リポジトリ
  （`packages.cloud.google.com`）から、Google の署名鍵で検証してインストールする。
  `CLOUDSDK_CONFIG=~/.gcloud` を設定して永続化し、`gcloud auth login` の状態が
  コンテナ再ビルド後も残る。
- 新しい `golangci-lint` プラグインを追加。[Go 用リンタランナー](https://github.com/golangci/golangci-lint)
  を GitHub Release のバイナリからインストールする。`version_capable`
  （`[plugins.versions].golangci-lint` でピン留め可）で、未ピンのビルドは
  最新リリースを解決し、ダウンロードをリリースの `checksums.txt` で検証する。
- 新しい `codex` プラグインを追加。[OpenAI Codex CLI](https://github.com/openai/codex)
  （OpenAI のターミナル向けコーディングエージェント）を GitHub Release の musl
  バイナリからインストールする。`version_capable`（`[plugins.versions].codex` で
  ピン留め可）で、`~/.codex`（認証・設定）を再ビルド後も永続化する。OpenAI は
  チェックサムマニフェストを公開していないため、未ピンのビルドは検証を警告付きで
  スキップする（`shfmt` / `shellcheck` と同じ）。検証済みビルドには
  `checksum_amd64` / `checksum_arm64` をピン留めする。

### 変更

- `cocoon` プラグインの `binary` インストール方式の説明文（`cocoon plugin show` で
  表示）からミラー URL を削除。

## [0.9.2] - 2026-05-31

### 変更

- `shfmt` プラグインが、pin 無し（LATEST）ダウンロードの検証のために上流の
  `sha256sums.txt` マニフェストを取得しなくなりました。shfmt は v3.13.0 以降
  このマニフェストを上流で廃止しており（GitHub がアセットごとのダイジェストを
  ネイティブ提供）、pin 無しの `shfmt` ビルドがハードフェイルしていました。
  `shfmt` は `shellcheck` と同じ方式になりました。`[plugins.versions]` に
  `checksum_amd64` / `checksum_arm64` を pin した場合はそれと照合し、未指定の
  場合は警告を出して検証をスキップしてビルドを続行します。完全に検証された
  再現可能なインストールには、バージョンを checksum 付きで pin してください。

### 修正

- `node` プラグインがデフォルト（pin 無し・最新 LTS）バージョンでインストールに
  失敗していました。バージョン解決で nodejs.org の約 200 KB の `index.tab` を
  `curl` から直接 `awk ... exit` にパイプしており、awk が最初の一致でパイプを
  閉じると curl が書き込みエラー（exit 23）で中断し、`set -o pipefail` により
  ビルドが失敗していました。index を parse 前にダウンロードするよう変更。pin
  済みの `node` バージョンは影響を受けていません。

## [0.9.1] - 2026-05-30

### 変更

- TLS 証明書の自動取り込み機能（`[certificates] enable = true`）が、`~/.cocoon/certs/`
  の `*.crt` に加えて `*.cer` も取り込むようになりました。両拡張子とも build 時に
  コンテナのトラストストアへマージされます（`update-ca-certificates` は `.crt`
  拡張子しか取り込まないため、`.cer` は `.crt` にリネームしてコピーします）。証明書は
  PEM 形式である必要があり、DER 形式の `.cer` はスキップされます（先に
  `openssl x509 -inform der -in x.cer -out x.crt` で変換してください）。

### 修正

- **Security**: `cocoon gen` が `[container.shell.env]` / `[container.shell.aliases]`
  の値に含まれる改行、およびプラグインの `[install.env]` の値に含まれる改行・
  ダブルクォートを拒否するようになりました。これらの文字は従来、生成される
  Dockerfile の heredoc / `ENV` 行を脱出し、ビルド時に任意のディレクティブを
  注入できました。`$` 展開（`$HOME`・`$PATH`・`$(cmd)`）は引き続きサポートされます。
- `cocoon gen` がロード／生成の失敗時に `failure: failure:` と prefix を
  二重表示しなくなりました（prefix は 1 回になります）。
- `cocoon gen` / `cocoon plugin pin` が `workspace.toml` の探索中に発生した
  permission・I/O エラーを、ワークスペースなしとして黙殺せず報告するように
  なりました。

## [0.9.0] - 2026-05-30

### 変更

- `terraform` / `opentofu` プラグインが、ダウンロードを毎回上流の GPG
  リリース署名（HashiCorp / OpenTofu の署名鍵をインストールスクリプトに同梱）
  で検証するようになりました。`aws-cli` と同じ `verify = "pgp"` 方式で、検証は
  **常時有効**です（従来は pin 無しビルドが警告のみで整合性チェックをスキップ
  していました）。これに伴い、この 2 つは `[plugins.versions]` の
  `checksum_amd64` / `checksum_arm64` を受け付けなくなりました（`cocoon gen`
  が拒否。`aws-cli` と同様）。`pin` でのバージョン指定は引き続き可能です。
  リリース毎の checksum 保守は不要で、上流鍵が失効しても過去の署名は検証
  できます（実際の鍵ローテーション時のみ鍵の更新が必要）。
- `lazygit` / `gitleaks` / `just` / `shfmt` / `starship` / `deno` /
  `docker-buildx` / `helm` / `copilot-cli` プラグインが、`[plugins.versions]`
  に `checksum_amd64` / `checksum_arm64` を pin していない場合でも、上流が各
  リリースで公開する checksum マニフェスト（`checksums.txt` / `SHA256SUMS`
  ファイル、または資産ごとの `.sha256` / `.sha256sum`）と照合するように
  なりました。従来は pin 無しビルドが警告のみで検証をスキップしていましたが、
  デフォルトビルドが常時検証されます。ユーザー指定の checksum が引き続き優先
  され、リリース毎の checksum 保守は不要です（マニフェストは資産と同じ
  リリースから取得）。
- `kubectl` / `go` / `node` / `zig` / `dart` / `flutter` / `nerd-fonts` /
  `cocoon` プラグインが、`[plugins.versions]` に `checksum_amd64` /
  `checksum_arm64` を pin していない場合でも、上流が公開する checksum
  （`.sha256` / `.sha256sum` サイドカー、`SHASUMS256.txt` / `SHA-256.txt`
  / `SHA256SUMS` マニフェスト、または上流リリース JSON の `shasum` /
  `sha256` フィールド）と照合するようになりました。従来は pin 無しビルドが
  警告のみで検証をスキップしていましたが、デフォルトビルドが常時検証され
  ます。ユーザー指定の checksum が引き続き優先され、リリース毎の checksum
  保守は不要です。`go` は `go.dev/dl` がリダイレクトする CDN
  `dl.google.com/go` から取得するようになり、tarball と `.sha256` が同一
  ホストになりました。

### 修正

- **Security**: `google-chrome` プラグインが Chrome を Google の署名付き
  apt リポジトリ（`signed-by` keyring）からインストールするようになりました。
  従来の「`.deb` を TLS 取得して未検証のままインストール」を廃止し、apt が
  Google の固定署名鍵で全 Chrome パッケージを検証します（`docker-cli` /
  `github-cli` と同じ方式）。Chrome の Linux 版は引き続き amd64 のみです。
- `docker-cli` プラグインがビルド時に `gnupg`（`vcs` apt カテゴリ）を必要と
  しなくなりました。Docker 署名鍵を ASCII-armored のまま保存し `signed-by`
  で直接参照する方式に変更し、`gpg --dearmor` を廃止しました。従来は `vcs`
  カテゴリを外して `docker-cli` を有効にすると最小ベースで
  `gpg: command not found` で失敗していました。apt のパッケージ署名検証は
  変更ありません。

## [0.8.0] - 2026-05-30

### 追加

- `plugin.toml` に新しい `[install.extra_versions]` セクションを追加。
  プラグインがサブコンポーネントのバージョンを「ユーザーが上書き可能な
  つまみ」として公開できます。各エントリは
  「キー名（`[plugins.versions].<id>` に書く名前）」「`env`（install
  スクリプトに渡る環境変数名）」「`default`（workspace.toml で未指定時
  の値）」を宣言します。`env` は `^[A-Z_][A-Z0-9_]*$` に一致し、cocoon
  予約 env 名（`PIN`、`CHECKSUM_*`、`RC_FILE`、`RC_SYNTAX`、
  `LOGIN_SHELL`、`COCOON_INSTALL_METHOD`、`USERNAME`）や
  `[install].build_args` の名前と衝突できません。
  プラグインの `[install].build_args` エントリ自体も同じ予約 env
  集合と衝突する名前は拒否されるようになりました — `build_args`
  ペアは framework value の後に RUN env プレフィックスへ追加される
  ため、衝突すると silent shadow が起きるため。
  予約キー名（`pin`、`checksum_amd64`、`checksum_arm64`）を
  `extra_versions` のキーとして宣言することは拒否されます
  （`[plugins.versions]` が予約フィールドとして消費するため、宣言しても
  ユーザーが上書きできない no-op になるため）。`default` は必須・非空。
  プラグイン側 `default` と workspace 側 override のどちらも
  `"` / `\` / `\n` / `\r` / `$` / backtick を含む値は拒否されます
  （値は Dockerfile の RUN 行の `KEY="..."` 環境変数に展開される
  ため、前 4 文字は shell quoting を破壊し、`$` / backtick は
  parameter / command substitution を引き起こしてリテラルな
  バージョン文字列として渡らなくなる）。
- `workspace.toml` の `[plugins.versions].<id>` インラインテーブルが、
  予約済み 3 キー（`pin` / `checksum_amd64` / `checksum_arm64`）に加えて、
  プラグインが `[install.extra_versions]` で宣言した任意のキーを受け
  入れるようになりました。例:
  `android-sdk = { pin = "14742923", api_level = "36", build_tools = "36.0.0" }`。
  未宣言キー（typo や削除済み宣言）は `cocoon gen` が拒否するため、
  default に silently fallback する事故を防ぎます。`cocoon plugin pin
  <id> --write` は既存行の extra キーを保全し、`pin` / `checksum_*`
  のみ書き換えます。
- `android-sdk` プラグイン（catalog id `android-sdk`、`archive` メソッド）。
  Android command-line tools (`sdkmanager`) を
  `/usr/local/android-sdk` にインストールし、同じ RUN 内で `sdkmanager`
  を実行して `platform-tools` / `platforms;android-<api_level>` /
  `build-tools;<build_tools>` を取得します。`ANDROID_HOME` /
  `ANDROID_SDK_ROOT` を export し、`PATH` の先頭に
  `cmdline-tools/latest/bin` と `platform-tools` を追加します。
  apt で OpenJDK 17 (AGP 8.x baseline) も同梱。`pin` は
  commandline-tools の `BUILD_NUMBER`。空 `pin` を指定したときは
  <https://developer.android.com/studio> の HTML を build 時にスクレイプして
  最新 BUILD_NUMBER に解決します（`version_capable` 契約に合わせた挙動）。
  `api_level`（default
  `35`）と `build_tools`（default `35.0.0`）を `[install.extra_versions]`
  で公開しているため、catalog に手を加えずに workspace 側で platform /
  build-tools のバージョンを上書きできます。Linux amd64 / arm64 両対応
  （ZIP は JVM-only で arch 非依存、同じ SHA を両方に pin します）。
  Android emulator と `system-images` は初期版のスコープ外です。
  `flutter` と組み合わせると `flutter doctor` の Android toolchain
  チェックが緑になります。

### 変更

- `cocoon self-update` がインストール先ディレクトリに現ユーザーの書込権限が
  無いケース（例: `/usr/local/bin/cocoon` のような root 所有ロケーション）
  で早期に失敗し、対処方法を提示するようになりました。従来は新バイナリ
  (~12 MB) をダウンロードして SHA256 検証を通したあとで `rename` 時に失敗
  し、内部の `*.cocoon-update.tmp` パスを露出した `permission denied` のみ
  が表示されていました。エラーメッセージが書込不可ディレクトリを名指しし、
  `sudo <selfPath> self-update` で再実行するよう案内します。
  `cocoon self-update --check-only` は read-only 操作なのでこの preflight
  をスキップし、root 所有のインストール先でも sudo なしにバージョン確認
  ができます。

### 修正

- **セキュリティ**: `[plugins.versions].<id>.pin` の値に `"`, `\`, `\n`,
  `\r`, `$`, backtick が含まれる場合に拒否するよう修正。pin は Dockerfile の
  RUN 行の `PIN="..."` env ペアに展開されるため、従来は裸のクォートで
  クォートを抜け出せ、`$` / backtick でパラメータ・コマンド置換が起きました。
  つまり細工された `workspace.toml` で `docker build` 時に任意コマンドが
  実行され得ました。これは `[install.extra_versions]` の override 値に既に
  適用されているガードと同じものです。

## [0.7.6] - 2026-05-24

### 追加

- 新しい `gitleaks` プラグインを埋め込みカタログに追加。
  [gitleaks](https://github.com/gitleaks/gitleaks) シークレットスキャナを
  公式 GitHub Release tarball から `/usr/local/bin/gitleaks` に SHA256 検証付き
  でインストールします (`linux_x64` / `linux_arm64`)。`[plugins.versions]` で
  `gitleaks = { pin = "..." }` を省略した場合は
  `https://github.com/gitleaks/gitleaks/releases/latest` から最新安定版の tag
  を解決します。`default = false` のため、利用するには
  `[plugins].enable = [..., "gitleaks"]` で明示的に有効化してください。

## [0.7.5] - 2026-05-24

### 追加

- GitHub Pages ミラー (`.github/workflows/pages.yml`) を追加: リリースのたび
  `install.sh`、ビルド済み cocoon バイナリ、`SHA256SUMS` を
  `https://sukekyo26.github.io/cocoon/` に発行します。各リリースタグは
  `/v<tag>/` 配下に不変アーカイブとして残り、最新リリースは `/latest/` と
  `/VERSION` にもミラーされます。`*.github.io` には到達できるが
  `raw.githubusercontent.com` / `api.github.com` には到達できない環境向けの
  代替インストール経路です。`actions/deploy-pages` は毎デプロイでサイト全体
  を置き換えるため、ワークフローは毎回 `gh release list` を辿って全 tag の
  アセットを再ダウンロードし、サイト全体を再構築します。これにより過去
  リリースの `/v<tag>/` アーカイブは新規リリース後も到達可能なまま維持され
  ます。GitHub Releases の "latest" フラグが付いたリリースが
  `/install.sh` / `/VERSION` / `/latest/` を populate します。
  `workflow_dispatch` は入力を取らず、現在の Releases 状態からサイトを
  再構築するだけです（取りこぼしたリリースの back-fill や、Pages が
  古い状態に陥った場合の復旧に使ってください）。
- `install.sh` に `COCOON_PAGES_BASE` 環境変数を追加 (既定: 空)。設定すると、
  最新バージョンは GitHub API ではなく `$COCOON_PAGES_BASE/VERSION` から
  読み込み、バイナリと `SHA256SUMS` も `github.com/.../releases/...` ではなく
  `$COCOON_PAGES_BASE/v<tag>/...` から取得します。未設定時の挙動 (GitHub
  API / GitHub Releases 経路) は従来通りで変化ありません。
- README に Pages ミラー版のワンライナー
  (`curl -fsSL https://sukekyo26.github.io/cocoon/install.sh | COCOON_PAGES_BASE=... sh`)
  を追加し、既定経路と並ぶ代替インストール経路として記載しました。
- 新しい `cocoon` プラグインを埋め込みカタログに追加。dev container 内で
  cocoon バイナリを GitHub Pages ミラー
  (`https://sukekyo26.github.io/cocoon/v<pin>/cocoon-linux-{amd64,arm64}`)
  からダウンロードして SHA256 検証付きでインストールします。
  `[plugins.versions]` で `cocoon = { pin = "..." }` を省略した場合、
  install スクリプトは `https://sukekyo26.github.io/cocoon/VERSION` から
  最新 stable を解決します。Pages ミラーからのみダウンロードし、
  `github.com/.../releases/download/...` ルートにはフォールバックしません。
  そのため `*.github.io` には到達できるが `raw.githubusercontent.com` /
  `api.github.com` には到達できない環境でも動作します。
  `default = false` のため、利用するには
  `[plugins].enable = [..., "cocoon"]` で明示的に有効化してください。

## [0.7.4] - 2026-05-24

### 追加

- 新しい `just` プラグイン: [just](https://github.com/casey/just) コマンドランナー（make の現代的な代替）を公式 GitHub Release tarball から `/usr/local/bin/just` に展開し、SHA256 検証を行います（`x86_64-unknown-linux-musl` / `aarch64-unknown-linux-musl`）。`[plugins.versions]` で `just = { pin = "..." }` を省略した場合は `https://github.com/casey/just/releases/latest` から最新安定版を解決します（just のリリースタグは `v` プレフィックスを持たない素のセマンティックバージョンなので、URL は `releases/download/<ver>/...` の形）。`default = false` のため、利用するには `[plugins].enable = [..., "just"]` に明示的に追加してください。

## [0.7.3] - 2026-05-24

### 修正

- `cocoon init` の `--image-path-fix`（language base image 選択時に既定 on で出るプロンプト）が `[container.shell.env]` ブロックと一緒にトップレベルの `[volumes]` ブロックも書き出すようになりました。これにより image 路線でも `docker compose down && up --build` を跨いで user install (`npm install -g <pkg>` / `cargo install <pkg>` / `go install <pkg>` / `deno install <script>`) の成果が保持されます。従来は env だけ設定して書き込み先パス自体は volume 化されておらず、コンテナ再生成のたびに毎回ゼロから消えていた（PATH は通っているのに rebuild で消える、という非対称な footgun）。イメージ別の追加ブロック: `node` → `npm-global` (`$HOME/.npm-global`) + `npm` (`$HOME/.npm` キャッシュ)、`golang` → `go` (`$HOME/go`、`$GOPATH` 全体)、`rust` → `cargo` (`$HOME/.cargo`)、`denoland/deno` → `deno` (`$HOME/.deno`)。volume 名は対応する catalog plugin の `[install].volumes` と同じ `plugin.DeriveVolumeName` 規約で派生するため、image 路線とプラグイン路線で生成される compose `volumes:` キーは一致します（image⇄plugin を切替えても compose snapshot が構造的に等価）。`python` は唯一のインストール先 (`$HOME/.local/bin`) が cocoon の既定 `local:` named volume に既に含まれるため変更なし。env と volume は 1 つのトグルで連動: `--no-image-path-fix`（またはプロンプトで「いいえ」）を選ぶと両方ともスキップされます（volume だけあって env が無いと、runtime が書き込まない無意味なマウントになるため）。既存の workspace.toml で image-path-fix の `[container.shell.env]` ブロックを持っているファイルは自動マイグレーションされないので、`cocoon init --force` を再実行するか、生成された自動コメントに従って `[volumes]` ブロックを手で追記してください。

## [0.7.2] - 2026-05-24

### 追加

- `cocoon init` で言語ベースイメージ（`node` / `python` / `golang` / `rust` / `denoland/deno`）を選んだとき、`[container.shell.env]` に user-local の `PATH` / インストール先を自動追加するか問うようになりました（既定 on）。これがないと、公式イメージは（a）user install を root 所有の `/usr/local/...` に書き込もうとして失敗する（`npm install -g` / `pip install` / `cargo install` が `EACCES`、python 3.11+ では PEP 668 もヒット）か、（b）書き込み可能な user ディレクトリに置くが `PATH` に無くて呼び出せない（`go install` → `$HOME/go/bin`、`deno install` → `$HOME/.deno/bin`、`pip install --user` → `$HOME/.local/bin`）状態になります。対話プロンプトは確認前に追加するキー・値をプレビューします。生成された `[container.shell.env]` ブロックの直上には自動コメント 3 行（追加理由 / 削除した場合の影響 / インライン `env = { ... }` 形式とは併用不可の警告）が付くので、時間が経ってもそのセクションが何のために追加されたか読み取れます。新フラグ `--image-path-fix` で自動設定を強制 on、`--no-image-path-fix` で自動設定をスキップできます（どちらも image 依存のため `--image` の指定が必須で、非言語イメージに対しては早期に ErrUsage で失敗するため、スクリプトの typo を黙ってスキップしません）。`ubuntu` / `debian` ではプロンプト自体出ません。イメージ別の追加内容は: `node` → `NPM_CONFIG_PREFIX="$HOME/.npm-global"` + `PATH="$HOME/.npm-global/bin:$PATH"`、`python` → `PATH="$HOME/.local/bin:$PATH"`、`golang` → `PATH="$HOME/go/bin:$PATH"`、`rust` → `CARGO_INSTALL_ROOT="$HOME/.cargo"` + `PATH="$HOME/.cargo/bin:$PATH"`（`CARGO_HOME` は意図的にイメージ既定のままにし、rustup の状態と `cargo build` のレジストリキャッシュを移さない）、`denoland/deno` → `PATH="$HOME/.deno/bin:$PATH"`。

## [0.7.1] - 2026-05-23

### 修正

- `cocoon gen` がロケール生成を「`/etc/locale.gen` の該当行をアンコメント → `locale-gen` を引数なしで実行」の正攻法に変更しました。従来は `locale-gen <name>` のように locale 名を引数に渡していましたが、Debian 系の一部イメージ（特に `*-slim` 系）では `locales` パッケージが配布する `/etc/locale.gen` が全行コメントアウトされた状態で出荷されており、`locale-gen <name>` だけでは要求した locale が実体化されず、`LC_ALL` が設定されているのに生成されていないためコンテナ起動時に `bash: warning: setlocale: LC_ALL: cannot change locale (...)` が出続けていました。新方式は Debian/Ubuntu canonical な手順を踏みつつ、既に locale がアンコメント済みのイメージでも no-op で安全に動作します。

## [0.7.0] - 2026-05-23

### 追加

- `cocoon <command> --help` の出力が `cocoon init` プロンプトや `cocoon gen` メッセージと同じ `WORKSPACE_LANG` / `LC_ALL` / `LC_MESSAGES` / `LANG` のロケール判定に追従するようになりました。`WORKSPACE_LANG=ja` を指定する (または `ja_*` ロケールで実行する) と、コマンド説明・フラグ usage・セクション見出しがすべての主要サブコマンド (`init`, `gen`, `gen workspace`, `plugin {list,show,pin,scaffold}`, `self-update`, `version`, `completion`, `help`) で日本語化されます。英語表示は引き続き既定のままです。ただし root の英語ヘルプのレイアウトは少し変わります。これまでの cocoon 独自の見出し (`Commands:` と末尾の `Run 'cocoon <command> --help' for command-specific usage.` のヒント行) は廃止され、cobra 標準のレイアウト (`Available Commands:` と `Use "cocoon [command] --help" for more information about a command.`) に統一されました。これにより全サブコマンドのヘルプが同一テンプレートに揃います。翻訳対象はヘルプ表示のみで、サブコマンド名・フラグ名・オプション値はシェルスクリプトでの取り扱いを変えないよう ASCII を維持します。
- `cocoon gen workspace` サブコマンドを新設しました。`workspace.toml` の新しい `[code_workspace]` セクションから VS Code `.code-workspace` ファイルを生成します。出力は **プロジェクトルート** (`workspace.toml` と同階層、`.devcontainer/` 配下ではない) なので `code <name>.code-workspace` でそのまま開けます。`[code_workspace].folders` は inline-table 配列 (`{ path, name }`) を受け付け、`path` は `~` 展開 + `.code-workspace` を書き出すディレクトリ (既定は `workspace.toml` と同階層、`--output <dir>` 指定時はその先) 起点の相対化に対応するため、`"~/.claude"` のようなエントリも VS Code が上方向に辿れる相対パスへ解決されます。`[code_workspace]` は `settings` テーブルと `extensions.recommendations` 配列も受け付け、いずれも verbatim に反映されます。CLI フラグも 2 つ追加: `--name <basename>` で出力ファイル名を上書き、`--folder <path>[=<name>]` (反復可) で `workspace.toml` を編集せず一時的にフォルダを追加できます。既存の `cocoon gen` は変更なし — このサブコマンドは opt-in です。
- `[workspace]` にオプションの `dir` フィールドを追加しました。コンテナ内 workdir の親ディレクトリ (`/home/<user>/` 配下) を上書きできます (既定 `workspace`)。スラッシュで多段階層も可 (例: `dir = "work/myproject"`)。AWS SAM などコンテナ内パスをホスト構成に合わせたいツール向け。値は `docker-compose.yml` の bind mount と `working_dir`、`devcontainer.json` の `workspaceFolder`、生成 `Dockerfile` の `WORKDIR` に反映されます。`cocoon init` で対話入力を取り (非対話なら `--dir`)、既定値でも `dir = "..."` を `workspace.toml` に必ず書き出します。
- 新しい `dart` プラグインを追加。`storage.googleapis.com/dart-archive/channels/stable/release/` から Dart SDK を取得して `/usr/local/dart` に展開し、SHA256 検証を行います (`linux-x64` / `linux-arm64`)。`[plugins.versions]` で `dart = { pin = "..." }` を省略すると、公式の `channels/stable/release/latest/VERSION` エンドポイントから最新 stable を自動解決します。`PUB_CACHE=/home/${USERNAME}/.pub-cache` を named volume で永続化するため、`dart pub global activate <pkg>` の成果がコンテナ再ビルドを跨いで残ります。
- 新しい `flutter` プラグインを追加。`storage.googleapis.com/flutter_infra_release/releases/stable/linux/` から Flutter SDK を取得して `/usr/local/flutter` に展開し、SHA256 検証を行います。**Linux/amd64 のみ対応** — Flutter は公式 Linux/arm64 ビルドを提供していないため、arm64 ホスト上では install が早期失敗し、`docker --platform linux/amd64` でコンテナを起動するよう案内します (Apple Silicon 上の Docker Desktop は自動的に linux/amd64 をエミュレートするので影響なし。ネイティブ arm64 Linux ホストでのみ問題になります)。`[plugins.versions]` で `flutter = { pin = "..." }` を省略すると、公式の `releases_linux.json` マニフェストから `current_release.stable` ハッシュを照合して最新 stable を自動解決します。`[apt].packages` に Linux desktop ビルド toolchain (`clang`, `cmake`, `ninja-build`, `pkg-config`, `libgtk-3-dev`, `liblzma-dev`, `build-essential`) と archive/utility 系依存 (`git`, `unzip`, `xz-utils`, `zip`, `libglu1-mesa`) を同梱しているので、追加設定なしで `flutter doctor` の Linux toolchain が緑になります。`PUB_CACHE=/home/${USERNAME}/.pub-cache` と `/home/${USERNAME}/.flutter` を named volume で永続化します。

### 修正

- `cocoon gen` が `[container.shell].env` の値をダブルクォートのセマンティクスで出力するようになり、rc を source した時点で `$HOME` / `$PATH` がシェルにより展開されるようになりました (`$(cmd)` は bash/zsh では常に、fish では 3.4+ が必要 — 古い fish は fish 固有の `(cmd)` 記法を使う)。従来はシングルクォートで囲んでいたため `export NPM_CONFIG_PREFIX='$HOME/.local'` となり、`$HOME` がリテラル文字列のまま残って `npm install -g` などが存在しないパスに書き込もうとしていました。fish 側 (`set -gx K "$HOME/..."`) も同じ修正を反映しています。リテラルの `$` は生成器に渡す値が `\$` を含む必要があります (`\` はシェルに素通し)。TOML では通常文字列 `"\\$RAW"` かリテラル文字列 `'\$RAW'` と書きます (`"\$RAW"` は TOML として無効なエスケープです)。alias の本文は呼び出し時にシェルが再パースするため、これまでどおりシングルクォートのまま出力します (`$1` や `$HOME` を含めても呼び出し時に正しく展開されます)。

### 削除

- **BREAKING**: `workspace.toml` から `[git]` と `[repositories]` を削除しました。両セクションは既に非推奨と明記されていましたが、ローダが unknown field として拒否するようになります。移行方法: `[git]` は `[home_files] files = [".gitconfig"]` に置き換え、生成される `.devcontainer/initializeCommand` がホストの `~/.gitconfig` を bind-mount します (不在時は 0o600 で touch するので、bind mount がディレクトリを誤作成しません)。`[repositories]` は `mount_root = ".."` を設定し、親ディレクトリ配下にホスト側で `git clone` する従来パターンに統一してください — 旧セクションが表現していた "fat workspace" レイアウトと同じことが、ホストの既存 git 認証情報で再現可能かつ debuggable に実現できます。

## [0.6.0] - 2026-05-18

### 追加

- `plugin.toml` にオプションの `[version].verify` フィールドを追加しました。`version_capable` プラグインがダウンロードをどう検証するかを選びます: `"checksum"`（既定 — install スクリプトが `$CHECKSUM_AMD64` / `$CHECKSUM_ARM64` を検証）または `"pgp"`（スクリプトが同梱署名鍵で in-script 検証し、workspace 単位の checksum を取らない）。`verify = "pgp"` のプラグインに `[plugins.versions]` で `checksum_amd64` / `checksum_arm64` を設定する、または `cocoon plugin pin --amd64-checksum` / `--arm64-checksum` を渡すと、対処方法を示すエラーで拒否されます。
- **セキュリティ**: リリースバイナリ（`cocoon-linux-amd64` / `cocoon-linux-arm64` / `cocoon-darwin-amd64` / `cocoon-darwin-arm64`）と `SHA256SUMS` に、リリースワークフローが生成する署名付きの build provenance attestation（SLSA provenance、Sigstore 経由）が付与されるようになりました。ダウンロードしたアセットは `gh attestation verify <file> --repo sukekyo26/cocoon` で、cocoon の GitHub Actions リリースパイプラインでビルドされ別の場所で再ビルド・差し替えされていないことを確認できます。

### 修正

- **セキュリティ**: `install.sh` ブートストラップインストーラの全ダウンロードを HTTPS に固定（`curl --proto '=https' --tlsv1.2`）。cocoon のプラグイン install スクリプトと同じ姿勢に揃えた。従来はプロトコル無制限で、ネットワーク攻撃者がリクエストを平文 HTTP にダウングレード／リダイレクトし、整合性検証が照合する `SHA256SUMS` を差し替える余地があった。
- **セキュリティ**: `aws-cli` / `aws-sam-cli` / `nerd-fonts` プラグインがダウンロードを検証し、バージョン固定に対応するようになりました。従来は各プラグインが upstream の `latest` 成果物を整合性チェックなしで取得してインストーラを実行しており、upstream リリースや CDN が汚染されると `docker build` 中に root で任意コード実行が起き得ました。3 つとも `version_capable` になり、他の versioned プラグインと同様に `[plugins.versions]` で固定できます。`aws-cli` は install スクリプトに同梱した署名鍵で AWS の detached PGP 署名を検証します（AWS は SHA256 を公開していないため）。`aws-sam-cli` と `nerd-fonts` は `[plugins.versions]` に `checksum_amd64` / `checksum_arm64` を指定すると SHA256 チェックサムを検証します。
- `cocoon gen` が、install スクリプトに CRLF（または単独の CR）改行を含むプラグインを拒否するようになりました。従来はそのまま install heredoc に埋め込んでいたため、混入した復帰文字が `docker build` 中に各コマンドを静かに壊していました。エラーはプラグイン名を示し、スクリプトを LF 改行で保存し直すよう促します。影響を受けるのは Windows で作成したカスタムプラグインのみで、同梱の catalog プラグインはすべて LF です。
- `cocoon gen` がプラグインの衝突を決定論的に報告するようになりました。有効なプラグインの衝突ペアが複数あるとき、毎回同じペアを報告します（プラグイン id をソート順に走査）。従来は Go のランダム化されたマップ反復順に依存しており、同じ `workspace.toml` でも実行のたびに異なる衝突メッセージが出ることがありました。
- `cocoon init --plugins` が重複したプラグイン id（例: `--plugins go,go`）を明確なエラーで拒否するようになった。従来は `[plugins].enable` が重複した `workspace.toml` を書き出し、後続の `cocoon gen` で初めて拒否されていた。
- `cocoon plugin pin` が `version_capable` でないプラグインを明確なエラーで拒否するようになった。従来は `cocoon gen` が後で拒否する `[plugins.versions]` エントリを出力していた。

## [0.5.0] - 2026-05-16

### 追加

- `[container]` に 4 つのオプションフィールドを追加しました。それぞれ対応する Compose の `services:` 属性に出力されます: `group_add` (コンテナユーザーの補助グループ — グループ名または数値 GID)、`devices` (ホストデバイスのマッピング `HOST:CONTAINER[:rwm]`)、`ipc` (IPC 名前空間モード。共有メモリを要する ML 用途では `"host"` など)、`gpus` (GPU アクセス。現状 `"all"` のみサポート)。`cocoon init` は 4 フィールドのコメントアウト済みテンプレートを `[container]` 配下に書き出します。
- `docker-cli` プラグインが有効なのに `[container].docker_socket` が未設定のとき、`cocoon gen` が警告するようになりました — コンテナ内の Docker クライアントが接続できる daemon ソケットが無いため、`docker` コマンドは実行時に失敗します。警告は修正方法 (`[container]` に `docker_socket = true`) を示し、リモートの `DOCKER_HOST` に接続する構成では無視してよい旨も伝えます。`cocoon init` も `[container]` 配下にコメントアウト済みの `docker_socket` テンプレート行を書き出すようになりました。
- `cocoon gen` が `.devcontainer/manage.sh` も書き出すようになりました。ホスト側で実行するプロジェクト単位の Docker クリーン / リビルド用ヘルパーです。`./.devcontainer/manage.sh clean` はこのプロジェクトのコンテナ・ネットワーク・ボリューム・ローカルビルド済みイメージを一括削除します。`clean containers` / `clean image` / `clean volumes` は 1 種類のリソースだけを削除し他は残します (例: `clean volumes` はビルド済みイメージを残すので高速リビルドできます)。`rebuild` は `--no-cache` でイメージを再ビルドしコンテナを再生成します。`prune-cache` は Docker のビルドキャッシュを prune します (他と違い構造上プロジェクト単位にスコープできないため全体対象です)。破壊的なコマンドは `-y` を渡さない限り実行前に確認します。スコープは自動 — スクリプトは生成された compose ファイルに対して `docker compose` を駆動するので、無関係なプロジェクトには触れません。

### 変更

- **BREAKING (生成物)**: 生成される `.devcontainer/` がホスト非依存になり、共有リポジトリにコミットして安全に使えるようになりました — チーム全員がコミットされた同じ `.devcontainer/` をビルドでき、各自での再生成は不要です。生成される `.devcontainer/.env` から `UID` / `GID` / `DOCKER_GID` キーを削除し、`docker-compose.yml` からも `user:` オーバーライド・`UID`/`GID`/`DOCKER_GID` の `build.args`・`group_add:` を削除しました。イメージはコンテナユーザーを固定 uid/gid (1000) で作成します。コンテナは `root` で起動し、`docker-entrypoint.sh` がバインドマウントされたワークスペースのホスト側所有者に合わせてユーザーを再マッピング (および `docker_socket` 有効時は docker ソケットのグループへ追加) してから、そのユーザーへ権限を落とします。`devcontainer.json` には `"remoteUser"` と `"updateRemoteUserUID": false` を追加し、VS Code が独自のホスト UID 再マッピングを重ねずに非特権ユーザーでアタッチするようにしました。`cocoon gen` を再実行して生成物を更新・コミットしてください。チームメンバーは `docker compose build` (または dev container を開き直す) だけで利用できます。エントリポイントが起動時に `CHOWN` / `SETUID` / `SETGID` ケイパビリティを必要とするため、`cocoon gen` はこれら (または `ALL`) を含む `[container.capabilities].drop` を拒否します。`[[mounts]].target` の検証も厳格化し、`[A-Za-z0-9._/-]` と `${USERNAME}` プレースホルダのみを許可します (target が生成 Dockerfile にも展開されるようになったため)。
- **BREAKING (プラグイン作者向け)**: `docker-cli` プラグインは `DOCKER_GID` ビルド引数を取らなくなりました (`plugin.toml` から `[install].build_args` を削除)。マウントされた docker ソケットへのコンテナユーザーのアクセスは、起動時に `docker-entrypoint.sh` が設定します。docker-cli の旧 `build_args = ["DOCKER_GID"]` パターンを真似してホストのグループ id を受け取っていたカスタムプラグインは、それを削除してソケットグループの処理をエントリポイントに任せてください。
- `docker-cli` プラグインが既定で有効ではなくなりました — `plugin.toml` を `default = false` に変更しました。`--plugins` を指定しない `cocoon init --yes` は `docker-cli` を事前選択せず `[plugins] enable = []` を生成し、対話式のプラグイン選択も初期チェックなしで始まります。コンテナ内で Docker クライアントが必要な場合は `docker-cli` を明示的に有効化し (`--plugins` または選択画面で)、ホスト daemon に到達できるよう `docker_socket = true` も併せて設定してください。

## [0.4.0] - 2026-05-15

### 追加

- `cocoon init` の対話モードで、`plugin.toml` の `[install.methods]` に 2 つ以上のエントリを宣言したプラグインに対して「インストール方式」のピッカーを表示するようにしました。各選択肢には method 名と description を併記し、プラグインの `default_method` を初期選択にしているので、推奨どおりで良ければそのまま Enter で確定できます。method を 1 つしか持たないプラグインはサイレントにスキップ — 一般プラグインに余計なプロンプトが増えることはありません。選択結果は生成される `workspace.toml` の新セクション `[plugins.methods]` (1 行 1 プラグインの `<id> = "<method>"` 形式) に書き出され、ここに現れないプラグインはインストール時に `default_method` にフォールバックします。method プロンプトは version プロンプトの **前** に走るので、選んだ method 固有の上流 URL が version ピッカーの説明欄に正しく出ます。
- 新フラグ `cocoon init --plugin-methods` を追加: `--plugin-versions` フラグと同じ形式 (`--plugin-methods="<id>=<method>,<id>=<method>"`) で、指定されたプラグインの method プロンプトをスキップします (`<id>` は `--plugins` にも含まれている必要があり、`<method>` はそのプラグインの `[install.methods]` に宣言されたキーでなければなりません)。`--yes` と組み合わせれば method 選択も含めて CI で完全に non-interactive 実行できます。
- `cocoon plugin pin` に `--method <name>` フラグを追加: pin は `[plugins.versions]` のインラインテーブル行に加えて `[plugins.methods]` 側の `<id> = "<method>"` 行も出力（`--write` 時は in-place で upsert）します。指定された method がプラグインの `plugin.toml` に宣言されているか検証し、未宣言だった場合は宣言済み method 名一覧をエラーに含めて出すので typo は即座に直せます。checksum (`--amd64-checksum` / `--arm64-checksum`) はワークスペーススコープのまま (method 別ではない) なので、method を切り替えるときは新しいアーティファクトに合う SHA256 を渡し直してください。
- `copilot-cli` プラグインに 2 つのインストール方式を提供: 既定の **`installer`** (`gh.io` 経由の上流インストーラ) と **`binary`** (`github/copilot-cli` の GitHub Release から `copilot-linux-${arch}.tar.gz` を直接ダウンロードして `~/.local/bin/copilot` に展開)。`gh.io` に到達できない環境や `curl|sh` をポリシーが禁止する環境では `binary` を選びます。切替は `cocoon init --plugin-methods="copilot-cli=binary"`（または対話ピッカー）。切替後は新しいアーティファクトに合う `checksum_amd64` / `checksum_arm64` を `[plugins.versions]` 側で更新してください（install スクリプトの `sha256sum -c -` 検証が走るため）。

### 変更

- **BREAKING (プラグイン作者向け)**: プラグインの install スクリプト名 `install.sh` は廃止しました。catalog の各プラグインは `install.<category>.sh` を持ち、`plugin.toml` には対応する `[install.methods.<category>]` の宣言が必須となります。loader はリテラル `install.sh` を reject し、エラーメッセージで移行手順を案内します。`<category>` は catalog 共通の 4 語彙語: **`binary`** (単一バイナリ配置)、**`installer`** (vendor の curl-to-bash 経由)、**`apt`** (apt repo / .deb)、**`archive`** (複数ファイルの tar/zip 展開) のいずれか。`~/.cocoon/plugins/<id>/install.sh` を持つカスタムプラグインを使っている場合のみ影響 (リネーム + plugin.toml に `[install.methods.<category>]` 追加が必要)。catalog 同梱のプラグインを使うエンドユーザーは影響なし。
- **BREAKING**: `cocoon plugin scaffold --template` は catalog 4 語彙 (`installer` / `binary` / `apt` / `archive`) を受理するようになりました (旧名 `curl-pipe` / `tarball` / `generic` は廃止)。scaffold 出力は `install.<category>.sh` 形式となり、生成される `plugin.toml` には `[install.methods.<category>]` と `default_method` が自動挿入されます (どちらも loader 必須化のため)。`--template tarball` は "unknown template" エラーで reject されるので `--template binary --version-capable` に置き換えてください。

## [0.3.1] - 2026-05-14

### 追加

- 新しい `node` プラグイン: nodejs.org 公式 tarball から Node.js を `/usr/local/node` にインストールし、SHA256 で検証します (`linux-x64` / `linux-arm64`)。`[plugins.versions]` 配下の `node = { pin = "..." }` を省略するとインストールスクリプトが `https://nodejs.org/dist/index.tab` をパースして最新 LTS を自動解決します。`NPM_CONFIG_PREFIX=/home/${USERNAME}/.npm-global` を設定し、`npm install -g` の書き込み先を `/usr/local` ではなくユーザーホーム配下の named volume に逃すので、`~/.npm` (キャッシュ) と `~/.npm-global` (グローバルインストール先) は再ビルドを跨いで永続化されます。
- 新しい `deno` プラグイン: GitHub Release の `deno-*-unknown-linux-gnu.zip` から Deno を `/usr/local/bin/deno` にインストールし、SHA256 で検証します (`x86_64` / `aarch64`)。`[plugins.versions]` 配下の `deno = { pin = "..." }` を省略するとインストールスクリプトが `releases/latest` のリダイレクトから最新 stable タグを取得します。`DENO_DIR=/home/${USERNAME}/.deno` を named volume として永続化します。
- `image = "node"` と `[plugins].enable = ["node"]` の併用、および `image = "denoland/deno"` と `[plugins].enable = ["deno"]` の併用を validation エラーで reject するようにしました。あわせて `cocoon init` のピッカーでも対応する base image を選んだとき該当プラグインを選択肢から非表示にします。どちらの組み合わせも node プラグインが `/usr/local/node/bin` を PATH 先頭に挿してベース (`/usr/local/bin/node`) を死蔵させ、deno プラグインは `/usr/local/bin/deno` を直接上書きするため、両方有効にすると docker-build 時間を浪費するだけで実行時の挙動は変わりません (`golang` ↔ `go` / `rust` ↔ `rust` の既存挙動と統一)。
- `cocoon init` の対話モードで、有効化した version_capable プラグイン 1 つずつに「LATEST / その他 (手動入力)」の 2 行のピッカーを表示するようにしました (イメージバージョンの選択 UI と同じ形式)。`Enter` で LATEST を確定、もしくはカーソルを 1 行下げて自由入力に切り替えてからバージョンを入力します。入力されたバージョンが上流に実在するかどうかの検証は行いません — プロンプトの説明文で上流のリリースページを参照するよう案内し、TOML が壊れない文字集合のみ regex で検査します (`image_version` と同じ規則)。`--plugin-versions` フラグで pin 済みの id は picker をスキップ (フラグ優先)。LATEST は `[plugins.versions]` から該当エントリを省く形で表現され、install.sh の PIN 空時の latest 解決ロジックが発動します。
- 新しい `kubectl` プラグイン: `https://dl.k8s.io/release/v${VERSION}/bin/linux/${ARCH}/kubectl` から Kubernetes CLI を `/usr/local/bin/kubectl` にインストールし、SHA256 で検証します (amd64 / arm64)。`[plugins.versions]` 配下の `kubectl = { pin = "..." }` を省略するとインストールスクリプトが `https://dl.k8s.io/release/stable.txt` を読んで最新 stable バージョンを自動解決します。
- 新しい `helm` プラグイン: 公式の `https://get.helm.sh/helm-v${VERSION}-linux-${ARCH}.tar.gz` から Kubernetes パッケージマネージャを `/usr/local/bin/helm` にインストールし、SHA256 で検証します (amd64 / arm64)。`[plugins.versions]` 配下の `helm = { pin = "..." }` を省略するとインストールスクリプトが GitHub の `helm/helm` リポジトリで `releases/latest` のリダイレクトを辿って最新 stable タグを取得します。
- 新しい `shellcheck` プラグイン: GitHub Release の `shellcheck-v${VERSION}.linux.${DOWNLOAD_ARCH}.tar.xz` から shell スクリプト静的解析ツールを `/usr/local/bin/shellcheck` にインストールし、SHA256 で検証します (`x86_64` / `aarch64`)。`[plugins.versions]` 配下の `shellcheck = { pin = "..." }` を省略するとインストールスクリプトが `koalaman/shellcheck` の `releases/latest` リダイレクトから最新タグを取得します。最小ベースイメージで tar.xz を展開できるよう、`xz-utils` を apt 依存として宣言しています。
- 新しい `shfmt` プラグイン: GitHub Release の `shfmt_v${VERSION}_linux_${ARCH}` (Go 製の static binary) を `/usr/local/bin/shfmt` にインストールし、SHA256 で検証します (amd64 / arm64)。`[plugins.versions]` 配下の `shfmt = { pin = "..." }` を省略するとインストールスクリプトが `mvdan/sh` の `releases/latest` リダイレクトから最新タグを取得します。
- 新しい `dev-tools` apt カテゴリ (デフォルト OFF): `git-lfs` (ML モデル / メディア / 大容量バイナリ asset 用)、`strace` (コンテナ内で固まったプロセスのシステムコール追跡)、`tmux` (端末多重化。`docker exec` で長時間 build / training を仕掛けて切断しても継続) を一括で導入します。
- 新しい `docker-buildx` プラグイン: BuildKit ベースの `docker buildx` CLI プラグイン (単体バイナリ) を `docker/buildx` の GitHub Release から `/usr/libexec/docker/cli-plugins/docker-buildx` にインストールし、SHA256 で検証します (amd64 / arm64)。`docker-cli` プラグイン (または docker 同梱のベースイメージ) と組み合わせると、cocoon e2e ワークフロー側の `docker buildx bake` と同等の操作 (`docker buildx version` / `buildx build --cache-from=type=gha` / マルチプラットフォーム build) がコンテナ内から直接行えるようになります。`[plugins.versions]` 配下の `docker-buildx = { pin = "..." }` を省略するとインストールスクリプトが `docker/buildx` の `releases/latest` リダイレクトから最新タグを取得します。

### 変更

- **BREAKING**: `cocoon init` および `cocoon plugin pin --write` の `[plugins.versions]` 出力形式を **インラインテーブル形式** (`[plugins.versions]` 1 つ + `go = { pin = "1.23.4" }` 形式の行) に変更しました。読み込みは subsection 形式 (`[plugins.versions.<id>]`) と inline 形式の両方を受理しますが、`cocoon plugin pin --write` は legacy subsection が残っている workspace.toml に対しては実行を拒否します (`<id> = { pin = "..." }` 形式に書き換えてから再実行してください)。既存ファイルは引き続きロードされ、`cocoon init` または `cocoon plugin pin --write` で再生成するとマイグレーションされます。
- **BREAKING**: `plugin.toml` の `[metadata]` に必須フィールド `url` を追加し、description から `... (https://...)` 形式の URL 埋め込みを廃止しました。`cocoon init` の version_capable プラグイン向けバージョン入力プロンプトは、各プラグインの上流 URL を説明文の直下に独立した行で表示するようになり、ユーザーは入力前にその URL を開いて有効なバージョン文字列を確認できます。`cocoon plugin show` には `url:` 行が追加され、`cocoon plugin list` には `URL` 列が追加されます。`cocoon plugin scaffold` には `--url` フラグ (`--non-interactive` 時は必須) と対話式 URL 入力プロンプトが追加され、`--description` に URL を括弧で含める必要はなくなりました。`~/.cocoon/plugins/` または `<project>/.cocoon/plugins/` の自作プラグインは `[metadata]` に `url = "https://..."` を追加する必要があります (未設定だと `url must not be empty` で読み込み失敗)。

### 削除

- **BREAKING**: `custom-ps1` プラグインを撤去しました。`starship` が bash / zsh / fish すべてで `custom-ps1` 相当のプロンプト機能を 1 つの宣言的設定でカバーするため (bash 専用だった `custom-ps1` を維持する理由が無くなった)。`[plugins].enable = ["custom-ps1"]` を残したワークスペースは `unknown plugin` で validation エラーになります。`"starship"` に置き換える (または該当エントリを削除する) して `cocoon gen` を再実行してください。

## [0.3.0] - 2026-05-13

### 追加

- `cocoon init` に **ポートフォワード入力プロンプト** を追加。ユーザーが値を入力した場合のみアクティブな `[ports]` ブロックを書き出し、空 Enter で見送るとコメントアウト済みの `# [ports]` 雛形だけが残る (後から有効化できるようにセクション自体は発見可能なまま)。非対話パス用に `--ports <values>` フラグも追加。受理する short-form は `[ports].forward` の全形式 — コンテナ単独 (`3000`)、host:container (`8000:8000`)、範囲 (`3000-3005:3000-3005` / `9090-9091:8080-8081`)、IPv4/IPv6 バインド (`127.0.0.1:8001:8001`、`[::1]:80:80`)、プロトコル (`6060:6060/udp`) — を `cocoon gen` と同じ regex + 数値範囲 + IP リテラル検証で受理するので、init が通した文字列は必ず gen でも通る。対話プロンプトでの拒否メッセージは i18n catalog 経由で出るので、`LANG=ja_*` 環境では日本語で表示される。`--ports` フラグの usage error は他フラグ (`--service-name` / `--image` / `--username` 等) と同じく英語のままで一貫させている。
- CLI 出力をセマンティック色分け: エラーは赤、警告は黄、成功メッセージ (ファイル生成・更新完了など) は緑、お知らせ (アップデート通知など) はシアン、見出し・ラベルは太字、`cocoon self-update` の `downloading ...` 進捗行は dim 表示。対象は `cocoon` 自体のエラー出口・`cocoon gen` / `cocoon init` / `cocoon self-update` / `cocoon plugin show|pin`・ブートストラップ用 `install.sh`・プラグイン install スクリプト (`internal/plugin/catalog/*/install.sh`)。`NO_COLOR` (https://no-color.org) と `FORCE_COLOR` をサポートし、stderr が TTY でない場合は自動で色を抑制します。
- アップデート通知: `cocoon <cmd>` 実行時に GitHub Releases を 1 日 1 回チェックし、新しいリリースがあればシアン色の 1 行通知を stderr に出力します (`A new version vX.Y.Z is available (current: vA.B.C). Run \`cocoon self-update\` to upgrade.`)。結果は `~/.cache/cocoon/update_check.json` にキャッシュ (`$XDG_CACHE_HOME` を尊重)。`cocoon version` / `cocoon self-update` / `cocoon help` / `--version` 実行時、stderr が TTY でない時、`COCOON_NO_UPDATE_CHECK=1` 設定時、およびネットワーク / キャッシュ I/O のエラー時はスキップ (silent fail) なので、通知のためにコマンド本体が止まることはありません。
- `cocoon gen` が `[home_files].files` の各エントリをホスト側 (`~/<rel>`) で mode `0600` の空ファイルとして自動 touch するようになった (idempotent — 既存ファイルは触らず、シンボリックリンクは尊重、既存ディレクトリは `rm -rf <path>` を案内するエラーになる)。併せて生成された `devcontainer.json` の `initializeCommand` でも同等の touch が走るので、VS Code「Reopen in Container」ユーザーは `cocoon gen` を介さなくても準備される。これまでファイル不在のまま `docker compose up` すると Docker が bind source を空ディレクトリとして自動作成してしまい、ファイル前提のリーダーが silent failure を起こしていた問題を解消する。
- `cocoon gen` の終了時に `[home_files]` の各ファイルを `~/<rel>` 形式で列挙する「Host files for [home_files]:」notice を表示。VS Code Dev Containers を経由しない (compose 直叩きの) 開発者が `docker compose up` 前にホスト側のファイル存在を確認できるようにする。
- `cocoon gen` をコンテナ内 (`/.dockerenv` 存在) で実行し、かつ `[home_files]` が非空の場合に stderr に警告を出す。compose のソースは `${HOME:?…}` 補間なので後から `docker compose up` をホストで実行すれば bind 自体は解決されるが、touch はコンテナ内 HOME に対して走ってしまう点を明示する (gen 自体は続行)。
- `[container].image` の選択肢を従来の `ubuntu` / `debian` に加えて 5 種の言語ランタイム公式イメージに拡張: `node` (`26-bookworm-slim` / `24-bookworm-slim` / `22-bookworm-slim`)、`python` (`3.14-slim-bookworm` / `3.13-slim-bookworm` / `3.12-slim-bookworm`)、`golang` (`1.26.3-bookworm` / `1.26-bookworm` / `1.25-bookworm` / `1.24-bookworm`)、`rust` (`1.95-bookworm` / `1.94-bookworm` / `1.93-bookworm`)、`denoland/deno` (`debian-2.7.14` / `debian-2.6.10` / `debian-2.5.7`)。すべて Debian (bookworm) ベース。image id は DockerHub の **正式名称** をそのまま記述します (`go` ではなく `golang`、deno は vendor namespace 込みで `denoland/deno`) — workspace.toml だけ見れば FROM 行が一意に決まり、cocoon 側のエイリアス解決は不要。言語ランタイムイメージを選ぶと apt インストール 1 ステップを省ける代わりに FROM レイヤーが少し大きくなります。
- `cocoon init` のインタラクティブピッカーが 7 種のイメージを提示し、選んだイメージごとの推奨候補がバージョン選択肢として並びます。非対話パスは `--image <id>` / `--image-version <tag>`。
- `cocoon init` のバージョン選択を、推奨候補を Tab キーで循環できる **1 画面テキスト入力** に変更。候補を Tab で送るか、任意の正しい形式のタグを直接入力できます (例: `golang:1.26.4-bookworm` を新パッチ公開日にすぐ pin)。非対話パス (`--image-version <tag>`) も同様の集合を受理。

### 変更

- 生成 `devcontainer.json` の `initializeCommand` が `${HOME:?…}` 由来のパスを全て二重引用符で quote し、`dirname --` を使うようになった。これにより `$HOME` がスペースを含むホスト (macOS の `/Users/Jane Doe` 等) で word-split しない。既存の `[certificates]` mkdir ステップと、新規 `[home_files]` touch ステップの両方に適用される。既存の `.devcontainer/devcontainer.json` は `cocoon gen` で再生成してください。
- 生成 `docker-compose.yml` の `[home_files]` bind ソースが、`cocoon gen` 実行時の絶対パス展開から `${HOME:?HOME must be set on the host}/<rel>` 形式に変わった。gen を実行した環境と `docker compose up` を実行するホストが異なっていても compose が機能する (両者の `$HOME` が一致している必要が無くなる)。`HOME` 未設定のホストで up したときの failure mode は、サイレントな `/<rel>` 折りたたみではなく明確な shell エラーになる。既存の `.devcontainer/docker-compose.yml` は `cocoon gen` で再生成して新形式に追従させてください。
- `image_version` の whitelist 厳格チェックを撤廃。Docker タグ文字集合 (英数字・ドット・アンダースコア・ハイフン、スラッシュ / コロン禁止) の形式チェックだけに緩和し、上流レジストリが公開するパッチ・新マイナーをすぐに pin できるようにしました (例: `golang:1.26.4-bookworm`)。`SupportedImageVersions` は `cocoon init` の推奨候補と `docs/configuration.md` の推奨タグ表に位置づけが変わります。タグがレジストリに実在するかは `docker pull` (ビルド時) に委ねます。
- **BREAKING**: `[container].os` / `os_version` を `[container].image` / `image_version` にリネームしました。サポート対象が 2 種の Linux ディストロから 7 種のイメージに広がったため「OS」という名前は実態と乖離しています。マイグレーション: `os = "ubuntu"` → `image = "ubuntu"`、`os_version = "26.04"` → `image_version = "26.04"`。旧キーを残した `workspace.toml` を読み込ませると validator がリライト例入りの fail-fast エラーを出すので、ファイル単位で 1 回の置換で済みます。
- **BREAKING**: `cocoon init` の `--os` / `--os-version` フラグを `--image` / `--image-version` にリネーム。旧フラグの alias は提供しないので、CI で旧フラグを固定指定している箇所は置換が必要です。
- **BREAKING**: 生成 `Dockerfile` の ARG 名 `OS_IMAGE` / `OS_VERSION` と、対応する `.env` / docker-compose の補間キーを `IMAGE` / `IMAGE_VERSION` にリネーム。cocoon の外側でこれらに依存していた箇所 (例: `docker build --build-arg OS_IMAGE=...` を自分で叩いていた等) は併せて更新してください。
- `image = "golang"` と `[plugins].enable = ["go"]` の併用、および `image = "rust"` と `[plugins].enable = ["rust"]` の併用を validation エラーで reject するようにしました。go プラグインは `/usr/local/go` をベースイメージごと上書き、rust プラグインは `$HOME/.cargo/bin` を PATH 先頭に挿入してベースを死蔵させるため、両方有効にすると docker-build 時間を浪費するだけで実行時の挙動は変わりません。ベースイメージかプラグインのどちらか一方を選んでください。エラーメッセージには具体的なリライト案が含まれます。

### 修正

- `cocoon gen` で生成される `.devcontainer/devcontainer.json` に `"forwardPorts": [3000]` がハードコードで付与される問題を修正。`[ports]` も `[devcontainer].forward_ports` 上書きも未設定のときは `forwardPorts` キー自体を出力しないようになり、VS Code の「Ports」パネルや `docker compose ps` にユーザーが宣言していないポートが現れない。`[ports].forward = [...]` か `cocoon init --ports ...` で明示的に opt-in したワークスペースには影響なし。既存の `.devcontainer/devcontainer.json` は `cocoon gen` で再生成して新挙動に追従させてください。
- UDP 限定の `[ports].forward` エントリ (short form `"6060:6060/udp"` や long form `{ target = 53, protocol = "udp" }`) を `devcontainer.json` の `forwardPorts` に流さないように修正。VS Code の port tunnel は TCP only なので UDP を登録しても Ports パネルに出るだけで実際には転送できない。今後はレンジや mode=host のスキップと同じ形式で 1 行 warning を出してスキップする (compose 側の `ports:` には引き続き UDP として正しく反映される)。
- アップデート通知の TTL チェックがウォールクロック巻き戻しで固まる問題を修正。キャッシュの `checked_at` が「未来」に書き込まれていた場合 (タイムゾーン変更を跨いでサスペンドした、NTP で進んだ時計を補正した、など) も stale 扱いとして次回呼び出し時に再フェッチするようになった。従来は未来タイムスタンプを尊重してしまい、ウォールクロックが追いつくまで通知が抑止されていた。
- GitHub 不達時にサブコマンドが最大 30 秒待たされる問題を修正。アップデート通知のネットワーク呼び出しに 2 秒のタイムアウトを設けたので (従来は `release.DefaultTimeout` の 30 秒)、`api.github.com` が応答しない場合でも数秒で silent-fail 経路に落ちて本来のサブコマンドが動き出す。
- **セキュリティ**: `[home_files].files` の各パスセグメントを `[A-Za-z0-9._/-]+` に制限するようにした。シェル特殊文字 (`$`、バッククォート、`;`、`&`、`|`、`<`、`>`、`*`、`?`、`!`、引用符、バックスラッシュ、空白) は validation 時に reject されるため、repo 提供の `workspace.toml` から生成された `initializeCommand` 経由でホストシェルへコマンド注入される経路を塞ぐ。従来から conventional な dotfile 名のみを使っていたワークスペースには影響しない。

### 削除

- **BREAKING**: `[container].ubuntu_version` (v0.2.0 で deprecated 化していたもの) を撤去。strict TOML パーサが unknown key として reject します。マイグレーション: `image = "ubuntu"` / `image_version = "..."` に直接リライト (v0.2 の `ubuntu_version` → `os` / `os_version` の中間段階は経由しなくて構いません)。

## [0.2.0] - 2026-05-11

### 追加

- `install.sh` に `COCOON_API_BASE` (デフォルト `https://api.github.com`) と `COCOON_RELEASE_BASE` (デフォルト `https://github.com`) の上書き入力を追加。GitHub Enterprise Server やローカルミラー経由でも公開インストーラを利用可能。
- `install.sh` に `COCOON_API_TOKEN` を追加。GitHub API 呼び出し時に Bearer トークンとして送られ、anonymous 60 req/hour のレート制限 (CI のように同一 runner IP プールを共有する環境でぶつかりやすい) を回避できる。`curl ... | sh` を 1 回だけ手で叩く一般ユーザは設定不要。
- コンテナ内 `/home/<user>/.cocoon` に named volume `cocoon` をマウント。ユーザー個人のシェル設定をコンテナリビルドを跨いで永続化する。コンテナの rc (bash / zsh / fish) が起動時に `~/.cocoon/.shellrc` (fish は `~/.cocoon/.shellrc.fish`) を自動 source するので、コンテナ内で編集した内容は `docker compose down && up --build` を跨いでも残る (リセットは `down -v` のみ)。
- `cocoon init --plugin-versions=<id>=<ref>,...` を追加。1 コマンドで `[plugins] enable` と `[plugins.versions]` の両方を出力できる。各 `<id>` は `--plugins` に含まれ、かつ `version_capable` である必要があり、重複は不可。これまで `cocoon plugin pin` の出力を手で貼り付けていた運用を置き換える。
- `cocoon plugin pin --write` を追加。`workspace.toml` の `[plugins.versions.<id>]` ブロックを直接挿入・置換する。行ベースのミューテータが対象ブロック外のコメント・空行を保持するため、既存ファイルを安全に編集できる。`--write` 無しの stdout-only 動作はデフォルトのまま。`[plugins.versions]` 直下に任意の key 代入 (例: `<id> = "..."` や `<id> = { ... }`) がある場合は重複ブロック追加を避けるため usage error で停止する。
- `workspace.toml` に新セクション `[certificates]` を追加。`enable = true` のとき `~/.cocoon/certs/*.crt` を build 時にコンテナイメージへ自動取り込み、デフォルト (セクション不在 or `enable = false`) では生成された `Dockerfile` / `docker-compose.yml` / `devcontainer.json` に **cert 関連の配線が一切乗らない** (additional_contexts も RUN --mount=type=bind も initializeCommand も SSL_CERT_FILE ENV も出ない)。社内 CA を扱わないチームは corp-CA 機構ゼロの成果物を commit できる。有効時は compose 側で `additional_contexts: cocoon_user_certs: ${HOME:?…}/.cocoon/certs`、Dockerfile 側で `RUN --mount=type=bind,from=cocoon_user_certs …` により他の apt 操作より前に trust store へマージするため、TLS インターセプトが行われる（プライベート CA を要する）ネットワーク環境でも build が成立する。
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

### 削除

- **BREAKING**: `cocoon plugin remove` サブコマンドを削除。実装は `os.RemoveAll` の薄いラッパーで `rm -rf <overlay>` と完全に等価だった。移行手順: `cocoon plugin remove <id> --scope user` を `rm -rf ~/.cocoon/plugins/<id>` (project スコープなら `<workspace>/.cocoon/plugins/<id>`) に置き換える。
- **BREAKING**: `cocoon plugin add` サブコマンドを削除。実装は埋め込みプラグインを書き込み可能な overlay にコピーするだけだったが、「add」という名前が「enable」と誤読されやすかった (LayeredFS のおかげで `[plugins].enable` に id を並べるだけで埋め込みカタログは有効化される)。移行手順: 埋め込みプラグインを使うだけなら `workspace.toml` の `[plugins].enable` に id を列挙する。改変したい場合のサポート手順は `cocoon plugin scaffold <new-id>` で新規 id を作りロジックを移植すること。cocoon リポジトリのクローン（または GitHub Release のソース tarball）を持っているなら `cp -r internal/plugin/catalog/<id> ~/.cocoon/plugins/<id>/` が近道だが、単体バイナリでインストールした場合は embedded ソースがディスク上に存在しないためこの近道は使えない。`add` が呼んでいた `plugin.Materialize` ヘルパも併せて削除。
- **BREAKING**: `cocoon config` 名詞グループを削除 (`get` / `list` / `volumes` / `plugin-get` / `plugin-list` / `plugin-volumes` / `plugins-table` / `validate-workspace` / `validate-plugins` / `has-section` / `list-sidecars` / `dump-devcontainer` / `dump-repositories` / `repositories` / `format-repositories`)。これらは v0.1.0 で全廃された bash entry-point スクリプト用の低レベル TOML アクセサで、cocoon 内部では既に未使用。`cocoon config` でスクレイプしていた外部スクリプトは専用の TOML パーサ (`tomlq` / `taplo` や小さな Go / Python ヘルパ) に切り替えてください。

## [0.1.0] - 2026-05-09

### 追加

- `cocoon init` を追加。サービス名・ユーザー名・OS・OS バージョン・ログインシェル・マウント範囲・devcontainer 出力切替・エイリアスバンドル・apt カテゴリ・プラグインを対話で選んで `workspace.toml` を生成。
- 非対話用フラグ (`--yes`, `--service-name`, `--username`, `--os`, `--os-version`, `--shell`, `--mount-root`, `--devcontainer`, `--no-devcontainer`, `--apt-categories`, `--plugins`, `--alias-bundles`, `--force`) を追加。CI やスクリプトから TTY なしで `cocoon init` を駆動可能。
- 生成 `workspace.toml` にローカライズされたインラインコメントとコメントアウト済セクション雛形を追加し、ファイル内で機能を発見できるようにする。
- `cocoon gen` を追加。`workspace.toml` から `.devcontainer/{Dockerfile, docker-compose.yml, docker-entrypoint.sh, .env, devcontainer.json}` を生成。
- `cocoon plugin` 名詞グループを追加 (`list` / `show` / `add` / `remove` / `pin` / `scaffold` の 6 サブコマンド)。プラグインの埋め込みカタログと `LayeredFS` (project > user > embedded) による上書きをサポート。
- `cocoon config` 名詞グループを追加 (`get` / `list` / `volumes` / `plugin-get` / `plugin-list` / `plugin-volumes` / `plugins-table` / `validate-workspace` / `validate-plugins` / `has-section` / `list-sidecars` / `dump-devcontainer` / `dump-repositories` / `repositories` / `format-repositories`)。
- `cocoon self-update` を追加。GitHub リリースからのダウンロード、SHA256 検証、atomic rename による差し替えに対応。
- `cocoon version` を追加。
- `cocoon init` で選択できる apt カテゴリ 10 種 (`text-editors`, `vcs`, `utilities`, `compression`, `build`, `search`, `network`, `monitoring`, `python3`, `json-yaml`) を追加。
- `cocoon init` で選択できるエイリアスバンドル 3 種 (`git`, `ls`, `docker`) を追加。`[container.shell] aliases` にマージされる。
- Dockerfile heredoc によるシェル rc 注入を追加。`[container.shell] env` と `aliases` がイメージビルド時に `~/.bashrc` / `~/.zshrc` / `~/.config/fish/config.fish` へ直接反映される。
- `COMPOSE_PROJECT_NAME` をプロジェクトディレクトリの basename から導出するように変更。docker compose の namespace がホストディレクトリと一致する。
- 国際化 (英語 / 日本語) カタログを追加。CLI プロンプト・エラーメッセージ・`workspace.toml` インラインコメントすべてを `WORKSPACE_LANG` / `LC_ALL` / `LC_MESSAGES` / `LANG` で切替可能。

[Unreleased]: https://github.com/sukekyo26/cocoon/compare/v0.13.0...HEAD
[0.13.0]: https://github.com/sukekyo26/cocoon/compare/v0.12.0...v0.13.0
[0.12.0]: https://github.com/sukekyo26/cocoon/compare/v0.11.0...v0.12.0
[0.11.0]: https://github.com/sukekyo26/cocoon/compare/v0.10.2...v0.11.0
[0.10.2]: https://github.com/sukekyo26/cocoon/compare/v0.10.1...v0.10.2
[0.10.1]: https://github.com/sukekyo26/cocoon/compare/v0.10.0...v0.10.1
[0.10.0]: https://github.com/sukekyo26/cocoon/compare/v0.9.2...v0.10.0
[0.9.2]: https://github.com/sukekyo26/cocoon/compare/v0.9.1...v0.9.2
[0.9.1]: https://github.com/sukekyo26/cocoon/compare/v0.9.0...v0.9.1
[0.9.0]: https://github.com/sukekyo26/cocoon/compare/v0.8.0...v0.9.0
[0.8.0]: https://github.com/sukekyo26/cocoon/compare/v0.7.6...v0.8.0
[0.7.6]: https://github.com/sukekyo26/cocoon/compare/v0.7.5...v0.7.6
[0.7.5]: https://github.com/sukekyo26/cocoon/compare/v0.7.4...v0.7.5
[0.7.4]: https://github.com/sukekyo26/cocoon/compare/v0.7.3...v0.7.4
[0.7.3]: https://github.com/sukekyo26/cocoon/compare/v0.7.2...v0.7.3
[0.7.2]: https://github.com/sukekyo26/cocoon/compare/v0.7.1...v0.7.2
[0.7.1]: https://github.com/sukekyo26/cocoon/compare/v0.7.0...v0.7.1
[0.7.0]: https://github.com/sukekyo26/cocoon/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/sukekyo26/cocoon/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/sukekyo26/cocoon/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/sukekyo26/cocoon/compare/v0.3.1...v0.4.0
[0.3.1]: https://github.com/sukekyo26/cocoon/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/sukekyo26/cocoon/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/sukekyo26/cocoon/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/sukekyo26/cocoon/releases/tag/v0.1.0
