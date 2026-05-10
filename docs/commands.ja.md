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
| `cocoon plugin pin <id> <ref>` | `[plugins.versions.<id>]` ブロックを生成 (stdout / `--write` で in-place) |
| `cocoon plugin scaffold <id>` | テンプレートから新規 `<id>/` ディレクトリを作成 |
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

`workspace.toml` を読み、レイヤード FS (project ∪ user ∪ embedded) でプラグインカタログを解決し、`.devcontainer/` を出力。プラグインの install スクリプトは生成 Dockerfile 内に直接埋め込まれるため、ビルドはプロジェクトツリー以外を必要としない。

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

### TLS 証明書

生成される `Dockerfile` / `docker-compose.yml` / `devcontainer.json` には、ホスト `~/.cocoon/certs/*.crt` を build 時にコンテナのトラストストアへ取り込む配線が常時含まれる (証明書の有無で生成物が変化しないため、チームで commit して共有可能)。詳細は [`configuration.ja.md` の TLS 証明書セクション](configuration.ja.md#tls-証明書-cocooncerts) を参照。

---

## `cocoon plugin`

cocoon プラグインの管理。プラグインは 3 層に分かれ、優先度 **project > user > embedded** で解決される。

| 層 | パス | 出所 |
|---|---|---|
| project | `<workspace>/.cocoon/plugins/<id>/` | `add --scope project` または同所への scaffold |
| user | `~/.cocoon/plugins/<id>/` | `add --scope user` (デフォルト) |
| embedded | `internal/plugin/catalog/<id>/` (バイナリに同梱) | cocoon 同梱カタログ |

典型的な利用フロー:

1. `cocoon plugin add <id>` で埋め込みプラグインを overlay にコピー
2. その overlay 内の `plugin.toml` / `install.sh` を編集
3. `workspace.toml` の `[plugins].enable` に `<id>` を追加
4. `cocoon gen && docker compose up -d --build`

overlay は `gen` 時にのみ参照される。`~/.cocoon/plugins/<id>/` にファイルを置いただけでは有効化されず、`[plugins].enable` への追加が必須。

### `cocoon plugin list`

**目的:** 層解決後にアクセスできるプラグイン ID 一覧を、各 ID の解決元層付きで表示。

**例:**

```console
$ cocoon plugin list
ID            SOURCE    DEFAULT  DESCRIPTION
claude-code   embedded  false    Claude Code — AI-powered coding assistant ...
go            embedded  false    Go programming language ...
my-internal   user      true     internal CLI ...
```

**フラグ:**

| フラグ | 説明 |
|---|---|
| `--source <embedded\|user\|project>` | 単一層に絞って表示 (複数値不可)。 |

**落とし穴:** 同 id が複数層にある場合、最高優先度の層のみ表示される。下位層を見たい場合は `cocoon plugin remove --scope <層>` で上位層を剥がす。

### `cocoon plugin show <id>`

**目的:** 解決後の `plugin.toml` をパースして安定化された差分しやすい形で再描画し、所属層と併せて表示。

**例:**

```console
$ cocoon plugin show go
id: go
source: embedded
name: Go
description: Go programming language ...
default: false
requires_root: true
version_capable: true
apt_packages: [build-essential]
env:
  GOPATH=/home/${USERNAME}/go
  PATH=/usr/local/go/bin:$GOPATH/bin:$PATH
volumes: [/home/${USERNAME}/go]
```

**落とし穴:** id が見つからなければ `plugin "<id>" not found in any layer` エラー。`apt_packages` / `env` は安定化のためアルファベット順ソート — `plugin.toml` 上の記述順とは一致しない。

### `cocoon plugin add <id>`

**目的:** 埋め込みプラグインを書き込み可能な overlay にコピーし、`plugin.toml` / `install.sh` をローカルで編集可能にする。

**例:**

```console
$ cocoon plugin add starship --scope user
Plugin "starship" copied to /home/alice/.cocoon/plugins/starship (user overlay)
$ $EDITOR ~/.cocoon/plugins/starship/install.sh
```

**フラグ:**

| フラグ | 説明 |
|---|---|
| `--scope <user\|project>` | コピー先。デフォルト `user` (`~/.cocoon/plugins/<id>/`)。`project` は `<workspace>/.cocoon/plugins/<id>/`。 |
| `--force` | 既存 overlay コピーを上書き (`--force` 無しで既存があるとエラー)。 |

**落とし穴:**

- overlay にコピーされても**自動有効化されない** — `workspace.toml` の `[plugins].enable` に `<id>` を追加し、`cocoon gen` を再実行する必要がある。
- `--scope project` は cwd から `workspace.toml` を発見できる必要がある。発見できなければ usage error。
- `*.sh` ファイルはコピー後に mode `0755` に再設定される (umask が厳しくても）。

### `cocoon plugin remove <id>`

**目的:** user / project スコープの overlay コピーを削除する。embedded カタログには影響しない。

**例:**

```console
$ cocoon plugin remove starship --scope user
Plugin "starship" removed from /home/alice/.cocoon/plugins/starship
```

**フラグ:**

| フラグ | 説明 |
|---|---|
| `--scope <user\|project>` | 削除対象の overlay (**必須**、デフォルト無し)。 |

**落とし穴:**

- どちらの overlay を削除するかを必ず明示させるため `--scope` は必須。
- 削除後の `cocoon plugin list` では、同 id について次の優先度層 (or embedded) が表示される。

### `cocoon plugin pin <id> <ref>`

**目的:** `version_capable` プラグイン用に上流バージョン (任意で per-arch チェックサム) を `workspace.toml` の `[plugins.versions.<id>]` に記録する。ブロックは `pin = "<ref>"` と任意の `checksum_amd64` / `checksum_arm64` 行を含み、`install.sh` が `$PIN` / `$CHECKSUM_AMD64` / `$CHECKSUM_ARM64` から読む。

**例 (デフォルト — stdout, 手動貼り付け):**

```console
$ cocoon plugin pin go 1.23.4 --amd64-checksum abc123 --arm64-checksum def456
# Append the following block to workspace.toml under [plugins.versions]:

[plugins.versions.go]
pin = "1.23.4"
checksum_amd64 = "abc123"
checksum_arm64 = "def456"
```

**例 (`--write` — in-place 編集):**

```console
$ cocoon plugin pin go 1.23.4 --write
Updated /home/alice/proj/workspace.toml: [plugins.versions.go]
```

`--write` は `workspace.toml` を行ベースでパースし、既存 `[plugins.versions.<id>]` ブロックがあれば置換、無ければ最後の `[plugins.versions.*]` ブロックの直後に追加する。対象ブロック外のコメント・空行は保持される。

**フラグ:**

| フラグ | 説明 |
|---|---|
| `--amd64-checksum <sha256>` | amd64 アーティファクトの SHA256。 |
| `--arm64-checksum <sha256>` | arm64 アーティファクトの SHA256。 |
| `--write` | `workspace.toml` (cwd から自動検出) に in-place 挿入・置換。 |

**落とし穴:**

- `pin` は `[version].version_capable = true` のプラグインでのみ意味を持つ。それ以外では `gen` 時に無視される。
- チェックサムフラグは `install.sh` が実際に `$CHECKSUM_AMD64` / `$CHECKSUM_ARM64` を読む場合 (= `tarball` テンプレート由来) のみ意味を持つ。`curl-pipe` / `generic` では無視される。
- `--write` は cwd から `workspace.toml` を発見できる必要がある。`--write` 無しなら id 検証用に LayeredFS を解決するだけなので、どこからでも動く。
- `--write` は multi-line `[plugins.versions.<id>]` 形式のみを編集する。`[plugins.versions]` 直下に任意の key 代入がある場合 — `<id> = "1.23.4"` / `<id> = [..]` / inline-table `<id> = { pin = "..." }` (`init` テンプレートがコメント行で示しているスタイル) のいずれも — `--write` は重複ブロックを追加せず usage error で停止する。各エントリを `[plugins.versions.<id>]` ブロックへ変換するか、`workspace.toml` を手動編集する。

### `cocoon plugin scaffold <id>`

**目的:** 3 種類のテンプレートから `plugin.toml` + `install.sh` 雛形を含む新規 `<id>/` ディレクトリを生成する。新規 project / user スコーププラグインの初期化用。

**例:**

```console
$ cd ~/projects/myapp
$ cocoon plugin scaffold gh-cli \
    --template curl-pipe --version-capable \
    --name "GitHub CLI" --description "GitHub CLI (https://cli.github.com)" \
    --non-interactive
OK: scaffolded /home/alice/projects/myapp/.cocoon/plugins/gh-cli (2 files)
```

**テンプレート:**

| テンプレート | 用途 | 雛形 |
|---|---|---|
| `curl-pipe` | 上流が `curl ... \| bash` 形式 (uv, proto) | `$PIN` バージョン制御。チェックサム検証なし |
| `tarball` | 上流が GitHub Release tarball (starship, go) | `$PIN` + `$CHECKSUM_AMD64` / `$CHECKSUM_ARM64` + `dpkg --print-architecture` 分岐 (`--version-capable` 強制) |
| `generic` | apt パッケージ or freeform | 最小雛形、`$PIN` 配線なし |

**フラグ:**

| フラグ | 説明 |
|---|---|
| `--plugins-dir <path>` | 出力ディレクトリ。デフォルト: `<workspace>/.cocoon/plugins` (`workspace.toml` から自動検出)。 |
| `--name <name>` | 表示名 (例: `"GitHub CLI"`)。 |
| `--description <text>` | 短い説明。括弧内に上流 URL を含める必要あり。 |
| `--default` | デフォルト有効化フラグを立てる。 |
| `--requires-root` | `install.sh` を root 実行に。 |
| `--version-capable` | `$PIN` / `$CHECKSUM_*` の雛形を生成。 |
| `--template <kind>` | `curl-pipe` \| `tarball` \| `generic`。 |
| `--with-install-user` | `install_user.sh` も生成 (`install.sh` の後に非特権ユーザーで走る)。 |
| `--non-interactive` | プロンプトをスキップ (上記すべて要指定)。 |
| `--force` | `<plugins-dir>/<id>/` が既存なら上書き。 |

**落とし穴:**

- `--plugins-dir` を指定せず、cocoon プロジェクト外 (workspace.toml 未発見) で実行すると、`./plugins/<id>/` に黙って書く代わりに actionable error で停止する。
- `--template tarball` は `--version-capable` を要求する。`tarball` 単体だと拒否される。
- scaffold 後、生成された `plugin.toml` は runtime と同じ strict validator で再ロードされる。失敗 (不正な name、description に URL なし、等) すればディレクトリはロールバックされる。
- `add` 由来の overlay と同様、scaffold 直後でも `[plugins].enable` に `<id>` を追加しないと `gen` で反映されない。

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
