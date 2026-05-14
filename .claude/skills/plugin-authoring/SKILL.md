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
- `[install.build_args]` — Dockerfile の `ARG` を経由して受けたい host 由来の値（例: `DOCKER_GID`）
- `install.<category>.sh` 本体 — 上流 URL / 依存ロジック / cleanup
- 必要なら `install_user.sh` — rc 編集や `<tool> init` 系のユーザー権限処理

## ステップ 3: 既存プラグインを参考にする

カテゴリ別の参照ポイント:

| カテゴリ | 参考プラグイン | 学べること |
|:---|:---|:---|
| `binary`    | `internal/plugin/catalog/kubectl/`     | `dl.k8s.io` 直 download、ARCH 切替、`sha256sum -c -` 検証 |
| `binary`    | `internal/plugin/catalog/starship/`    | tar から単一抽出 + `install_user.sh` で rc 編集 |
| `installer` | `internal/plugin/catalog/bun/`         | `bun.sh/install \| bash` + `[install.env]` で PATH 出力 |
| `installer` | `internal/plugin/catalog/proto/`       | `moonrepo.dev/install/proto.sh` + `PROTO_HOME` |
| `apt`       | `internal/plugin/catalog/docker-cli/`  | apt source 追加、`build_args = ["DOCKER_GID"]` |
| `apt`       | `internal/plugin/catalog/google-chrome/` | `.deb` を `apt-get install` |
| `archive`   | `internal/plugin/catalog/go/`          | `version_capable`、ARCH 切替、`/usr/local/go` ツリー展開 |
| `archive`   | `internal/plugin/catalog/zig/`         | 上流 `index.json` 経由でアセット名解決 |

`copilot-cli` は同一プラグインが `installer` (gh.io) と `binary`
(GitHub Release) の **2 method** を提供する catalog 唯一の例。Zscaler 等で
vendor ドメインが落ちる環境向けの「代替経路」設計の参考になる。

## ステップ 4: ローカル検証

```bash
# 静的検査
shellcheck internal/plugin/catalog/<id>/install.<category>.sh

# strict TOML 検証 + 全 Go テスト
just ci
```

`internal/generate/dockerfile/plugins_test.go` 等の golden が差分になったら、
内容を確認した上で `just regen-snapshots` で更新する。

## ステップ 5: CI に組み込む

`.github/workflows/e2e.yml` の `docker-roundtrip` ジョブのプラグインリストに
`<id>` を追加する。追加しないと CI で実ビルド検証が走らない。

## ステップ 6: CHANGELOG を書く

`changelog` スキルの手順に従い、`CHANGELOG.md` と `docs/CHANGELOG.ja.md` の両方の `[Unreleased]` に追記する。

- プラグイン新規追加 → `### Added`
- 既存プラグインの挙動変更 → `### Changed`
- プラグイン削除 → `### Removed`

## 補足: 仕様で迷ったら

仕様面の質問（フィールドの意味、env 変数、heredoc collision、層解決、
カテゴリの選び方の具体例）は **すべて [`docs/plugins.ja.md`](../../../docs/plugins.ja.md) で答えが出る**
ので、このスキルの中で重複説明しない。
