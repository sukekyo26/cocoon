# 設定 (`workspace.toml`)

`workspace.toml` は `cocoon gen` の唯一の入力です。本ページではスキーマが受理する全セクション・全フィールドを説明します。

`cocoon init` は妥当なデフォルトとコメントアウト済の雛形を含むファイルを書き出すので、多くの場合は生成物を編集するだけで済みます。本リファレンスは必要に応じて参照してください。

## 探索順序

`cocoon gen` はカレントディレクトリから上方向に `workspace.toml` を探します:

1. `<cwd>/workspace.toml`
2. `<cwd>/.cocoon/workspace.toml`
3. 親ディレクトリの `workspace.toml`、続いて `.cocoon/workspace.toml`、というように上昇
4. `.git` 境界か `$HOME` で停止

最初に見つかったものを使います。`cocoon gen --workspace <path>` で探索を上書きできます。

## セクション一覧

| セクション | 必須? | 役割 |
|---|---|---|
| `[workspace]` | optional | 生成全体の挙動 (マウント範囲、Dev Container 出力切替) |
| `[container]` | **required** | イメージの素性 (サービス名、ユーザー名、OS / バージョン) |
| `[container.resources]` | optional | Compose のリソース上限 |
| `[container.shell]` | optional | ログインシェル + シェル別 rc 注入 |
| `[container.hosts]` | optional | `/etc/hosts` 追加エントリ |
| `[container.dns]` | optional | カスタム DNS リゾルバと検索ドメイン |
| `[container.sysctls]` | optional | カーネルパラメータ |
| `[container.capabilities]` | optional | Linux capabilities 追加 / 剥奪 |
| `[container.security_opt]` | optional | Compose の `security_opt` |
| `[[container.skel]]` | optional | `/etc/skel` 経由で配置する dotfiles |
| `[plugins]` | **required** | 有効化するプラグイン |
| `[plugins.versions]` | optional | プラグインのバージョン固定 + チェックサム |
| `[apt]` | optional | 追加 apt パッケージ |
| `[apt.mirror]` | optional | 地域 apt ミラー |
| `[apt.proxy]` | optional | apt-get の HTTP/HTTPS プロキシ |
| `[[apt.sources]]` | optional | サードパーティ apt リポジトリ |
| `[ports]` | optional | ホストポート転送 |
| `[volumes]` | optional | named Docker ボリューム |
| `[env]` | optional | コンテナ環境変数 |
| `[[mounts]]` | optional | 追加バインドマウント |
| `[home_files]` | optional | `~/` 配下のファイル単位 bind mount |
| `[locale]` | optional | タイムゾーンと言語 |
| `[dockerfile]` | optional | Dockerfile への独自フラグメント注入 |
| `[services.<name>]` | optional | サイドカーサービス |
| `[devcontainer.*]` | optional | `devcontainer.json` への pass-through |

