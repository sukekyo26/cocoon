---
name: version-bump
description: 'Bump project version following Semantic Versioning. Use when asked to: (1) Release a new version, (2) Bump major/minor/patch version, (3) Move Unreleased changelog entries to a new release. Triggers: "version bump", "bump version", "release", "バージョンアップ", "バージョン上げ", "リリース", "semver".'
---

# バージョンアップ手順

セマンティックバージョニングに基づきプロジェクトバージョンを更新する。

## バージョン決定基準（SemVer）

| 変更内容 | バージョン |
|:---------|:-----------|
| 破壊的変更（BREAKING） | **major** (X.0.0) |
| 新機能追加（Added） | **minor** (x.Y.0) |
| バグ修正のみ（Fixed/Changed/Removed without BREAKING） | **patch** (x.y.Z) |

> **pre-1.0（0.x）の例外**: メジャー 0 系は SemVer §4「初期開発用、いつでも変更してよい」に該当する。BREAKING があっても major（1.0.0）には上げず **minor で吸収する**（0.4.0 → 0.5.0）。1.0.0 は安定版リリースの意思表示なので、プロジェクトの alpha v0.x 方針が続く限り `0.y.z` に留める。

> CHANGELOG のカテゴリは `Added` → `Changed` → `Fixed` → `Removed` の 4 種類のみ。詳細は changelog スキル参照。

`CHANGELOG.md` の `## [Unreleased]` セクションの内容から判断する。

## 更新対象ファイル

| # | ファイル | 更新内容 |
|:-:|:---------|:---------|
| 1 | `VERSION` | ファイル内容を `x.y.z\n` に書き換える（リポジトリ root） |
| 2 | `internal/version/version.go` | `var Version = "x.y.z"` の値を新バージョンに書き換え |
| 3 | `CHANGELOG.md` | `## [Unreleased]` の直下に `## [x.y.z] - YYYY-MM-DD` を追加 |
| 4 | `docs/CHANGELOG.ja.md` | 同上 |

> cocoon は `bin/cocoon-*` を `.gitignore` 済で、リリースバイナリは `.github/workflows/release.yml` が CI 上でクロスコンパイル + `gh release` 公開する。**ローカルで `bin/` を再ビルドしてコミットする必要はない**。

## 手順

1. `CHANGELOG.md` の `## [Unreleased]` の内容を確認し、SemVer に従いバージョンを決定
2. **CHANGELOG 精査（リリース前の必須チェック）**: `## [Unreleased]` は前回リリースからの**正味の差分**を表す。リリース確定前に、英日両ファイル（`CHANGELOG.md` / `docs/CHANGELOG.ja.md`）の `## [Unreleased]` を changelog スキルの基準で見直し、最終的に残すべきエントリだけにする。
   - **相殺の削除**: 同一 Unreleased 内で追加→削除されて元に戻った変更は、リリース前後で見ると差分ゼロなのでログ自体が不要 → エントリを両方とも削除する。
     - Added した機能を同バージョン内で Removed した → 両方削除
     - A→B→A と元に戻った → 削除
     - A→B→C と変遷した → 最終状態のみ（A→C）を記載
   - **対象外エントリの除去**: エンドユーザーまたはプラグイン作者の操作・設定・動作が変わらない項目（内部ロジックのリファクタリング、内部関数のリネーム、ファイル移動、テスト追加、CI/lint 設定変更等）が紛れていないか精査し、見つけたら削除する。判断基準と除外リストは changelog スキルの「記載対象」節に従う。
   - 整理は `CHANGELOG.md`（英語）と `docs/CHANGELOG.ja.md`（日本語）の両方に同じ内容を適用する。

   > 詳細ルール: **changelog スキル**の「同一バージョン内の整理」「記載対象」節を参照。バージョンアップ時の CHANGELOG 検査はこのスキルに委譲する。
3. 上記 1〜4 を更新（`echo x.y.z > VERSION`、`internal/version/version.go` の `var Version = "..."` を新値に書き換え）
4. `## [Unreleased]` 見出しは残す（次の開発用）。見出しの下は空にする
5. `just ci` を実行してグリーンを確認（`internal/version/version.go` 変更で test 影響がないか保険的に）
6. コミット: `feat: release vX.Y.Z` を **`develop` ブランチに直接コミット** する（version bump は CLAUDE.md「機能単位で feature/ ブランチを切る」ルールの例外。`feature/release-vX.Y.Z` は作らない）。その後 `develop` を push する。
7. `develop` → `main` のリリース PR を **pr-create スキルの手順で**作成する。リリース PR ならではの要点:
   - **タイトルは `develop` のリリースコミットと同じ `feat: release vX.Y.Z`**（`chore:` ではない）。
   - **本文は通常の `.github/pull_request_template.md` ではなく、リリース専用テンプレート `.github/PULL_REQUEST_TEMPLATE/release.md` に従って埋める**。`## Released changes` には今回 `CHANGELOG.md` / `docs/CHANGELOG.ja.md` に追加した `## [x.y.z]` セクションの内容（Added/Changed/Fixed/Removed のうち実在カテゴリのみ）をそのまま転記し、Release checklist を確認する。
   - マージ後、`main` 上の VERSION 変更が `release.yml` をトリガしてタグ・クロスコンパイル・`gh release` 公開が走る。

> `justfile` の `version` 変数は `VERSION` ファイルの内容を読み込み `-ldflags "-X github.com/sukekyo26/cocoon/internal/version.Version=..."` を渡す。`just build` 経由のバイナリは ldflags が効くが、`go install github.com/sukekyo26/cocoon/cmd/cocoon@latest` 経由では効かないので、ソース側 `internal/version/version.go` の literal も同期させる必要がある。

## CHANGELOG フォーマット

```markdown
## [Unreleased]

## [x.y.z] - YYYY-MM-DD

### Added
- ...

### Changed
- ...

### Fixed
- ...

### Removed
- ...
```

`## [Unreleased]` と `## [x.y.z]` の間に空行を1つ入れる。

ファイル末尾の比較リンク参照も更新する（英日両ファイル）。`[Unreleased]` の比較元を新バージョンに差し替え、新バージョンの行を1つ追加する:

```markdown
[Unreleased]: https://github.com/sukekyo26/cocoon/compare/vX.Y.Z...HEAD
[X.Y.Z]: https://github.com/sukekyo26/cocoon/compare/v<前バージョン>...vX.Y.Z
```
