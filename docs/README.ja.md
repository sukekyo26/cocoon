# cocoon

[![Go CI](https://github.com/sukekyo26/cocoon/actions/workflows/go-ci.yml/badge.svg)](https://github.com/sukekyo26/cocoon/actions/workflows/go-ci.yml)
[![E2E](https://github.com/sukekyo26/cocoon/actions/workflows/e2e.yml/badge.svg)](https://github.com/sukekyo26/cocoon/actions/workflows/e2e.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](../LICENSE)

[English README](../README.md)

> [!WARNING]
> **プロジェクトステータス: Alpha (v0.x)。** cocoon は現在開発中です。お使いになる場合は、安定版 (1.0) に達するまでに CLI フラグ・`cocoon.toml` スキーマ・プラグイン契約が変更され得ること、各リリースに breaking change が含まれうることをご了承のうえご利用ください。アップグレード時は [CHANGELOG](CHANGELOG.ja.md) の **BREAKING** 行を必ず確認してください。

## なぜ cocoon を使うのか

**`Dockerfile` も `docker-compose.yml` も自分で書きたくない人のためのツールです。**

Docker ベースの開発環境を自前で組むと、毎プロジェクトでこれを書くことになります:

- `Dockerfile` (60〜120 行) — ベースイメージ、apt、ユーザー作成、各 CLI のインストール手順
- `docker-compose.yml` (30〜80 行) — service / mounts / volumes / env / ports
- `devcontainer.json` (20〜40 行) — VS Code Dev Containers 連携

cocoon ならこうなります:

```bash
cocoon init   # 「ベースイメージは？シェルは？欲しい CLI は？」に答える
cocoon gen    # .devcontainer/ をフルで再生成
docker compose -f .devcontainer/docker-compose.yml up -d
```

真実の源は 30 行ほどの `cocoon.toml` です。`cocoon gen` はそこから `.devcontainer/` 全体を決定的に再生成するので、設定の "魔法" がリポジトリに溜まらず、すべての変更がジェネレータの再実行になります。生成物はホスト非依存なので、`cocoon.toml` だけをコミットしてホストごとに再生成してもよいですし、`.devcontainer/` を一度コミットしてチーム全員がそのままビルドしてもかまいません。

## 何が生成されるか

`cocoon gen` は `.devcontainer/` 配下に次のファイルを書き出します:

| ファイル | 役割 |
|---|---|
| `Dockerfile` | 有効化された各プラグインを `bash` heredoc でインライン化したマルチステージビルド |
| `docker-compose.yml` | サービス + named volumes + ports + 任意のサイドカー |
| `devcontainer.json` | VS Code Reopen-in-Container 用 (出力しない選択も可) |
| `docker-entrypoint.sh` | コンテナ起動毎にユーザーをホスト UID/GID へ再マッピングし、イメージ焼き込みバイナリを named volume へ復元 |
| `manage.sh` | プロジェクト単位の Docker クリーン / リビルド用ヘルパー（ホスト側で実行） |
| `.env` | `COMPOSE_PROJECT_NAME`、`CONTAINER_SERVICE_NAME`、`USERNAME`、IMAGE / IMAGE_VERSION — ホスト非依存・コミット可 |

同じ生成物で `docker compose up`（CLI 経由）と VS Code の "Reopen in Container" の両方が動きます。

### 掃除とリビルド

Docker は未使用のイメージ・ボリューム・ビルドキャッシュを溜め込み、ディスクを圧迫します。`.devcontainer/manage.sh` は **このプロジェクトの** リソースだけを掃除・リビルドします — スクリプトが生成された compose ファイルに対して `docker compose` を駆動するため、スコープは自動です。

```bash
./.devcontainer/manage.sh clean             # コンテナ + ネットワーク + ボリューム + ビルド済みイメージ
./.devcontainer/manage.sh clean containers  # コンテナのみ（ネットワーク・ボリューム・イメージは残す）
./.devcontainer/manage.sh clean image       # コンテナ + ネットワーク + ビルド済みイメージ（ボリュームのデータは残す）
./.devcontainer/manage.sh clean volumes     # コンテナ + ネットワーク + ボリューム（ビルド済みイメージは残す — 高速リビルド）
./.devcontainer/manage.sh rebuild           # --no-cache でイメージを再ビルドしコンテナを再生成
./.devcontainer/manage.sh prune-cache       # Docker ビルドキャッシュを prune（全プロジェクトに影響）
```

破壊的なコマンドは実行前に確認します。`-y` で確認をスキップできます。ビルドキャッシュはプロジェクト単位にスコープできないため `prune-cache` は構造上グローバルで、意図的に `clean` とは別コマンドにしています。全コマンドは `./.devcontainer/manage.sh -h` を参照してください。

## 動作要件

- Linux / macOS / WSL2
- Docker 23 以上 (BuildKit 有効) + `docker compose` v2.18 以上
- Go 1.26 以上 (ソースビルド時のみ)

## インストール

```bash
# 既定: ビルド済みバイナリを SHA256 検証付きでインストール。
curl -fsSL https://raw.githubusercontent.com/sukekyo26/cocoon/main/install.sh | sh

# 代替: 同じバイナリを GitHub Pages ミラー (`*.github.io`) 経由で取得。
# `*.github.io` には到達できるが `raw.githubusercontent.com` /
# `api.github.com` には到達できない環境で利用してください。
curl -fsSL https://sukekyo26.github.io/cocoon/install.sh | \
  COCOON_PAGES_BASE=https://sukekyo26.github.io/cocoon sh

# ソースビルド (Go 1.26 以上)
go install github.com/sukekyo26/cocoon/cmd/cocoon@latest
```

Pages ミラーは各リリースタグを `https://sukekyo26.github.io/cocoon/v<tag>/` (例: `/v0.7.4/`) に発行し、リリースのたびに `/latest/` と `/VERSION` を更新します。`COCOON_VERSION=0.7.4 sh` のように既存のバージョン pin もそのまま使えます。リポジトリを fork して独自ミラーをホストする場合は、初回のみ **Settings → Pages → Source: GitHub Actions** を有効化してください。以降は `pages.yml` ワークフローがリリースごとに自動デプロイします。

## クイックスタート

```bash
cd ~/projects/my-api
cocoon init                                              # 対話に答える
cocoon gen                                               # .devcontainer/ を生成
docker compose -f .devcontainer/docker-compose.yml up -d # または VS Code で「Reopen in Container」
```

## `cocoon init` で聞かれること

1. コンテナの **サービス名** と **ユーザー名**
2. **ベースイメージ** — `ubuntu` / `debian` / `node` / `python` / `golang` / `rust` / `denoland/deno` (DockerHub 正式名称)
3. **イメージバージョン** — 推奨候補からの選択、または任意の Docker タグを直接入力
4. **user-local インストール先 / PATH の自動設定** (言語イメージ選択時のみ) — `[container.shell.env]` に追加して `npm install -g` / `pip` / `go install` / `cargo install` / `deno install` を `sudo` なしで動かす。`node` / `python` / `golang` / `rust` / `denoland/deno` で既定 on。詳細は [`docs/configuration.ja.md#言語イメージ向け-path-自動設定`](configuration.ja.md#言語イメージ向け-path-自動設定) を参照
5. **ログインシェル** — `bash` / `zsh` / `fish`
6. **エイリアスバンドル** — `git` / `ls` / `docker` のショートカット集 (複数選択)
7. **マウント範囲** — cwd のみ、または親ディレクトリ (兄弟リポジトリも見える fat ワークスペース向け)
8. **コンテナ内 workdir 名** — `/home/<user>/` 配下の親ディレクトリ名 (既定 `workspace`。スラッシュで多段階層も可。例: `work/myproject`。AWS SAM などコンテナ内パスをホスト構成に合わせたいツール向け)
9. **VS Code Dev Containers** 対応 — `devcontainer.json` を出力するかどうか
10. **社内 CA 自動取り込み** — `~/.cocoon/certs/` 配下の `.crt` / `.cer` をビルド時に取り込むか opt-in (デフォルト off。下記参照)
11. **ポートフォワード** — カンマ区切りの docker-compose short form (例: `3000:3000,5432:5432`)。空 Enter で見送ると `[ports]` 雛形はコメント行のまま残る (後で有効化可能)
12. **apt カテゴリ** — agent / text-editors / vcs / utilities / build / network / … (複数選択)
13. **プラグイン** — 同梱カタログから選択 (複数選択)

各回答は自己説明的な 1 行として `cocoon.toml` に書き込まれます。`--yes` と各値フラグ (`--service-name` / `--username` / `--image` / `--dir` / `--plugins` / `--certificates` / `--ports` …) を組み合わせれば TTY なしで CI から呼び出せます。

## プラグイン

cocoon は `go:embed` でバイナリに同梱したプラグインカタログを提供します。全カタログは `cocoon plugin list`、個別プラグインの詳細は `cocoon plugin show <id>` で確認できます。コマンドが信頼できる一次情報なので、README ではリストを重複させません（ドリフト防止）。

`~/.cocoon/plugins/<id>/` (ユーザースコープ) や `<project>/.cocoon/plugins/<id>/` (プロジェクトスコープ。リポジトリにコミット可) で上書き・追加できます。どちらの層も埋め込みカタログより優先されます。作成手順は [`docs/plugins.ja.md`](plugins.ja.md) を参照してください。

## 社内 CA 対応

プライベート CA をコンテナ内で信頼させたい (TLS インターセプトプロキシ、開発用自己署名 等) 場合は `cocoon init --certificates` (または `cocoon.toml` に `[certificates] enable = true`) で opt-in したうえで、ホスト側の `~/.cocoon/certs/` に `.crt` / `.cer` を置いてください。コンテナビルド時に自動で取り込まれます。opt-in しないワークスペースの成果物には cert 関連の配線は一切乗りません。詳細は [`[certificates]`](configuration.ja.md#certificates) を参照。

## 個人シェル設定の永続化

cocoon はコンテナ内の `~/.cocoon/` に named Docker volume をマウントするので、ユーザーごとのシェル設定がコンテナリビルドを跨いで残ります。bash / zsh / fish の rc ファイルは起動時に `~/.cocoon/.shellrc` (fish の場合 `~/.cocoon/.shellrc.fish`) を自動 source するため、コンテナ内から編集すれば `docker compose down && up --build` を跨いでも内容が保持されます (`docker compose down -v` でのみリセット)。

## 国際化対応

プロンプト・エラーメッセージ・生成 `cocoon.toml` のインラインコメントが英語 / 日本語に切り替わります。ロケールは `WORKSPACE_LANG` → `LC_ALL` / `LC_MESSAGES` / `LANG` の順に検出され、`ja` で始まる値で日本語が選ばれます。

## ドキュメント

| トピック | English | 日本語 |
|---|---|---|
| アーキテクチャ | [architecture.md](architecture.md) | [architecture.ja.md](architecture.ja.md) |
| 設定 (`cocoon.toml`) | [configuration.md](configuration.md) | [configuration.ja.md](configuration.ja.md) |
| コマンド | [commands.md](commands.md) | [commands.ja.md](commands.ja.md) |
| プラグイン作成 (`plugin.toml`, `install.<category>.sh`, `install_user.sh`) | [plugins.md](plugins.md) | [plugins.ja.md](plugins.ja.md) |
| 変更履歴 | [CHANGELOG.md](../CHANGELOG.md) | [CHANGELOG.ja.md](CHANGELOG.ja.md) |

## 開発

`just ci` が push 前のシングルゲートです (Go fmt / vet / lint / test / vuln / mod-verify + `shellcheck` + `shfmt-check`)。任意で pre-commit 連携を入れると、その高速サブセット (`shellcheck` / `shfmt` / `golangci-lint fmt --diff` / `go vet` / `golangci-lint run` / `go mod verify` / `go mod tidy` ドリフト検査) が `git commit` ごとに走ります。重いゲート (test / coverage / govulncheck / cross-compile) は引き続き `just ci` 側に残します。

```bash
pip install pre-commit  # または `brew install pre-commit`
pre-commit install      # `git commit` ごとにフックが起動
```

`$PATH` 上に必要なもの: `shellcheck` / `shfmt` / `go` / `golangci-lint`。macOS: `brew install shellcheck shfmt golangci-lint`。Linux / WSL: `apt-get install shellcheck` + `shfmt` を <https://github.com/mvdan/sh/releases> からダウンロード + `golangci-lint` は <https://golangci-lint.run/welcome/install/> を参照。
