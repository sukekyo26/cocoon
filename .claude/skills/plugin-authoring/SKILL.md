---
name: plugin-authoring
description: 'cocoon プラグインの新規作成・改修ガイド。plugins/<id>/{plugin.toml, install.<category>.sh} の二段構成、$PIN/$CHECKSUM_* 入力規約、method 名カテゴリ規約、テスト・CI 登録手順を扱う。Triggers: "create plugin", "new plugin", "add plugin", "plugin.toml", "install.sh", "install.methods", "version_capable", "プラグイン作成", "プラグイン追加", "新しいプラグイン".'
---

# plugin-authoring

cocoon プラグインの新規追加・改修をエージェントが進めるための作業手順。

**仕様の単一ソースは [`docs/plugins.ja.md`](../../../docs/plugins.ja.md)
（英語版: [`docs/plugins.md`](../../../docs/plugins.md)）**。
`plugin.toml` の全フィールド、`install.<category>.sh` / `install_user.sh`
の使い分け、渡される環境変数、`$PIN` / `$CHECKSUM_*` の契約、
トラブルシューティングはすべてそちらにまとまっている。本スキルは
agent 向けの作業フローに絞る。

## method 名のカテゴリ規約 — まず読む

`[install.methods.<name>]` の `<name>` と対応する `install.<name>.sh`
の `<name>` は **catalog 共通の 4 カテゴリ語彙** から選ぶ。plugin 固有の
vendor 名（例: `gh-cli`、`bun-sh`、`astral`）は **付けない** — 異なる
プラグイン間で語彙がずれて、ユーザーが workspace.toml の
`[plugins.methods]` を書くときに plugin.toml を毎回参照する羽目になる。

| カテゴリ | 選ぶ基準 |
|:---|:---|
| `binary`    | 単一 ELF binary を `/usr/local/bin` 等に配置するだけ。tar/zip から「1 個だけ取り出す」も該当 (kubectl, helm, terraform, shellcheck) |
| `installer` | vendor の curl-to-bash インストーラ (例: `bun.sh/install`, `sh.rustup.rs`) を pipe する。失敗ドメインは vendor 固有 |
| `apt`       | apt repository 登録 or `.deb` 直 install (docker-cli, github-cli, google-chrome) |
| `archive`   | 複数ファイルの tar/zip を展開してディレクトリツリーを作る (bin + lib + share 等)、または archive 内 installer 実行 (go, node, zig, nerd-fonts, aws-cli) |

ルール:

- method を 1 つしか持たないプラグインも、必ずこの 4 つのうち適切な
  ものを選んで `[install.methods.<category>]` を 1 つ宣言する。
- `install.sh` という名前は **サポートされない** (loader が reject)。
  必ず `install.<category>.sh` で配置する。
- method を **2 つ以上** 宣言すると `cocoon init` の対話 UI に method
  ピッカーが出る。1 つだけならピッカーは出ない (default_method が
  サイレントに使われる)。
- validator (`^[a-z][a-z0-9_-]*$`) は 4 つ以外の名前も技術的に許容するが、
  **catalog の plugin は必ずこの 4 つを使う**。レビューで指摘する対象。

### `install_user.sh` は method 非依存

`install_user.sh` は plugin 単位の hook で、method ごとに分けない
（`install_user.<method>.sh` というファイル名は無い）。複数 method を
持つプラグインでも `install_user.sh` は 1 本だけ置き、どの method が
選ばれても実行後に走る。マトリクス膨張を避ける規約。

## ステップ 1: scaffold で雛形を作る

```bash
# 対話モード（推奨。テンプレート選択・install_user.sh の要否などを聞かれる）
cocoon plugin scaffold my-tool

# 非対話モード（CI / 自動化）
cocoon plugin scaffold my-tool \
  --template binary --version-capable --requires-root \
  --name "My Tool" \
  --description "Short description" \
  --url "https://github.com/owner/repo" \
  --non-interactive
```

`--template` は上記 4 カテゴリ語彙と 1 対 1 対応 (`installer` /
`binary` / `apt` / `archive`)。指定したカテゴリで `install.<category>.sh`
が生成され、`plugin.toml` には `[install.methods.<category>]` と
`default_method = "<category>"` が自動挿入される。

`--with-install-user` を聞かれたときは、対話プロンプトの説明文を読んで判断する
（`docs/plugins.ja.md` §5 の判断マトリクスを反映している）。迷ったら **付けない**。
ほとんどのプラグインは `install.<category>.sh` だけで完結する。

## ステップ 2: scaffold が埋めない部分を手書きする

`docs/plugins.ja.md` §4 / §6 / §7 を参照しながら次を埋める:

- `[apt].packages` — 必要な OS パッケージ
- `[install.env]` — `ENV` として出力したい変数
- `[install.build_args]` — Dockerfile の `ARG` 経由で受けたいビルド時変数名（catalog では現在未使用; 自作プラグイン向けの機構）
- `install.<category>.sh` 本体 — 上流 URL / 依存ロジック / cleanup
- 必要なら `install_user.sh` — rc 編集や `<tool> init` 系のユーザー権限処理

