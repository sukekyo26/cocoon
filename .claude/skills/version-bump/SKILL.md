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
| バグ修正のみ（Fixed/Changed） | **patch** (x.y.Z) |

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
2. **CHANGELOG 相殺チェック**: 同一 Unreleased 内で追加（Added）と削除/変更（Changed/Removed）が相殺する項目を両方削除する（例: 機能Aを追加 → 同バージョン内で機能Aを削除 → 両エントリとも記載しない）
3. 上記 1〜4 を更新（`echo x.y.z > VERSION`、`internal/version/version.go` の `var Version = "..."` を新値に書き換え）
4. `## [Unreleased]` 見出しは残す（次の開発用）。見出しの下は空にする
5. `just ci` を実行してグリーンを確認（`internal/version/version.go` 変更で test 影響がないか保険的に）
6. コミット: `feat: release vX.Y.Z`（`feature/*` ブランチで実行 → develop → main の流れ。`main` 上の VERSION 変更が `release.yml` をトリガしてタグを切る）

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
```

`## [Unreleased]` と `## [x.y.z]` の間に空行を1つ入れる。
