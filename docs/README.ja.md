# cocoon

[![Go CI](https://github.com/sukekyo26/cocoon/actions/workflows/go-ci.yml/badge.svg)](https://github.com/sukekyo26/cocoon/actions/workflows/go-ci.yml)
[![E2E](https://github.com/sukekyo26/cocoon/actions/workflows/e2e.yml/badge.svg)](https://github.com/sukekyo26/cocoon/actions/workflows/e2e.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](../LICENSE)

[English README](../README.md)

> [!WARNING]
> **プロジェクトステータス: Alpha (v0.x)。** cocoon は現在開発中です。お使いになる場合は、安定版 (1.0) に達するまでに CLI フラグ・`workspace.toml` スキーマ・プラグイン契約が変更され得ること、各リリースに breaking change が含まれうることをご了承のうえご利用ください。アップグレード時は [CHANGELOG](CHANGELOG.ja.md) の **BREAKING** 行を必ず確認してください。

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

リポジトリにコミットされるのは 30 行ほどの `workspace.toml` だけです。`Dockerfile` も compose ファイルも `devcontainer.json` も、必要なときに毎回フルで作り直されるので、設定の "魔法" がリポジトリに溜まりません。すべての変更がジェネレータの決定的な再実行になります。

## 何が生成されるか

`cocoon gen` は `.devcontainer/` 配下に次のファイルを書き出します:

| ファイル | 役割 |
|---|---|
| `Dockerfile` | 有効化された各プラグインを `bash` heredoc でインライン化したマルチステージビルド |
| `docker-compose.yml` | サービス + named volumes + ports + 任意のサイドカー |
| `devcontainer.json` | VS Code Reopen-in-Container 用 (出力しない選択も可) |
| `docker-entrypoint.sh` | コンテナ起動毎にイメージ焼き込みバイナリを named volume へ復元 |
| `.env` | `COMPOSE_PROJECT_NAME`、UID/GID、IMAGE / IMAGE_VERSION |

同じ生成物で `docker compose up`（CLI 経由）と VS Code の "Reopen in Container" の両方が動きます。

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
cocoon init                                              # 対話に答える
cocoon gen                                               # .devcontainer/ を生成
docker compose -f .devcontainer/docker-compose.yml up -d # または VS Code で「Reopen in Container」
```

## `cocoon init` で聞かれること

1. コンテナの **サービス名** と **ユーザー名**
2. **ベースイメージ** — `ubuntu` / `debian` / `node` / `python` / `golang` / `rust` / `denoland/deno` (DockerHub 正式名称)
3. **イメージバージョン** — 推奨候補からの選択、または任意の Docker タグを直接入力
4. **ログインシェル** — `bash` / `zsh` / `fish`
5. **エイリアスバンドル** — `git` / `ls` / `docker` のショートカット集 (複数選択)
6. **マウント範囲** — cwd のみ、または親ディレクトリ (兄弟リポジトリも見える fat ワークスペース向け)
7. **VS Code Dev Containers** 対応 — `devcontainer.json` を出力するかどうか
8. **社内 CA 自動取り込み** — `~/.cocoon/certs/` 配下の `.crt` をビルド時に取り込むか opt-in (デフォルト off。下記参照)
9. **ポートフォワード** — カンマ区切りの docker-compose short form (例: `3000:3000,5432:5432`)。空 Enter で見送ると `[ports]` 雛形はコメント行のまま残る (後で有効化可能)
10. **apt カテゴリ** — text-editors / vcs / utilities / build / network / … (複数選択)
11. **プラグイン** — 同梱カタログ 25 種から選択 (複数選択)

各回答は自己説明的な 1 行として `workspace.toml` に書き込まれます。`--yes` と各値フラグ (`--service-name` / `--username` / `--image` / `--plugins` / `--certificates` / `--ports` …) を組み合わせれば TTY なしで CI から呼び出せます。

## プラグイン

25 のプラグインが `go:embed` でバイナリに同梱されています:

`aws-cli`, `aws-sam-cli`, `bun`, `claude-code`, `copilot-cli`, `deno`, `docker-cli`, `github-cli`, `go`, `google-chrome`, `helm`, `kubectl`, `lazygit`, `mise`, `nerd-fonts`, `node`, `opentofu`, `proto`, `rust`, `shellcheck`, `shfmt`, `starship`, `terraform`, `uv`, `zig`

`~/.cocoon/plugins/<id>/` (ユーザースコープ) や `<project>/.cocoon/plugins/<id>/` (プロジェクトスコープ。リポジトリにコミット可) で上書き・追加できます。どちらの層も埋め込みカタログより優先されます。作成手順は [`docs/plugins.ja.md`](plugins.ja.md) を参照してください。

## 社内 CA 対応

社内 CA をコンテナ内で信頼させたい (Zscaler、開発用自己署名 等) 場合は `cocoon init --certificates` (または `workspace.toml` に `[certificates] enable = true`) で opt-in したうえで、ホスト側の `~/.cocoon/certs/` に `.crt` を置いてください。コンテナビルド時に自動で取り込まれます。opt-in しないワークスペースの成果物には cert 関連の配線は一切乗りません。詳細は [`[certificates]`](configuration.ja.md#certificates) を参照。

## 個人シェル設定の永続化

cocoon はコンテナ内の `~/.cocoon/` に named Docker volume をマウントするので、ユーザーごとのシェル設定がコンテナリビルドを跨いで残ります。bash / zsh / fish の rc ファイルは起動時に `~/.cocoon/.shellrc` (fish の場合 `~/.cocoon/.shellrc.fish`) を自動 source するため、コンテナ内から編集すれば `docker compose down && up --build` を跨いでも内容が保持されます (`docker compose down -v` でのみリセット)。

## 国際化対応

プロンプト・エラーメッセージ・生成 `workspace.toml` のインラインコメントが英語 / 日本語に切り替わります。ロケールは `WORKSPACE_LANG` → `LC_ALL` / `LC_MESSAGES` / `LANG` の順に検出され、`ja` で始まる値で日本語が選ばれます。

## ドキュメント

| トピック | English | 日本語 |
|---|---|---|
| アーキテクチャ | [architecture.md](architecture.md) | [architecture.ja.md](architecture.ja.md) |
| 設定 (`workspace.toml`) | [configuration.md](configuration.md) | [configuration.ja.md](configuration.ja.md) |
| コマンド | [commands.md](commands.md) | [commands.ja.md](commands.ja.md) |
| プラグイン作成 (`plugin.toml`, `install.sh`, `install_user.sh`) | [plugins.md](plugins.md) | [plugins.ja.md](plugins.ja.md) |
| 変更履歴 | [CHANGELOG.md](../CHANGELOG.md) | [CHANGELOG.ja.md](CHANGELOG.ja.md) |

## 開発

`just ci` が push 前のシングルゲートです (Go fmt / vet / lint / test / vuln / mod-verify + `shellcheck` + `shfmt-check`)。任意で同じシェル系フックをコミット時に走らせる pre-commit 連携も用意しています:

```bash
pip install pre-commit  # または `brew install pre-commit`
pre-commit install      # `git commit` ごとに shellcheck + shfmt が走る
```

`shellcheck` と `shfmt` は `$PATH` 上に必要です。macOS: `brew install shellcheck shfmt`。Linux / WSL: `apt-get install shellcheck` + `shfmt` を <https://github.com/mvdan/sh/releases> からダウンロードしてください。
