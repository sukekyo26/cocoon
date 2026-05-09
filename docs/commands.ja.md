# コマンドリファレンス

`cocoon` バイナリが提供する全コマンドのリファレンス。

## クイックリファレンス

| コマンド | 役割 |
|---|---|
| `cocoon init` | `workspace.toml` を対話で生成 |
| `cocoon gen` | `.devcontainer/` 配下の成果物を生成 |
| `cocoon plugin list` | 利用可能な全プラグインを表示 (埋め込み + 上書き) |
| `cocoon plugin show <id>` | 解決後の plugin マニフェストを表示 |
| `cocoon plugin add <id>` | プラグインを user / project 上書き層にコピー |
| `cocoon plugin remove <id>` | 上書き層のコピーを削除 |
| `cocoon plugin pin <id> <ref>` | `[plugins.versions.<id>]` ブロックを stdout 出力 |
| `cocoon plugin scaffold <id>` | テンプレートから新規 `<id>/` ディレクトリを作成 |
| `cocoon config get <file> <field>` | `workspace.toml` のスカラーを表示 |
| `cocoon config list <file> <field>` | `workspace.toml` の配列を表示 |
| `cocoon config volumes <file>` | `[volumes]` エントリを表示 |
| `cocoon config plugin-get <dir-or-file> <field>` | `plugin.toml` のスカラーを表示 |
| `cocoon config plugin-list <dir-or-file> <field>` | `plugin.toml` の配列を表示 |
| `cocoon config plugin-volumes <dir-or-file>` | `plugin.install.volumes` を表示 |
| `cocoon config plugins-table <dir>` | プラグイン 1 件 1 行で表示 |
| `cocoon config validate-workspace <file> [plugins]` | `workspace.toml` を検証 |
| `cocoon config validate-plugins <dir>` | `<dir>` 配下の全プラグインを検証 |
| `cocoon config has-section <file> <section>` | true / false を表示 |
| `cocoon config list-sidecars <file>` | `[services.<name>]` キーを 1 行 1 件で表示 |
| `cocoon config dump-devcontainer <file>` | `[devcontainer]` を TOML で出力 |
| `cocoon config dump-repositories <file>` | `[repositories]` を TOML で出力 |
| `cocoon config repositories <file>` | `[repositories].clone` を JSON で出力 |
| `cocoon config format-repositories <file\|->` | JSON エントリを TOML へ整形 |
| `cocoon self-update` | 最新 GitHub リリースで自分自身を置換 |
| `cocoon version` | バイナリのバージョンを表示 |
| `cocoon help [command]` | ヘルプを表示 (Cobra 標準) |
| `cocoon completion {bash,zsh,fish,powershell}` | シェル補完スクリプトを生成 (Cobra 標準) |

---

## `cocoon init`

カレントディレクトリに `workspace.toml` を生成。

### フラグ

| フラグ | 型 | 説明 |
|---|---|---|
| `--yes` | bool | プロンプトをスキップ。`--service-name` と `--username` が必須になる。 |
| `--service-name <name>` | string | Compose サービス名 (`--yes` 指定時必須)。 |
| `--username <name>` | string | コンテナ内ユーザー名 (`--yes` 指定時必須)。 |
| `--os <id>` | string | ベース OS: `ubuntu` \| `debian`。 |
| `--os-version <ver>` | string | ベース OS バージョン (`--os` と整合する必要あり)。 |
| `--shell <id>` | string | コンテナログインシェル: `bash` \| `zsh` \| `fish`。 |
| `--mount-root <path>` | string | マウント範囲: `"."` (cwd) または `".."` (親)。 |
| `--devcontainer` | bool | `.devcontainer/devcontainer.json` 出力を強制有効化。 |
| `--no-devcontainer` | bool | `.devcontainer/devcontainer.json` をスキップ。 |
| `--apt-categories <ids>` | string | カンマ区切り apt カテゴリ ID (プロンプトをスキップ)。 |
| `--plugins <ids>` | string | カンマ区切りで有効化するプラグイン ID。 |
| `--alias-bundles <ids>` | string | カンマ区切りエイリアスバンドル ID (例: `git,ls`)。 |
| `--force` | bool | 既存 `workspace.toml` を上書き。 |

