# cocoon

[![Go CI](https://github.com/sukekyo26/cocoon/actions/workflows/go-ci.yml/badge.svg)](https://github.com/sukekyo26/cocoon/actions/workflows/go-ci.yml)
[![E2E](https://github.com/sukekyo26/cocoon/actions/workflows/e2e.yml/badge.svg)](https://github.com/sukekyo26/cocoon/actions/workflows/e2e.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](../LICENSE)

[English README](../README.md)

プロジェクトごとに最適化されたコンテナ開発環境を生成する**ジェネレータ**。任意のプロジェクトディレクトリで `cocoon init && cocoon gen` を実行すると、そのリポジトリ用の `.devcontainer/` 一式が出来あがります。コンテナ起動は `docker compose` か VS Code の「Reopen in Container」を使ってください。

## 特徴

- **単一バイナリ** — `go:embed` で全プラグインをバイナリに同梱。一度入れればどのプロジェクトでも使えます
- 1 つの `workspace.toml` から `.devcontainer/{Dockerfile, docker-compose.yml, devcontainer.json, docker-entrypoint.sh, .env}` を生成
- VS Code Dev Containers と素の `docker compose` が同じ出力を共有
- **レイヤード上書き** — `<project>/.cocoon/plugins/` > `~/.cocoon/plugins/` > 埋め込みカタログ (標準 20 プラグイン)
- **対話的な `cocoon init`** — マウント範囲、ログインシェル、apt カテゴリ、プラグイン、エイリアスバンドルを選び、自己説明的な `workspace.toml` を出力
- **個人シェル設定の永続化** — コンテナ内 `~/.cocoon/.shellrc` (fish は `~/.cocoon/.shellrc.fish`) が named volume にバックされており、`docker compose down && up --build` を跨いでもユーザー個別の alias / PATH が残ります
- **国際化対応** — プロンプト・エラーメッセージ・生成 `workspace.toml` のインラインコメントが `$LANG` に応じて英語 / 日本語に切替

## 動作要件

- Linux / macOS / WSL2
- Docker 23 以上 (BuildKit 有効) + `docker compose` v2.18 以上
- Go 1.26 以上 (ソースビルド時のみ)

## インストール

```bash
# 推奨: SHA256 検証付きビルド済みバイナリ
curl -fsSL https://raw.githubusercontent.com/sukekyo26/cocoon/main/install.sh | sh

# 代替: ソースビルド (Go 1.26 以上)
go install github.com/sukekyo26/cocoon/cmd/cocoon@latest
```

## クイックスタート

```bash
cd ~/projects/my-api
cocoon init                                              # 対話で workspace.toml を生成
cocoon gen                                               # .devcontainer/ を生成
docker compose -f .devcontainer/docker-compose.yml up -d # または VS Code で「Reopen in Container」
```

> **社内 CA をコンテナで信頼させたい** (Zscaler、開発用自己署名 等) 場合は `cocoon init --certificates` (または `workspace.toml` に `[certificates] enable = true`) で opt-in したうえでホスト側の `~/.cocoon/certs/` に `.crt` を置く。opt-in しないワークスペースの成果物には cert 関連の配線は一切乗らない。詳細は [`[certificates]`](configuration.ja.md#certificates) 参照。

## ドキュメント

| トピック | English | 日本語 |
|---|---|---|
| アーキテクチャ | [architecture.md](architecture.md) | [architecture.ja.md](architecture.ja.md) |
| 設定 (`workspace.toml`) | [configuration.md](configuration.md) | [configuration.ja.md](configuration.ja.md) |
| コマンド | [commands.md](commands.md) | [commands.ja.md](commands.ja.md) |
| プラグイン作成 (`plugin.toml`, `install.sh`, `install_user.sh`) | [plugins.md](plugins.md) | [plugins.ja.md](plugins.ja.md) |
| 変更履歴 | [CHANGELOG.md](../CHANGELOG.md) | [CHANGELOG.ja.md](CHANGELOG.ja.md) |

## ライセンス

MIT — 詳細は [LICENSE](../LICENSE) を参照。