## `[version.source]` — `cocoon lock` 対応の宣言

`version_capable = true` のプラグインに `[version.source]` を書くと、`cocoon lock`
が「最新バージョンの発見」と「arch ごとの checksum の取得」をネットワーク越しに
行えるようになる（解決結果は workspace ルートの `cocoon.lock` に記録され、`cocoon gen`
はそれをオフラインで消費する）。`[version.source]` を **書かない** プラグインは
**exact-only** — `cocoon lock` で `"latest"` を解決できず、`[plugins].enable` 配列に
インライン（例: `"aws-cli=2.34.48"`）で厳密バージョンを pin する必要がある。

`[version.source.latest]`（最新の探し方）と `[version.source.checksum]`（hash の取り方）の
2 ブロックで構成し、URL / asset 名に `${arch}` を使うときだけ `[version.source.arch]`
で arch トークンの対応表を足す。

### `[version.source.latest]` — `type` 別

| `type` | 必須キー | 任意キー | 動作 |
|:---|:---|:---|:---|
| `github-release` | `repo = "owner/name"` | `strip_prefix` | resolver が `repo` から GitHub の `releases/latest` API を叩く（`plugin.toml` に `api.github.com` は書かない）。タグ先頭に `v` が付くなら `strip_prefix = "v"` |
| `text` | `url` | `strip_prefix` | レスポンスの **1 行目** をバージョンとして読む |
| `json-field` | `url`, `field = "dotted.path"` | `strip_prefix` | JSON を取得し dotted path のフィールドを読む |
| `tab` | `url` | `lts_only` | Node の `dist/index.tab` 形式。`lts_only = true` で LTS のみ対象 |

### `[version.source.checksum]` — `type` 別

| `type` | 必須キー | 任意キー | 動作 |
|:---|:---|:---|:---|
| `sidecar` | `asset_url` | `suffix`（既定 `.sha256`） | `asset_url + suffix` を取得。body は素の hash 1 個 |
| `shasums-file` | `manifest_url`, `asset_name` | — | `manifest_url`（`SHA256SUMS` 等）を取得し、`asset_name` に一致する行の hash を取る |
| `none` | — | — | fetch 可能な arch ごとの hash が無いプラグイン（pgp 検証、または hash 非公開の `\| bash` インストーラ）。checksum は記録されない |

### `${version}` / `${arch}` テンプレート

`url` / `asset_url` / `manifest_url` / `asset_name` では次を展開できる:

- `${version}` — 解決後のクリーンなバージョン（先頭の `v` は付かない）。上流タグが
  `v` を持つ URL には **リテラルで** `v${version}` と書く。
- `${arch}` — `[version.source.arch]` の対応表で置換される arch トークン
  （例: node→`x64`、just→`x86_64`）。`amd64` / `arm64` の 2 キーを宣言する。
  `${arch}` を一切使わないなら `[version.source.arch]` ブロックは不要。

### カタログ実例

`text` + `sidecar`（`internal/plugin/catalog/go/plugin.toml`）:

```toml
[version.source.latest]
type = "text"
url = "https://go.dev/VERSION?m=text"
strip_prefix = "go"

[version.source.checksum]
type = "sidecar"
asset_url = "https://dl.google.com/go/go${version}.linux-${arch}.tar.gz"
suffix = ".sha256"

[version.source.arch]
amd64 = "amd64"
arm64 = "arm64"
```

`tab` + `shasums-file`（`internal/plugin/catalog/node/plugin.toml`）:

```toml
[version.source.latest]
type = "tab"
url = "https://nodejs.org/dist/index.tab"
lts_only = true

[version.source.checksum]
type = "shasums-file"
manifest_url = "https://nodejs.org/dist/v${version}/SHASUMS256.txt"
asset_name = "node-v${version}-linux-${arch}.tar.xz"

[version.source.arch]
amd64 = "x64"
arm64 = "arm64"
```

`github-release` + `shasums-file`（`internal/plugin/catalog/just/plugin.toml`）:

```toml
[version.source.latest]
type = "github-release"
repo = "casey/just"

[version.source.checksum]
type = "shasums-file"
manifest_url = "https://github.com/casey/just/releases/download/${version}/SHA256SUMS"
asset_name = "just-${version}-${arch}-unknown-linux-musl.tar.gz"

[version.source.arch]
amd64 = "x86_64"
arm64 = "aarch64"
```

`github-release` + `none`（`internal/plugin/catalog/uv/plugin.toml`、`installer` で per-arch hash を取らない）:

```toml
[version.source.latest]
type = "github-release"
repo = "astral-sh/uv"

[version.source.checksum]
type = "none"
```

exact-only（`[version.source]` を持てない上流）の参考: `aws-cli`（バージョン無し
ダウンロード alias）/ `android-sdk`（HTML スクレイプのビルド番号）/ `flutter`
（コミットハッシュをキーとするリリース）。これらは `[plugins].enable` 配列に
インライン（例: `"aws-cli=2.34.48"`）で必ず厳密 pin する。

## ステップ 3: 既存プラグインを参考にする

