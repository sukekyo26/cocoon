# コマンドリファレンス

> [!WARNING]
> cocoon は v0.x（alpha）開発段階です。お使いになる場合は、1.0 までに CLI フラグ・サブコマンドが変更され得ること、各リリースに breaking change が含まれうることをご了承のうえご利用ください。詳細は [CHANGELOG](CHANGELOG.ja.md) と README の「プロジェクトステータス」を参照してください。

`cocoon` バイナリが提供する全コマンドのリファレンス。

## クイックリファレンス

| コマンド | 役割 |
|---|---|
| `cocoon init` | 設定ファイルを対話で生成 |
| `cocoon gen` | `.devcontainer/` 配下の成果物を生成 |
| `cocoon gen workspace` | プロジェクトルートに `<name>.code-workspace` を生成 |
| `cocoon lock` | プラグインのバージョンを解決し `cocoon.lock` を書き出して再現性を確保 |
| `cocoon plugin list` | 利用可能な全プラグインを表示 (埋め込み + 上書き) |
| `cocoon plugin show <id>` | 解決後の plugin マニフェストを表示 |
| `cocoon plugin pin <id> <ref>` | バージョンを pin する `[plugins].enable` 配列要素を生成 (stdout / `--write` で in-place) |
| `cocoon plugin scaffold <id>` | テンプレートから新規 `<id>/` ディレクトリを作成 |
| `cocoon self-update` | 最新 GitHub リリースで自分自身を置換 |
| `cocoon version` | バイナリのバージョンを表示 |
| `cocoon help [command]` | ヘルプを表示 (Cobra 標準) |
| `cocoon completion {bash,zsh,fish,powershell}` | シェル補完スクリプトを生成 (Cobra 標準) |

---

## `cocoon init`

カレントディレクトリに設定ファイルを生成。

### フラグ

| フラグ | 型 | 説明 |
|---|---|---|
| `--yes` | bool | プロンプトをスキップ。`--service-name` と `--username` が必須になる。 |
| `--service-name <name>` | string | Compose サービス名 (`--yes` 指定時必須)。 |
| `--username <name>` | string | コンテナ内ユーザー名 (`--yes` 指定時必須)。 |
| `--image <id>` | string | ベースイメージ (DockerHub の正式名称): `ubuntu` \| `debian` \| `node` \| `python` \| `golang` \| `rust` \| `denoland/deno`。省略時の既定は `debian`。 |
| `--image-version <ver>` | string | ベースイメージのタグ。正しい形式 (先頭は英数字または `_`、2 文字目以降は `.` / `-` も可、スラッシュ・コロン禁止) なら任意の Docker タグを受理 (レジストリ実在性は `docker pull` に委ねる)。`--image` が設定されている必要あり。省略時はイメージの先頭サジェストタグ (`debian` → `12`)。 |
| `--shell <id>` | string | コンテナログインシェル: `bash` \| `zsh` \| `fish`。 |
| `--mount-root <path>` | string | マウント範囲: `"."` (cwd) または `".."` (親)。 |
| `--devcontainer` | bool | `.devcontainer/devcontainer.json` 出力を強制有効化。 |
| `--no-devcontainer` | bool | `.devcontainer/devcontainer.json` をスキップ。 |
| `--certificates` | bool | `[certificates] enable = true` を強制有効化（`~/.cocoon/certs/` の自動取り込み）。 |
| `--no-certificates` | bool | `[certificates]` セクション省略を強制（デフォルト）。 |
| `--sudo <mode>` | string | コンテナ内 sudo の方針: `nopasswd`（既定・パスワード不要）/ `password`（`.devcontainer/.env.local` の `SUDO_PASSWORD` を build secret 経由で要求）/ `none`（`no_new_privileges = true`・sudo 無効化）。対話で `password` を選ぶとパスワードを尋ねて `.env.local`（0600）を生成。 |
| `--apt-categories <ids>` | string | カンマ区切り apt カテゴリ ID (プロンプトをスキップ)。 |
| `--plugins <ids>` | string | カンマ区切りで有効化するプラグイン ID。 |
| `--plugin-versions <id>=<ref>,...` | string | カンマ区切りの `<id>=<ref>` で `version_capable` プラグインを pin する。各 `<id>` は `--plugins` にも含まれ、かつ `version_capable = true` である必要があり、重複は不可。バージョンは生成される設定ファイルの `[plugins].enable` 配列にインラインで書き込まれる (例: `--plugin-versions go=1.23.4` → 要素 `"go=1.23.4"`)。checksum なし。 |
| `--alias-bundles <ids>` | string | カンマ区切りエイリアスバンドル ID (例: `git,ls`)。 |
| `--ports <values>` | string | カンマ区切りの docker-compose short-form ポートマッピング (例: `3000:3000,5432:5432`)。`[ports].forward` で扱う全形式を受理: コンテナ単独 `3000`、範囲 `3000-3005:3000-3005`、IPv4/IPv6 バインド `127.0.0.1:8001:8001` / `[::1]:80:80`、プロトコル `6060:6060/udp`。プロンプトをスキップ。空 / 未指定の場合はアクティブな `[ports]` ブロックを書かない（コメント雛形のみ残り、後から有効化できる）。 |
| `--force` | bool | 既存設定ファイルを上書き。 |