[`[git]`](#廃止セクション) と [`[repositories]`](#廃止セクション) はパーサが受理するものの非推奨です。新規プロジェクトでは使わないでください。

---

## `[workspace]`

生成全体の挙動。すべて optional、未設定時はデフォルトが適用されます。

| フィールド | 型 | デフォルト | 説明 |
|---|---|---|---|
| `mount_root` | string | `"."` | `"."` は cwd をプロジェクトとしてマウント、`".."` は親ディレクトリをマウントして兄弟リポジトリも見える状態にする。 |
| `devcontainer` | bool | `true` | VS Code Reopen-in-Container 用の `.devcontainer/devcontainer.json` を生成。 |

```toml
[workspace]
mount_root = "."
devcontainer = true
```

---

## `[container]` (必須)

イメージの素性。`service_name` / `username` / `os` / `os_version` は必須。

| フィールド | 型 | バリデーション | 説明 |
|---|---|---|---|
| `service_name` | string | `^[a-z][a-z0-9_-]*$` | Compose の `services:` キー。`docker compose exec <service_name>` で参照される。 |
| `username` | string | `^[a-z_][a-z0-9_-]*$` | コンテナ内に作成される Linux ユーザー。 |
| `os` | string | `ubuntu` \| `debian` | ベースディストリビューション。 |
| `os_version` | string | 選択した `os` に対応する版 (下記) | ディストロのバージョン (例: `26.04` / `13`)。 |
| `docker_socket` | bool | — | `/var/run/docker.sock` をマウントして docker-in-docker を有効化。デフォルト `false`。 |

**サポートされる OS / バージョンの組合せ:**

| `os` | `os_version` |
|---|---|
| `ubuntu` | `26.04`, `24.04`, `22.04` |
| `debian` | `13`, `12` |

```toml
[container]
service_name = "myapp"
username = "dev"
os = "ubuntu"
os_version = "26.04"
```

### `[container.resources]`

Compose のリソース上限。未設定なら Docker のデフォルト (無制限) を継承。

| フィールド | 型 | 例 |
|---|---|---|
| `shm_size` | string | `"2gb"` |
| `pids_limit` | int | `2048` |
| `cpus` | float | `2.0` |
| `memory` | string | `"4gb"` |

### `[container.shell]`

ログインシェルとシェル別 rc 注入。aliases / env はイメージビルド時にコンテナ内 rc ファイルへ追記されます。bash と zsh は POSIX 記法 (`alias k='v'`、`export K=V`) を共有、fish は自動翻訳。

| フィールド | 型 | デフォルト | 備考 |
|---|---|---|---|
| `default` | string | `"bash"` | `bash` / `zsh` / `fish` のいずれか。 |
| `aliases` | inline table | — | エイリアスキーは `^[a-zA-Z_][a-zA-Z0-9_-]*$`。 |
| `env` | inline table | — | 環境変数キーは `^[A-Z_][A-Z0-9_]*$` (大文字)。 |

```toml
[container.shell]
default = "bash"
aliases = { ll = "ls -lah", gs = "git status" }
env     = { EDITOR = "vim", PAGER = "less -R" }
```

> `EDITOR=vim` / `nano` は apt カテゴリ `text-editors` の有効化が前提。`EDITOR=code` は VS Code Dev Containers から起動したとき (VS Code が `code` シムを注入) に使えます。`PAGER=less` は apt カテゴリ `utilities` が前提。

`[container.shell]` はリポジトリにチェックインするプロジェクト共通設定向けです。**個人ごと、コンテナリビルドを跨いで永続化したい**設定は、起動時に rc ファイルから自動 source される `~/.cocoon/.shellrc` (fish の場合 `~/.cocoon/.shellrc.fish`) に書きます。このパスは Docker named volume (`cocoon`) でバックされており、`docker compose down && up --build` を跨いでも編集が残ります。コンテナ内から編集してください:

```bash
docker compose -f .devcontainer/docker-compose.yml exec dev bash -lc 'vim ~/.cocoon/.shellrc'
```

(エディタはイメージにインストールされているものを選んでください。`bash -lc` を経由することでコンテナ内の `EDITOR` / `PATH` が解決されます。`"$EDITOR"` を直接渡すとホスト側で展開されてしまいます。)

`docker compose down -v` でのみリセットされます。このパスは**コンテナ内専用**で、ホスト `~/.cocoon/` とは無関係です (ホスト側は cocoon CLI のプラグインオーバーレイ・ビルドコンテキスト・証明書置き場として独立)。

### `[container.hosts]`

`/etc/hosts` の追加エントリ。キーはホスト名 (RFC 1123)、値は IPv4 / IPv6 アドレスまたはリテラル `"host-gateway"` (ホスト機を指す)。

```toml
[container.hosts]
"db.local"     = "host-gateway"
"corp.example" = "10.0.0.42"
```

### `[container.dns]`

カスタム DNS 設定。

| フィールド | 型 | 備考 |
|---|---|---|
| `servers` | array of strings | IPv4 / IPv6 として検証。 |
| `search` | array of strings | RFC 1123 ホスト名として検証。 |

### `[container.sysctls]`

Compose にそのまま渡されるカーネルパラメータ。キーは `^[a-z][a-z0-9._-]*$`、値は int か string。

```toml
[container.sysctls]
"vm.max_map_count" = 262144
```

### `[container.capabilities]`

Linux capabilities。名前は `CAP_` プレフィックスなし、すべて大文字。

```toml
[container.capabilities]
add  = ["SYS_PTRACE"]
drop = ["AUDIT_WRITE"]
```

### `[container.security_opt]`

| フィールド | 型 | 備考 |
|---|---|---|
| `seccomp` | string | 例: `"unconfined"`。設定する場合は空文字列不可。 |
| `apparmor` | string | 同上。 |
| `no_new_privileges` | bool | setuid 権限昇格を遮断。 |

### `[[container.skel]]`

`/etc/skel` 経由で新規ユーザーのホームに dotfiles を配置。配列なので複数指定可能。

| フィールド | 型 | バリデーション |
|---|---|---|
| `source` | string | ワークスペースルート相対。先頭 `/` 不可、`..` セグメント不可、空白文字不可。 |
| `target` | string | `/etc/skel` 相対。同上。 |

```toml
[[container.skel]]
source = ".cocoon/skel/example.bashrc"
target = ".bashrc"
```

---

## `[plugins]` (必須)

| フィールド | 型 | バリデーション |
|---|---|---|
| `enable` | array of strings | プラグイン ID は `^[a-z][a-z0-9-]*$`。重複不可。 |

```toml
[plugins]
enable = ["go", "uv", "github-cli"]
```

利用可能なプラグイン一覧は `cocoon plugin list` で確認できます (埋め込み + ユーザー / プロジェクト上書き含む)。

### `[plugins.versions]`

`version_capable` プラグインのバージョン固定。チェックサム (64 文字の小文字 hex) で install tarball を検証可能 (任意)。

```toml
[plugins.versions]
go = { pin = "1.22.5" }
uv = { pin = "0.5.7", checksum_amd64 = "<sha256>", checksum_arm64 = "<sha256>" }
```

---

## `[apt]`

| フィールド | 型 | 説明 |
|---|---|---|
| `packages` | array of strings | cocoon の最小ベース + init で選択したカテゴリに追加する Debian パッケージ。 |

### `[apt.mirror]`

| フィールド | 型 | バリデーション |
|---|---|---|
| `url` | string | `http://` または `https://` で始まる必要あり。生成 Dockerfile の sed 処理を壊さないため空白・`'`・`|`・`&`・`\` 不可。 |

### `[apt.proxy]`

| フィールド | 型 | 備考 |
|---|---|---|
| `http` | string | `http://` または `https://`。 |
| `https` | string | 同上。 |

### `[[apt.sources]]`

signed-by GPG キー方式のサードパーティ apt リポジトリ。`key_url` から取得したキーを `/etc/apt/keyrings/` 配下に dearmor して配置。

| フィールド | 型 | 備考 |
|---|---|---|
| `name` | string | `^[a-z][a-z0-9-]*$`。一意。 |
| `suite` | string | `^[a-z][a-z0-9._-]*$`。 |
| `components` | array of strings | 1 件以上。各要素は `^[a-z][a-z0-9_-]*$`。 |
| `url` | string | `http://` または `https://`。 |
| `key_url` | string | `http://` または `https://`。 |
| `arch` | string | `amd64` \| `arm64` \| `i386` \| `armhf` \| `ppc64el` \| `s390x` (任意)。 |

---

## `[ports]`

| フィールド | 型 | 説明 |
|---|---|---|
| `forward` | array | Compose short-form 文字列 (`"3000:3000"`、`"127.0.0.1:5432:5432/tcp"`、レンジ `"3000-3005:3000-3005"`) または `target` / `published` / `host_ip` / `protocol` / `mode` を持つ long-form テーブル。 |

```toml
[ports]
forward = ["3000:3000", "5432:5432"]
```

---

## `[volumes]`

コンテナホーム配下にマウントする named Docker ボリューム。書式: `<ボリューム名> = <コンテナ内パス>`。

```toml
[volumes]
my-data = "/home/${USERNAME}/.my-tool"
```

---

## `[env]`

コンテナへ渡す環境変数。キーは `^[A-Za-z_][A-Za-z0-9_]*$`。値中の `${VAR}` は `cocoon gen` 実行時にホストの環境変数を解決します。

```toml
[env]
OPENAI_API_KEY = "${OPENAI_API_KEY}"
DEBUG          = "1"
```

---

## `[[mounts]]`

ホストからコンテナへの追加バインドマウント。配列なので複数指定可能。

| フィールド | 型 | 必須 | 説明 |
|---|---|---|---|
| `source` | string | yes | ホスト側パス。`~` 可。空文字列不可。 |
| `target` | string | yes | コンテナ側パス。絶対パスのみ。 |
| `readonly` | bool | no | デフォルト `false`。 |

```toml
[[mounts]]
source   = "~/.ssh"
target   = "/home/${USERNAME}/.ssh"
readonly = true
```

---

## TLS 証明書 (`~/.cocoon/certs/`)

社内 CA (Zscaler、企業プロキシ、開発用自己署名 CA 等) をコンテナで信頼させたい場合、PEM 形式の `.crt` ファイルを **`~/.cocoon/certs/`** に置きます。コンテナビルド時に自動的にトラストストアへ取り込まれます。

```sh
mkdir -p ~/.cocoon/certs
cp /path/to/corp-ca.crt ~/.cocoon/certs/
docker compose -f .devcontainer/docker-compose.yml build
```

このディレクトリは `workspace.toml` のセクションではなくホスト側のグローバル設定です。**複数の cocoon プロジェクトで同じ corp CA を共有できます** (プロジェクトごとに証明書をコピーする必要はありません)。

### チーム運用シナリオ

cocoon の生成物 `.devcontainer/*` は、証明書の有無に関わらず **常に同じ内容** です。1 人のメンバーが `cocoon gen` で生成・コミットすれば、他のメンバーは cocoon バイナリ無しでもそのまま dev container を利用できます。

| メンバー | cocoon バイナリ | `~/.cocoon/certs/` 作成 | 必要な操作 |
|---|---|---|---|
| 生成担当 | あり | 自身で `mkdir -p ~/.cocoon/certs` | `cocoon gen && commit` |
| VS Code 利用者 (cert 不要) | 不要 | `initializeCommand` が自動作成 | なし。dev container を開くだけ |
| VS Code 利用者 (cert 必要) | 不要 | `initializeCommand` が自動作成 | `cp corp.crt ~/.cocoon/certs/` して Rebuild Container |
| `docker compose` 直接利用 / CI | 不要 | **手動 `mkdir -p ~/.cocoon/certs`** | 初回のみ手動 mkdir、cert 必要なら配置して build |

> **Note**: VS Code Dev Containers を使わずに `docker compose build` を直接実行する場合は、初回のみホスト側で `mkdir -p ~/.cocoon/certs` を実行してください。VS Code 経由のメンバーは `initializeCommand` により自動作成されます。CI 環境ではセットアップステップに `mkdir -p ~/.cocoon/certs` を 1 行追加してください。

### 仕組み

- `.devcontainer/docker-compose.yml`: `additional_contexts: cocoon_user_certs: ${HOME}/.cocoon/certs` により `~/.cocoon/certs/` をビルドコンテキストとして直接参照 (コピー無し)。
- `.devcontainer/Dockerfile`: `RUN --mount=type=bind,from=cocoon_user_certs ... if find ... ; then ... update-ca-certificates ; fi` により build 時に `*.crt` を取り込む。apt install より前に実行されるため、Zscaler 等の TLS インターセプト下でも build が通る。
- `.devcontainer/devcontainer.json`: `initializeCommand: "mkdir -p ${HOME}/.cocoon/certs"` により VS Code Dev Containers がコンテナ作成前にホスト側ディレクトリを自動作成する。

環境変数 `SSL_CERT_FILE` / `CURL_CA_BUNDLE` / `REQUESTS_CA_BUNDLE` / `NODE_EXTRA_CA_CERTS` は、マージ済み trust store (`/etc/ssl/certs/ca-certificates.crt`) を指すように常時設定されます (証明書の有無に依らず安全)。

### 注意点

- `~/.cocoon/certs/` 配下のファイルは **すべて** ビルドコンテキストとして BuildKit に渡されます。`.crt` 以外 (特に秘密鍵 `.key` 等) を置かないでください。
- 証明書を更新したらコンテナを rebuild してください。BuildKit が bind-mount 内容ハッシュを cache key に含めるため、自動的に層が再構築されます。

---

## `[home_files]`

ファイル単位 bind mount による永続化。各パスは `~/` 相対 (先頭 `/` 不可、`~` 不可、`..` 不可)。ホスト側の `~/.gitconfig` 等をコンテナ内で共有する用途。

```toml
[home_files]
files = [".gitconfig", ".claude.json"]
```

---

## `[locale]`

| フィールド | 型 | デフォルト | 備考 |
|---|---|---|---|
| `timezone` | string | ホスト準拠 | IANA タイムゾーン (例: `"Asia/Tokyo"`)。 |
| `lang` | string | `"en_US.UTF-8"` | パターン `^[a-z]{2,3}_[A-Z]{2}\.UTF-8$`。 |

---

## `[dockerfile]`

Dockerfile の所定フックポイントへ独自フラグメントを注入。注入された内容はそのまま使われるので、各 RUN コマンドは自分で検証してください。

| フィールド | 型 | 実行タイミング |
|---|---|---|
| `pre_user_setup` | string | `useradd` の前。 |
| `post_plugins` | string | 各プラグインの `install.sh` の後。 |

```toml
[dockerfile]
pre_user_setup = """
RUN apt-get update && apt-get install -y my-extra-pkg
"""
```

---

## `[services.<name>]`

同じ Compose ネットワーク上のサイドカー (postgres / redis 等)。各 `<name>` は `^[a-z][a-z0-9_-]*$`、`[container].service_name` と衝突不可。

| フィールド | 型 | 必須 | 備考 |
|---|---|---|---|
| `image` | string | yes | OCI イメージ。空文字列不可。 |
| `ports` | array | no | Compose のポート指定。 |
| `env` | map | no | キーは `^[A-Za-z_][A-Za-z0-9_]*$`。 |
| `volumes` | map | no | 値は絶対パス。予約キー `local` は不可。 |
| `mounts` | array of tables | no | `[[mounts]]` と同形式。 |
| `command` | string or array | no | イメージの CMD を上書き。 |
| `depends_on` | array of strings | no | 他のサイドカー名のみ。自分自身やメインサービスは不可。 |
| `healthcheck` | table | no | Compose にそのまま転送。 |
| `restart` | string | no | `no` / `always` / `on-failure` / `unless-stopped` のいずれか。 |

```toml
[services.postgres]
image       = "postgres:16-alpine"
environment = { POSTGRES_PASSWORD = "dev" }
ports       = ["5432:5432"]
```

---

## `[devcontainer.*]`

`[devcontainer.*]` 配下のすべてが、生成 `devcontainer.json` にそのままマージされます。`[workspace] devcontainer = false` のときは無視。

```toml
[devcontainer.customizations.vscode]
extensions = [
    "ms-azuretools.vscode-docker",
    "eamodio.gitlens",
]
```

---

## 廃止セクション

下記セクションは後方互換のためパーサが受理しますが、将来のメジャーリリースで削除される可能性があります。

### `[git]`

代わりに [`[home_files]`](#home_files) を使ってください。ホストの `~/.gitconfig` を bind mount すれば git identity を一元管理できます。

### `[repositories]`

複数リポジトリの "fat" ワークスペースには、親ディレクトリ配下に手動で `git clone` してから `mount_root = ".."` を使ってください。