カテゴリ別の参照ポイント:

| カテゴリ | 参考プラグイン | 学べること |
|:---|:---|:---|
| `binary`    | `internal/plugin/catalog/kubectl/`     | `dl.k8s.io` 直 download、ARCH 切替、`sha256sum -c -` 検証 |
| `binary`    | `internal/plugin/catalog/starship/`    | tar から単一抽出 + `install_user.sh` で rc 編集 |
| `installer` | `internal/plugin/catalog/bun/`         | `bun.sh/install \| bash` + `[install.env]` で PATH 出力 |
| `installer` | `internal/plugin/catalog/proto/`       | `moonrepo.dev/install/proto.sh` + `PROTO_HOME` |
| `apt`       | `internal/plugin/catalog/docker-cli/`  | サードパーティ apt repo 追加（keyring + `sources.list.d`）からの install |
| `apt`       | `internal/plugin/catalog/google-chrome/` | `.deb` を `apt-get install` |
| `archive`   | `internal/plugin/catalog/go/`          | `version_capable`、ARCH 切替、`/usr/local/go` ツリー展開 |
| `archive`   | `internal/plugin/catalog/zig/`         | 上流 `index.json` 経由でアセット名解決 |
| `archive`   | `internal/plugin/catalog/aws-cli/`     | `verify = "pgp"`、同梱署名鍵で in-script PGP 検証（SHA256 非公開の上流向け） |

`copilot-cli` は同一プラグインが `installer` (gh.io) と `binary`
(GitHub Release) の **2 method** を提供する catalog 唯一の例。Zscaler 等で
vendor ドメインが落ちる環境向けの「代替経路」設計の参考になる。

## ステップ 4: 契約テストと e2e に登録する

新規プラグインは次を更新する。契約テスト spec は **必須** — 忘れると `just ci` が落ちる。

- **契約テスト spec 行（必須）** — `internal/plugin/contracts_test.go` の
  `TestPluginContracts` の `specs` スライスに 1 行追加する。無いと
  `plugin "<id>" has no contract spec in TestPluginContracts` で `just ci` が失敗する。
  `requiresRoot` / `versionCapable` / `verify` / `firstVolume` を `plugin.toml` と一致
  させ、`mustContain` で install スクリプトの要点（上流ドメイン・`sha256sum -c -`・
  `tlsv1.2`・`dpkg --print-architecture` 等）を pin、`mustNotContain` で
  `noPlaceholders` / `noApiNoJq` を否定する。
- **e2e** — プラグインは `e2e/docker-roundtrip.sh` が catalog から自動検出して全
  プリセットで enable するため、`.github/workflows/e2e.yml` の編集は不要。未 pin なら
  LATEST でテストされる（pin は必須ではない）。再現性のため version_capable は慣例的に
  `pin_entries` に `<id>=<version>` を追加する（pin できるのは version_capable のみ。
  非対応 id を入れると e2e が `not version_capable; it cannot be pinned` で fail-fast する）。
- **init スナップショット（手動追記が必要）** — e2e は catalog を自動検出するが、
  `internal/cli/init/cmd_snapshot_test.go` の `plugins-amd64-full` /
  `plugins-arm64-full` ケースの `--plugins` は**ハードコードされた手動ミラー**で、
  e2e full preset と整合させる必要がある。新規プラグインを `--plugins` に追記し
  （version_capable なら `--plugin-versions` にも `pin_entries` と同じ値で追記）、
  `just regen-snapshots` で golden（`testdata/init/plugins-*-full.workspace.toml`）を
  再生成する。arm64 非対応（`arm64-exclude.txt` 掲載）プラグインは amd64 リストのみに
  入れる。**この追記を怠ってもカタログとリストがドリフトするだけで `just ci` は落ちない**
  ので忘れやすい — 必ず手動で追記する。
- **arm64 非対応** — amd64 をハードコード／fail-fast するプラグインは
  `e2e/arm64-exclude.txt` に `<id>` を追加する（`TestArm64ExcludeIDsExist` が実在 id を
  ガード）。追加漏れは arm64 で壊れたまま素通りする。

## ステップ 5: ローカル検証

```bash
# 静的検査
shellcheck internal/plugin/catalog/<id>/install.<category>.sh

# strict TOML 検証 + 全 Go テスト
just ci
```

`internal/generate/dockerfile/plugins_test.go` 等の golden が差分になったら、
内容を確認した上で `just regen-snapshots` で更新する。

## ステップ 6: CHANGELOG を書く

`changelog` スキルの手順に従い、`CHANGELOG.md` と `docs/CHANGELOG.ja.md` の両方の `[Unreleased]` に追記する。

- プラグイン新規追加 → `### Added`
- 既存プラグインの挙動変更 → `### Changed`
- プラグイン削除 → `### Removed`

## 補足: 仕様で迷ったら

仕様面の質問（フィールドの意味、env 変数、heredoc collision、層解決、
カテゴリの選び方の具体例）は **すべて [`docs/plugins.ja.md`](../../../docs/plugins.ja.md) で答えが出る**
ので、このスキルの中で重複説明しない。