### 対話フロー

`--yes` なしで実行すると、1 画面ずつ次の順で質問されます:

1. service name
2. username
3. OS
4. OS バージョン (選択した OS で絞り込み)
5. login shell
6. alias bundles (multi-select)
7. mount range
8. devcontainer y/n
9. apt categories (multi-select)
10. plugins (multi-select)

### 例

```bash
# 完全対話
cocoon init

# 非対話
cocoon init --yes \
    --service-name myapp --username dev \
    --os ubuntu --os-version 26.04 \
    --shell bash --mount-root . --devcontainer \
    --apt-categories text-editors,vcs,utilities,compression,build \
    --plugins go,uv,github-cli \
    --alias-bundles git,ls
```

---

## `cocoon gen`

`workspace.toml` を読み、プラグインを materialize し、`.devcontainer/` を出力。

### フラグ

| フラグ | 型 | 説明 |
|---|---|---|
| `--workspace <path>` | string | `workspace.toml` のパス (デフォルト: cwd から探索)。 |
| `--output <dir>` | string | 成果物の書き出し先プロジェクトルート (デフォルト: `workspace.toml` のディレクトリ)。 |

### 例

```bash
# プロジェクトルートから
cocoon gen

# 別の場所の workspace.toml を指定
cocoon gen --workspace ./infra/workspace.toml --output ./infra
```

---

## `cocoon plugin`

cocoon プラグインの管理。`LayeredFS` (project > user > embedded) によりプラグイン ID ごとの解決層が決まる。

### `cocoon plugin list`

| フラグ | 説明 |
|---|---|
| `--source <embedded\|user\|project>` | 単一層から解決されたプラグインのみ表示。 |

### `cocoon plugin show <id>`

解決後の `plugin.toml` と所属層を表示。

### `cocoon plugin add <id>`

埋め込みプラグインを書き込み可能な上書き層へコピーして編集可能にする。

| フラグ | 説明 |
|---|---|
| `--scope <user\|project>` | コピー先。デフォルト `user` (`~/.cocoon/plugins/<id>/`)。 |
| `--force` | 既存上書きコピーを上書き。 |

### `cocoon plugin remove <id>`

上書きコピーを削除。埋め込み版は変更されない。

| フラグ | 説明 |
|---|---|
| `--scope <user\|project>` | 削除する上書き層 (必須)。 |

### `cocoon plugin pin <id> <ref>`

`workspace.toml` 用の `[plugins.versions.<id>]` スニペットを stdout に出力。ファイルは編集しない (既存コメントを保つため)。

| フラグ | 説明 |
|---|---|
| `--amd64-checksum <sha256>` | amd64 アーティファクトの SHA256。 |
| `--arm64-checksum <sha256>` | arm64 アーティファクトの SHA256。 |

### `cocoon plugin scaffold <id>`

テンプレート (`curl-pipe` / `tarball` / `generic`) から新規 `<id>/` ディレクトリを作成。

| フラグ | 説明 |
|---|---|
| `--plugins-dir <path>` | 出力ディレクトリ。デフォルト `plugins`。 |
| `--name <name>` | 表示名 (例: `"GitHub CLI"`)。 |
| `--description <text>` | 短い説明。 |
| `--default` | デフォルト有効化フラグを立てる。 |
| `--requires-root` | `install.sh` を root 実行に。 |
| `--version-capable` | `$PIN` / `$CHECKSUM_*` の雛形を生成。 |
| `--template <kind>` | `curl-pipe` \| `tarball` \| `generic`。 |
| `--with-install-user` | `install_user.sh` も生成。 |
| `--non-interactive` | プロンプトをスキップ (上記すべて要指定)。 |
| `--force` | 既存 `<id>/` を上書き。 |

