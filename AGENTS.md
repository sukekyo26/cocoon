# AI Agent Rules

## Repository Context

cocoon は `cocoon.toml` から `.devcontainer/` を生成する Go CLI です。入口は `cmd/cocoon/main.go`、主要実装は `internal/` 配下にあります。生成対象は Dockerfile、docker-compose.yml、devcontainer.json、entrypoint、`.env`、`manage.sh` です。実行ライフサイクルは Docker Compose と VS Code Dev Containers に委譲します。

## Key Paths

- `internal/config/`: 設定探索、schema、validation。
- `internal/plugin/`: プラグイン manifest、layered loading、pin / lock 関連。
- `internal/generate/`: Dockerfile、Compose、devcontainer、env、shellrc 生成。
- `internal/cli/`: cobra コマンド、CLI 入出力、help snapshot。
- `internal/plugin/catalog/<id>/`: 組み込みプラグイン。
- `tests/` と `e2e/`: 統合・E2E テスト。
- `docs/`: 英日ペアのユーザー向けドキュメント。

## Commands

- `just --list`: 利用可能な recipe を確認する。
- `just build`: `bin/cocoon` をビルドする。
- `just test`: ビルド後に `go test -shuffle=on ./...` を実行する。
- `just lint`: `.golangci.yml` に従って lint を実行する。
- `just ci`: push 前の総合チェックを実行する。
- `just regen-snapshots`: generator、help、`cocoon init` 出力を意図的に変えた時だけ実行し、更新された `testdata/` を同じ変更に含める。

## Project-Specific Rules

- `cocoon gen` は config discovery → plugin layered load → in-memory render → atomic write の流れを保つ。
- プラグインの優先順位は project `<workspace>/.cocoon/plugins/<id>/`、user `~/.cocoon/plugins/<id>/`、embedded `internal/plugin/catalog/<id>/` の順にする。
- バージョン pin は `[plugins].enable` の inline 形式（例: `"go=1.23.4"`, `"go=latest"`, `"docker-cli"`）を使う。旧 `[plugins.versions]` は復活させない。
- 再現可能ビルドは `cocoon lock` で `cocoon.lock` を作り、その後 `cocoon gen` する二段構成を守る。
- CLI exit code は `ErrCanceled` が 130、usage error が 2、その他が 1。cobra 側は `SilenceErrors=true` の前提を崩さない。
- ユーザー向け挙動を変えたら `CHANGELOG.md` と `docs/CHANGELOG.ja.md` を同期する。
- `docs/<topic>.md` と `docs/<topic>.ja.md` は同じ内容に保つ。
- ドキュメントにプラグイン数などの変動しやすい件数をハードコードしない。
