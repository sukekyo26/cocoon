# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## コマンド

- `just ci` — push 前ゲート（`fmt-check + vet + lint + test + cover-check + vuln + mod-verify + shellcheck + shfmt-check`）。
- `just regen-snapshots` — golden / fixture（dockerfile / compose / devcontainerjson / plugin / init）を一括再生成。ジェネレータ・プラグインミューテータ・`cocoon init` 出力を意図的に変えたら走らせ、`testdata/` をソース変更と同じコミットに入れる。CI は `-update-golden` なしで走るためドリフトは即落ちる。
- 単一テスト: `go test ./internal/cli/init -run TestInitInteractive`
- カバレッジ閾値は 90%（`MIN_COVERAGE` で上書き可、`just cover-check`）。
- 個別レシピは `just --list` 参照。

## CHANGELOG

`CHANGELOG.md`（英）と `docs/CHANGELOG.ja.md`（日）を同期する。

- **記載する ✓**: `workspace.toml` フィールド / プラグイン仕様（`install.sh`, `plugin.toml`）/ `cocoon` サブコマンド・フラグ / BREAKING / セキュリティ修正。
- **記載しない ✗**: テスト用 DI ヘルパ / `just` レシピ追加 / lint・フォーマット設定 / 内部リファクタ。

## アーキテクチャ

cocoon は純粋なジェネレータ。`workspace.toml` → `.devcontainer/`（Dockerfile + compose + devcontainer.json + entrypoint + .env + manage.sh）。ライフサイクルは `docker compose` / VS Code Dev Containers に委譲。詳細は [`docs/architecture.md`](../docs/architecture.md)。

**`cocoon gen` パイプライン**: `internal/config/discovery.go`（cwd → `.cocoon/` → 親、`.git` / `$HOME` で停止）→ `internal/plugin/layered.go`（3 層オーバーレイ）→ `internal/generate/{dockerfile,compose,devcontainerjson,envfile,shellrc}` がインメモリ描画 → `internal/cli/generate/WriteArtifacts` がアトミック書き込み。

**プラグイン LayeredFS（優先順位 project > user > embedded）**:
- project: `<workspace>/.cocoon/plugins/<id>/`
- user: `~/.cocoon/plugins/<id>/`
- embedded: `internal/plugin/catalog/<id>/`（`go:embed`）

**プラグイン契約**（詳細は [`docs/plugins.md`](../docs/plugins.md) と `plugin-authoring` スキル）:
- `install.<category>.sh`（`installer` / `binary` / `apt` / `archive`）+ 任意の `install_user.sh`。Dockerfile に `bash <<'COCOON_PLUGIN_EOF' … COCOON_PLUGIN_EOF` で verbatim 埋め込み、ホスト側キャッシュは作らない。
- `version_capable = true` のプラグインは `$PIN` と（`binary` / `archive` で）`$CHECKSUM_AMD64` / `$CHECKSUM_ARM64` を同一 `RUN` で受け取る。値は `[plugins.versions]` のインラインテーブル行から供給。
- バージョン選択は LATEST + フリーテキストのみ（ホワイトリスト非保持）。

**CLI**: `cmd/cocoon/main.go` がエントリ。exit code は `ErrCanceled → 130`、`ErrUsage → 2`、その他 → 1。cobra は `SilenceErrors=true` なのでエラー出力は `main.go` の責務。サブコマンド: `init` / `gen` / `plugin {list,show,pin,scaffold}` / `self-update` / `version` / `completion`。

**リリース**: `VERSION` ファイル変更を含む `main` への PR がマージされるとタグ・クロスコンパイル・`SHA256SUMS`・`gh release` が走る（`.github/workflows/release.yml`）。

## プロジェクト固有の規約

`~/.claude/CLAUDE.md` と `.claude/rules/{testing,defensive-coding,refactor-discipline}.md`（自動ロード）を補完する。

- `[plugins.versions]` はインラインテーブル（`go = { pin = "..." }`）。`[plugins.versions.go]` サブセクションは禁止（`cocoon plugin pin --write` が usage error で停止する）。
- Sentinel error は `var ErrXxx = errors.New(...)` で export し、`%w` でラップする（err113 lint）。wrap は呼び出し連鎖で 1 回のみ — helper は生の sentinel を返し caller が context を付ける。
- `docs/<topic>.md` と `docs/<topic>.ja.md` は常に同期。
- ドキュメントに件数（プラグイン数等）をハードコードしない。ドリフトしたら修正ではなく数字自体を削除。
- `.github/workflows/*.yml` を編集したら、Bash → Go 移行で削除済みのパス（例: `lib/generators.sh`）が残っていないか grep する。