---

## `cocoon config`

`workspace.toml` / `plugin.toml` のパースと検証。これらのサブコマンドは旧来の bash 連携用で、将来削減される可能性があります。

| サブコマンド | 役割 |
|---|---|
| `get <file> <field>` | `workspace.toml` のスカラー値を表示。 |
| `list <file> <field>` | `workspace.toml` の配列を 1 件 1 行で表示。 |
| `volumes <file>` | `[volumes]` エントリを `name<TAB>path` 形式で表示。 |
| `plugin-get <dir-or-file> <field>` | `plugin.toml` のスカラーを表示。 |
| `plugin-list <dir-or-file> <field>` | `plugin.toml` の配列を表示。 |
| `plugin-volumes <dir-or-file>` | `plugin.install.volumes` を `name<TAB>path` で表示。 |
| `plugins-table <dir>` | プラグイン 1 件 1 行で `id<TAB>name<TAB>default<TAB>description`。 |
| `validate-workspace <file> [plugins]` | `workspace.toml` を検証 (プラグインディレクトリ任意)。 |
| `validate-plugins <dir>` | `<dir>` 配下の `plugin.toml` を全件検証。 |
| `has-section <file> <section>` | `true` / `false` を表示。 |
| `list-sidecars <file>` | `[services.<name>]` キーを 1 行 1 件で表示。 |
| `dump-devcontainer <file>` | `[devcontainer]` を TOML で出力。 |
| `dump-repositories <file>` | `[repositories]` を TOML で出力。 |
| `repositories <file>` | `[repositories].clone` を JSON で出力。 |
| `format-repositories <file\|->` | JSON エントリを TOML へ整形 (`-` で stdin)。 |

---

## `cocoon self-update`

実行中バイナリを最新 GitHub リリースで置換。新バイナリは `SHA256SUMS` と一緒にダウンロード・検証され、atomic rename で差し替え (cross-device 時は fallback あり)。

| フラグ | 説明 |
|---|---|
| `--check-only` | ダウンロードせず更新の有無のみ確認。終了コード 100 は更新あり。 |
| `--force` | 既に最新でも再インストール。 |

---

## `cocoon version`

cocoon バイナリのバージョン (ビルド時 `-X main.Version` 注入) を表示。

---

## `cocoon help [command]`

Cobra 標準。指定コマンドのヘルプを表示。`cocoon help <command>` は `cocoon <command> --help` と等価。

---

## `cocoon completion {bash,zsh,fish,powershell}`

Cobra 標準。シェル補完スクリプトを生成:

```bash
# bash (システム全体)
cocoon completion bash | sudo tee /etc/bash_completion.d/cocoon

# zsh (ユーザーローカル)
cocoon completion zsh > "${fpath[1]}/_cocoon"

# fish
cocoon completion fish > ~/.config/fish/completions/cocoon.fish

# PowerShell
cocoon completion powershell | Out-String | Invoke-Expression
```

---

## 環境変数

| 変数 | 役割 |
|---|---|
| `WORKSPACE_LANG` | cocoon プロンプト・インライン TOML コメントの最優先ロケール。 |
| `LC_ALL` / `LC_MESSAGES` / `LANG` | フォールバックロケールチェーン。`ja` で始まる値で日本語選択。 |
| `COCOON_INSTALL_DIR` | `install.sh` のインストール先 (デフォルト: `$HOME/.local/bin`)。 |

---

## 終了コード

| コード | 意味 |
|---|---|
| `0` | 成功 |
| `1` | 失敗 (`ErrFailure` ラップ) |
| `2` | usage エラー (`ErrUsage` ラップ) |
| `100` | `cocoon self-update --check-only`: 更新あり |
