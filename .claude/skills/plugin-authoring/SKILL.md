---
name: plugin-authoring
description: 'cocoon プラグインの新規作成・改修ガイド。plugins/<id>/{plugin.toml, install.sh} の二段構成、$PIN/$CHECKSUM_* 入力規約、テスト・CI 登録手順を扱う。Triggers: "create plugin", "new plugin", "add plugin", "plugin.toml", "install.sh", "version_capable", "プラグイン作成", "プラグイン追加", "新しいプラグイン".'
---

# plugin-authoring

cocoon プラグインの新規追加・改修をエージェントが進めるための作業手順。

**仕様の単一ソースは [`docs/plugins.ja.md`](../../../docs/plugins.ja.md)
（英語版: [`docs/plugins.md`](../../../docs/plugins.md)）**。
`plugin.toml` の全フィールド、`install.sh` / `install_user.sh` の使い分け、
渡される環境変数、`$PIN` / `$CHECKSUM_*` の契約、トラブルシューティングは
すべてそちらにまとまっている。本スキルは agent 向けの作業フローに絞る。

## ステップ 1: scaffold で雛形を作る

```bash
# 対話モード（推奨。テンプレート選択・install_user.sh の要否などを聞かれる）
cocoon plugin scaffold my-tool

# 非対話モード（CI / 自動化）
cocoon plugin scaffold my-tool \
  --template tarball --version-capable --requires-root \
  --name "My Tool" \
  --description "Short description" \
  --url "https://github.com/owner/repo" \
  --non-interactive
```

`--template` の指針:

- `curl-pipe` — 上流が `curl ... | bash` 形式の公式インストーラ（uv, proto 系）
- `tarball` — GitHub Release tarball + sha256 検証（starship, go, lazygit 系）。`--version-capable` 必須
- `generic` — apt / .deb / 自作スクリプト用の最小骨組み

`--with-install-user` を聞かれたときは、対話プロンプトの説明文を読んで判断する
（`docs/plugins.ja.md` §5 の判断マトリクスを反映している）。迷ったら **付けない**。
ほとんどのプラグインは `install.sh` だけで完結する。

## ステップ 2: scaffold が埋めない部分を手書きする

`docs/plugins.ja.md` §4 / §6 / §7 を参照しながら次を埋める:

- `[apt].packages` — 必要な OS パッケージ
- `[install.env]` — `ENV` として出力したい変数
- `[install.build_args]` — Dockerfile の `ARG` を経由して受けたい host 由来の値（例: `DOCKER_GID`）
- `install.sh` 本体 — 上流 URL / 依存ロジック / cleanup
- 必要なら `install_user.sh` — rc 編集や `<tool> init` 系のユーザー権限処理

## ステップ 3: 既存プラグインを参考にする

| 例 | 学べること |
|:---|:---|
| `internal/plugin/catalog/go/`         | `version_capable`, ARCH 切替, 公式 tar の取得, `[install.env]` |
| `internal/plugin/catalog/docker-cli/` | `build_args = ["DOCKER_GID"]`, GID マッピング |
| `internal/plugin/catalog/proto/`      | `[install.env]` で `PROTO_HOME` / `PATH` 出力 |
| `internal/plugin/catalog/starship/`   | `install_user.sh` で `~/.bashrc` を編集 |
| `internal/plugin/catalog/zig/`        | 上流 `index.json` 経由でアセット名を解決 |
| `internal/plugin/catalog/lazygit/`    | GitHub API を使わない latest 取得 |

## ステップ 4: ローカル検証

```bash
# 静的検査
shellcheck internal/plugin/catalog/<id>/install.sh

# strict TOML 検証 + 全 Go テスト
just ci
```

`internal/generate/dockerfile/plugins_test.go` 等の golden が差分になったら、
内容を確認した上で `just regen-snapshots` で更新する。

## ステップ 5: CI に組み込む

`.github/workflows/ci.yml` の `docker-build` ジョブのプラグインリストに `<id>` を追加する。
追加しないと CI で実ビルド検証が走らない。

## ステップ 6: CHANGELOG を書く

`changelog` スキルの手順に従い、`CHANGELOG.md` と `docs/CHANGELOG.ja.md` の両方の `[Unreleased]` に追記する。

- プラグイン新規追加 → `### Added`
- 既存プラグインの挙動変更 → `### Changed`
- プラグイン削除 → `### Removed`

## 補足: 仕様で迷ったら

仕様面の質問（フィールドの意味、env 変数、heredoc collision、層解決）は
**すべて [`docs/plugins.ja.md`](../../../docs/plugins.ja.md) で答えが出る**ので、
このスキルの中で重複説明しない。