### 対話フロー

`--yes` なしで実行すると、1 画面ずつ次の順で質問されます:

1. service name
2. username
3. ベースイメージ
4. イメージバージョン (選択した image で絞り込み)
5. login shell
6. alias bundles (multi-select)
7. mount range
8. devcontainer y/n
9. certificates y/n (`~/.cocoon/certs/` からの TLS 自動取り込みを opt-in。デフォルト no)
10. secure y/n (`no_new_privileges = true` を事前設定しコンテナ内 `sudo` を無効化。デフォルト no)
11. ポートフォワード (カンマ区切りの docker-compose short form。空 Enter で見送り — `[ports]` のコメント雛形は残り、後から有効化できる)
12. apt categories (multi-select)
13. plugins (multi-select)

### 例

```bash
# 完全対話
cocoon init

# 非対話
cocoon init --yes \
    --service-name myapp --username dev \
    --image debian --image-version 12 \
    --shell bash --mount-root . --devcontainer \
    --apt-categories text-editors,vcs,utilities,compression,build \
    --plugins go,uv,github-cli \
    --alias-bundles git,ls \
    --ports 3000:3000,5432:5432
```

---

## `cocoon gen`

設定ファイルを読み、レイヤード FS (project ∪ user ∪ embedded) でプラグインカタログを解決し、`.devcontainer/` を出力。プラグインの install スクリプトは生成 Dockerfile 内に直接埋め込まれるため、ビルドはプロジェクトツリー以外を必要としない。生成は完全オフライン: [`cocoon.lock`](#cocoon-lock) があれば、ロック済みプラグインの解決バージョンと arch ごとの checksum が Dockerfile (`PIN` / `CHECKSUM_*`) に焼き込まれ、ビルドが再現可能になる。

### フラグ

| フラグ | 型 | 説明 |
|---|---|---|
| `--workspace <path>` | string | 設定ファイルのパス (デフォルト: cwd から探索)。 |
| `--output <dir>` | string | 成果物の書き出し先プロジェクトルート (デフォルト: 設定ファイルのディレクトリ)。 |
| `--locked` | bool | 有効なプラグインが `cocoon.lock` エントリ無しで `"latest"` を使っていれば失敗 (再現性 CI 用)。付けない場合、該当プラグインは警告のうえ build 時に最新を解決するフォールバックになる。 |

### 例

```bash
# プロジェクトルートから
cocoon gen

# 別の場所の設定ファイルを指定
cocoon gen --workspace ./infra/cocoon.toml --output ./infra
```

### TLS 証明書

生成される `Dockerfile` / `docker-compose.yml` / `devcontainer.json` に cert 自動取り込み配線が乗るのは、ワークスペースが `[certificates] enable = true` (または `cocoon init --certificates`) で opt-in したときのみ。cert を扱わないワークスペースは cert-free 成果物 (`additional_contexts` なし、`RUN --mount=type=bind` なし、`initializeCommand` なし) を commit する。opt-in 時は `~/.cocoon/certs/*.crt` / `*.cer` が build 時にトラストストアへマージされる。詳細は [`configuration.ja.md` の `[certificates]`](configuration.ja.md#certificates) を参照。

### `cocoon gen workspace`

設定ファイルの `[code_workspace]` セクションから VS Code `.code-workspace` ファイルを生成します。出力先は既定で設定ファイルと同階層 (`.devcontainer/` 配下ではない) なので、`code <name>.code-workspace` で開いてプロジェクトのエントリポイントとして扱えます。`--output <dir>` で出力先を切り替えた場合、folder path は **実際に書き出されるディレクトリ** 起点で相対化されるため、移動先からでも VS Code がパスを正しく解決します。`~` 展開にも対応するので、`"~/.claude"` のようなエントリが VS Code 側で上方向に辿れる相対パスへ解決されます。

`cocoon gen` 本体はこのファイルを生成しません。サブコマンドは opt-in です。

#### フラグ

| フラグ | 型 | 説明 |
|---|---|---|
| `--workspace <path>` | string | 設定ファイルのパス (デフォルト: cwd から探索)。 |
| `--output <dir>` | string | `.code-workspace` を書き出すプロジェクトルート (デフォルト: 設定ファイルのディレクトリ)。 |
| `--name <basename>` | string | 出力ファイル名 (拡張子 `.code-workspace` を除く)。デフォルト: `[code_workspace].name` またはプロジェクトディレクトリの basename。単一のパスセグメントとしてバリデーションされます。 |
| `--folder <path>[=<name>]` | 反復可 | `[code_workspace].folders` の後ろにフォルダを追加。`~` 展開対応。`=<name>` で自動導出された display name を上書き可能。 |

#### 例

```bash
# 設定ファイルの [code_workspace] をそのまま使う
cocoon gen workspace

# 設定ファイルを編集せず一時的にフォルダを足す
cocoon gen workspace --folder ~/.config/nvim --folder ../sibling-repo=Sibling

# 出力ファイル名を上書き
cocoon gen workspace --name my-stack
```

TOML スキーマとパス解決ルールは [`configuration.ja.md` の `[code_workspace]`](configuration.ja.md#code_workspace) を参照。

---

## `cocoon lock`

有効化された `version_capable` プラグインの `[plugins].enable` バージョン pin を、ネットワーク越しに具体的なバージョン (および arch ごとの SHA256 checksum) へ解決し、`cocoon.lock` を workspace ルート (設定ファイルと同階層) に書き出す。以降 `cocoon gen` は `cocoon.lock` をオフラインで消費するため、生成される `.devcontainer/` は再現性を持つ — 同じプラグインバージョン・同じ checksum で、生成時にネットワークを使わない。

- `"latest"` 制約は最新リリースへ凍結される。`"=x.y.z"` の厳密 pin はバージョンを保ったまま arch ごとの checksum を記録する。
- 再実行はべき等。`--upgrade` を渡さない限り、既に lock 済みのエントリは **ネットワークなし** で再利用される。`--upgrade` は `"latest"` 制約を現在の最新リリースへ再解決する。厳密 pin は変化しない。
- lock ファイル名は既定で `cocoon.lock`。設定ファイルの [`[lockfile].name`](configuration.ja.md#lockfile) で別の basename にできる（`cocoon lock` / `cocoon gen` 両方が従う）。

### `cocoon.lock`

生成・コミットされる TOML ファイル。machine-owned なので **手で編集しない** — 代わりに `cocoon lock` を再実行する。トップレベルに `lock_version` (lock フォーマットのバージョン) と `inputs_hash` (有効プラグインとその制約のダイジェスト。`--check` と `cocoon gen --locked` が設定ファイルとのドリフト検出に使う) を持ち、続いて解決済みプラグインごとに 1 つの `[[plugins]]` エントリを持つ:

| フィールド | 意味 |
|---|---|
| `id` | プラグイン id。 |
| `requested` | エントリを生んだ設定ファイルの制約 (`"latest"` または `"=x.y.z"`)。 |
| `version` | 解決された具体的なバージョン。 |
| `checksum_amd64` / `checksum_arm64` | ダウンロードしたアーティファクトの arch ごとの SHA256。fetch 可能な arch ごとの hash を公開しないプラグイン (例: `verify = "pgp"` や `| bash` インストーラ) では省略。 |
| `extra` | プラグインが持つ場合の subcomponent セレクタの凍結値 (例: android-sdk の `api_level`)。 |

### フラグ

| フラグ | 型 | 説明 |
|---|---|---|
| `--workspace <path>` | string | 設定ファイルのパス (デフォルト: cwd から探索)。 |
| `--check` | bool | `cocoon.lock` が設定ファイルと一致するかを **解決せずに** 検証 (ネットワークなし)。ドリフト時は非ゼロ終了 — lock 欠落、`inputs_hash` の変化、有効プラグインの `requested` 記録が一致しない、のいずれか。CI 向け。 |
| `--upgrade` | bool | `"latest"` 制約を現在の最新リリースへ再解決する。厳密 pin は触らない。 |

### exact-only プラグイン

一部のプラグインは上流が machine-readable な「latest」を公開していない: **`aws-cli`** (バージョン無しのダウンロード alias)、**`android-sdk`** (HTML スクレイプのビルド番号)、**`flutter`** (コミットハッシュをキーとするリリース)。これらは `"latest"` を解決できず、`[plugins].enable` 配列でインラインに厳密バージョンを pin する必要がある (例: `"flutter=3.44.1"`)。未 pin や `latest` のままだと `cocoon lock` は pin を促すヒント (`"<id>=<version>"`) と共にエラーになる。

### 例

```console
$ cocoon lock
OK: Locked go 1.23.4
OK: Locked uv 0.5.11
OK: Wrote /home/alice/proj/cocoon.lock (2 plugin(s))
```

生成される `cocoon.lock` (抜粋):

```toml
# cocoon.lock — generated by `cocoon lock`; do not edit by hand.
# Records resolved plugin versions + per-arch checksums for reproducible builds.

lock_version = 1
inputs_hash = "…"

[[plugins]]
id = "go"
requested = "latest"
version = "1.23.4"
checksum_amd64 = "…"
checksum_arm64 = "…"

[[plugins]]
id = "uv"
requested = "latest"
version = "0.5.11"
```

```bash
# "latest" 制約を最新リリースへ更新
cocoon lock --upgrade

# CI ゲート: コミット済み lock が設定ファイルと一致しなければ失敗
cocoon lock --check
```

---

## `cocoon plugin`

cocoon プラグインの参照と作成支援。プラグインは 3 層に分かれ、優先度 **project > user > embedded** で解決される。

| 層 | パス | 出所 |
|---|---|---|
| project | `<workspace>/.cocoon/plugins/<id>/` | overlay（同所で scaffold するか、他 overlay からコピー） |
| user | `~/.cocoon/plugins/<id>/` | overlay（scaffold または他 overlay からコピー） |
| embedded | `internal/plugin/catalog/<id>/` (cocoon ソースリポジトリ内、`go:embed` でバイナリに同梱) | cocoon 同梱カタログ。**単体バイナリでインストールしたユーザーのディスクには存在しない** |

埋め込みプラグインを使うだけなら設定ファイルの `[plugins].enable` に id を並べるだけでよい。embedded を改変したい場合のサポート手順は `cocoon plugin scaffold <new-id>` で新規 id を作りロジックを移植すること。cocoon リポジトリのクローン（または GitHub Release のソース tarball を展開したもの）があるなら `cp -r internal/plugin/catalog/<id> ~/.cocoon/plugins/<id>/` が近道だが、単体バイナリでインストールした場合は embedded ソースがディスクに存在しないため利用できない。

overlay は `gen` 時にのみ参照される。`~/.cocoon/plugins/<id>/` にファイルを置いただけでは有効化されず、`[plugins].enable` への追加が必須。

> プラグインを書く・改修するなら [`docs/plugins.ja.md`](plugins.ja.md) を参照 — `plugin.toml` の全フィールド、`install.sh` / `install_user.sh` の使い分け、バージョン pin の契約がまとまっている。

### `cocoon plugin list`

**目的:** 層解決後にアクセスできるプラグイン ID 一覧を、各 ID の解決元層付きで表示。

**例:**

```console
$ cocoon plugin list
ID            SOURCE    DEFAULT  DESCRIPTION                                  URL
claude-code   embedded  false    Claude Code — AI-powered coding assistant... https://github.com/anthropics/claude-code
go            embedded  false    Go programming language ...                  https://github.com/golang/go
my-internal   user      true     internal CLI ...                             https://git.example.com/team/internal-cli
```

**フラグ:**

| フラグ | 説明 |
|---|---|
| `--source <embedded\|user\|project>` | 単一層に絞って表示 (複数値不可)。 |

**落とし穴:** 同 id が複数層にある場合、最高優先度の層のみ表示される。下位層を見たい場合は overlay ディレクトリを直接削除する（例: `rm -rf ~/.cocoon/plugins/<id>`）。

### `cocoon plugin show <id>`

**目的:** 解決後の `plugin.toml` をパースして安定化された差分しやすい形で再描画し、所属層と併せて表示。

**例:**

```console
$ cocoon plugin show go
id: go
source: embedded
name: Go
description: Go programming language ...
url: https://github.com/golang/go
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

### `cocoon plugin pin <id> <ref>`

**目的:** `version_capable` プラグインのバージョンを設定ファイルの `[plugins].enable` 配列で pin する。pin は配列要素 — `"<id>=<ref>"` — として出力され、プラグインの install スクリプトが `$PIN` から読む。素の `<ref>` (例: `1.23.4`) はバージョンをそのまま書く (`"go=1.23.4"`)、`latest` を渡すと `"go=latest"` が書かれ、範囲 (`>=`, `^` …) は usage error で拒否される。

**例 (デフォルト — stdout, 手動貼り付け):**

```console
$ cocoon plugin pin go 1.23.4
# Add (or update) this entry in the [plugins].enable array in 設定ファイル:

"go=1.23.4"
```

**例 (`--write` — in-place 編集):**

```console
$ cocoon plugin pin go 1.23.4 --write
Updated /home/alice/proj/cocoon.toml: [plugins].enable "go=1.23.4"
```

`--write` は `[plugins].enable` 配列内の `<id>` 要素を upsert する — id が既に有効なら既存の `"<id>"` / `"<id>=..."` 要素を置換し、無ければ新しい要素を追加する — そして配列を canonical なマルチライン形式 (1 行 1 要素) で再出力する。ファイル内の他のコメント・空行は保持される。

**フラグ:**

| フラグ | 説明 |
|---|---|
| `--method <name>` | ref を検証する install メソッド (プラグインが複数宣言する場合)。 |
| `--write` | 設定ファイル (cwd から自動検出) の `[plugins].enable` 配列に pin 要素を upsert する。 |

**落とし穴:**

- pin は `[version].version_capable = true` のプラグインでのみ意味を持つ。それ以外では要素のバージョンが `gen` 時に無視される。
- checksum はここでは pin しない。checksum は `cocoon lock` が `cocoon.lock` に記録する。それまでは install スクリプトの fallback が上流のリリース公開 checksum とダウンロードを照合する。
- `--write` は cwd から設定ファイルを発見できる必要がある。`--write` 無しなら id 検証用に LayeredFS を解決するだけなので、どこからでも動く。
- `--write` は設定ファイルに `[plugins.versions]` セクション (削除済みスキーマ) がまだ残っていると usage error で停止する。まず各 pin を `[plugins].enable` 配列へ移行し — `go = { pin = "1.23.4" }` のようなインラインテーブル pin を要素 `"go=1.23.4"` にして `[plugins.versions]` セクションを削除する — 再実行するか、設定ファイルを手動編集する。

### `cocoon plugin scaffold <id>`

**目的:** 4 種類のテンプレート (catalog method 名語彙) から `plugin.toml` + `install.<category>.sh` 雛形を含む新規 `<id>/` ディレクトリを生成する。新規 project / user スコーププラグインの初期化用。

**例:**

```console
$ cd ~/projects/myapp
$ cocoon plugin scaffold gh-cli \
    --template installer --version-capable \
    --name "GitHub CLI" --description "GitHub CLI" \
    --url "https://cli.github.com" \
    --non-interactive
OK: scaffolded /home/alice/projects/myapp/.cocoon/plugins/gh-cli (2 files)
```

**テンプレート:**

| テンプレート | 用途 | 雛形 |
|---|---|---|
| `installer` | 上流が `curl ... \| bash` 形式 (uv, proto, mise) | `$PIN` バージョン制御。チェックサム検証なし |
| `binary` | 上流が単一バイナリを PATH 配置 (helm, kubectl, terraform) | `$PIN` + `$CHECKSUM_AMD64` / `$CHECKSUM_ARM64` + `dpkg --print-architecture` 分岐 |
| `apt` | apt リポジトリ or `.deb` パッケージ (docker-cli, github-cli, google-chrome) | apt keyring + sources.list 雛形、`$PIN` 配線なし |
| `archive` | 上流がマルチファイル tar/zip をディレクトリツリーに展開 (go, node, zig) | `$PIN` + `$CHECKSUM_AMD64` / `$CHECKSUM_ARM64` + `tar --strip-components=1` |

**フラグ:**

| フラグ | 説明 |
|---|---|
| `--plugins-dir <path>` | 出力ディレクトリ。デフォルト: `<workspace>/.cocoon/plugins` (設定ファイルから自動検出)。 |
| `--name <name>` | 表示名 (例: `"GitHub CLI"`)。 |
| `--description <text>` | 短い説明。上流 URL を埋め込まず、`--url` で渡すこと。 |
| `--url <url>` | 上流プロジェクト URL (`https://...`、空白不可)。`--non-interactive` 時は必須。 |
| `--default` | デフォルト有効化フラグを立てる。 |
| `--requires-root` | install スクリプトを root 実行に。 |
| `--version-capable` | `$PIN` / `$CHECKSUM_*` の雛形を生成。 |
| `--template <kind>` | `installer` \| `binary` \| `apt` \| `archive`。 |
| `--with-install-user` | `install_user.sh` も生成 (`install.<category>.sh` の後に非特権ユーザーで走る)。 |
| `--non-interactive` | プロンプトをスキップ (上記すべて要指定)。 |
| `--force` | `<plugins-dir>/<id>/` が既存なら上書き。 |

**落とし穴:**

- `--plugins-dir` を指定せず、cocoon プロジェクト外 (設定ファイル未発見) で実行すると、`./plugins/<id>/` に黙って書く代わりに actionable error で停止する。
- `--template binary` は `--version-capable` を要求する。`binary` 単体だと拒否される。
- scaffold 後、生成された `plugin.toml` は runtime と同じ strict validator で再ロードされる。失敗 (不正な name、`url` 欠落・形式不正、等) すればディレクトリはロールバックされる。
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
