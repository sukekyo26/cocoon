---
applyTo: "internal/plugin/catalog/**"
---

# プラグイン追加・改修レビュー観点

`internal/plugin/catalog/<id>/` の新規追加・改修 PR をレビューする際の確認観点。
仕様の単一ソースは [`docs/plugins.ja.md`](../../docs/plugins.ja.md)、作業手順は
`plugin-authoring` スキル。本ファイルは **レビューで落とすべき点** に絞る。

## 1. method カテゴリ語彙

- `[install.methods.<name>]` と対応する `install.<name>.sh` の `<name>` は
  **binary / installer / apt / archive** の 4 語彙のみ。vendor 名（`gh-cli`,
  `astral`, `bun-sh` 等）は指摘 — workspace.toml の `[plugins.methods]` が
  プラグインごとにブレる。
- `install.sh`（カテゴリ無し）は loader が reject。必ず `install.<category>.sh`。
- method 1 つなら `cocoon init` に method ピッカーは出ない。不要な複数 method は
  KISS 違反として指摘。

## 2. 契約テスト spec 行（**必須・忘れると just ci が落ちる**）

- 新規プラグインは `internal/plugin/contracts_test.go` の `TestPluginContracts`
  の `specs` スライスに spec 行を **必ず追加**。無いと
  `plugin "<id>" has no contract spec in TestPluginContracts` で `just ci` 失敗。
- `requiresRoot` / `versionCapable` / `verify` / `firstVolume` を plugin.toml と
  一致させ、`mustContain` で install スクリプトの要点（上流ドメイン・
  `sha256sum -c -`・`tlsv1.2`・`dpkg --print-architecture` 等）を pin、
  `mustNotContain` で `noPlaceholders` / `noApiNoJq` を否定する。

## 3. e2e への登録

- プラグイン本体は `e2e/docker-roundtrip.sh` が catalog を**ディレクトリ走査して
  自動検出**し、`amd64-full` / `arm64-full` プリセットで自動 enable する
  （`.github/workflows/e2e.yml` の編集は **不要**）。**pin が無ければ LATEST で
  テストされる — pin は必須ではない**。
- 再現性のため version_capable プラグインは慣例として `pin_entries` に
  `<id>=<version>` を追加する（未 pin = LATEST = 上流リリースで結果が変動）。
  `pin_entries` に書けるのは **version_capable のみ** — 非対応 id を入れると e2e が
  `not version_capable; it cannot be pinned` で fail-fast するので指摘。
- arm64 で動かない（amd64 ハードコード・fail-fast）プラグインは
  `e2e/arm64-exclude.txt` に `<id>` を追加。`TestArm64ExcludeIDsExist` が実在 id を
  ガードする。追加漏れは arm64 で壊れたまま素通りするので指摘。

## 4. $PIN / $CHECKSUM 契約とチェックサム検証

- `version_capable` は `$PIN`（leading `v` / prefix を剥いだ版）と、binary/archive で
  `$CHECKSUM_AMD64` / `$CHECKSUM_ARM64` を **同一 `RUN`** で受ける。
- 検証は次のいずれか。**黙ってスキップは禁止**:
  - pin 時: `echo "${CHECKSUM}  <file>" | sha256sum -c -`
  - 上流が `.sha256` / `SHA256SUMS` 等を公開: fetch して検証（CDN 破損対策）
  - 上流が一切公開しない（例: openai/codex, shfmt, shellcheck）: **warn-and-skip**
    （`printf 'WARNING: ... verification skipped ...' >&2`）し、pin で検証可能にする
- `api.github.com` + `jq` でのチェックサム取得は rate-limit を招く。代替
  （`.sha256` sidecar / warn-and-skip）が取れるなら指摘。

## 5. install スクリプトの堅牢性

- 先頭に `set -euo pipefail`。
- curl は `--proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --retry-all-errors`。
- ARCH は `dpkg --print-architecture`（amd64/arm64）→ 上流命名（x86_64/aarch64 等）に写像。
- `/tmp` の中間物は最後に `rm`。`install -o root -g root -m 0755` 等で明示権限。
- 生成 Dockerfile は heredoc `bash <<'COCOON_PLUGIN_EOF' … COCOON_PLUGIN_EOF` で
  verbatim 埋め込み。スクリプト本文に `COCOON_PLUGIN_EOF` を書かない。
- ホスト側キャッシュを作らない（ビルド時 download のみ）。

## 6. plugin.toml メタデータ

- `metadata.url` は `https://` で始まる実 URL（`TestCatalogMetadataURL` がガード）。
  description に URL を埋め込まない。
- `requires_root` は実態に一致。`volumes` / `[install.env]` のパスは
  `/home/${USERNAME}/...` 形式。

## 7. golden・CHANGELOG・docs

- ジェネレータ出力が drift したら `just regen-snapshots` で再生成し、`testdata/` を
  ソース変更と同一コミットに入れる（CI は `-update-golden` 無しで落ちる）。
- ユーザー可視の追加（新プラグイン）は `CHANGELOG.md` と `docs/CHANGELOG.ja.md` の
  `[Unreleased]` を **両言語同期**で更新（`### Added` / `### 追加`）。
- `docs/plugins.md` / `docs/plugins.ja.md` の catalog tour は例示であり網羅リスト
  ではない。件数ハードコードや全プラグイン列挙を足さない。
