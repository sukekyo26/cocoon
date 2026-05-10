# プラグイン

このページは **プラグイン作成者向け** の単一ソースドキュメント。
`plugin.toml` のスキーマ、`install.sh` / `install_user.sh` の使い分け、
スクリプトに渡される環境変数、バージョン pin の契約を網羅する。

既存プラグインを単に有効化したいだけなら、`workspace.toml` の
`[plugins].enable` に id を並べるだけでよい。このページの残りは
プラグインを書く・改修する人のためのもの。

## 1. プラグインとは

プラグインは Dockerfile ジェネレータが取り込む自己完結したインストーラ。
必須は `plugin.toml` 1 ファイルだけで、追加で 2 つのシェルスクリプトを
持てる:

- `install.sh` は `docker build` 中に実行される。`[install].requires_root`
  に応じて root または非特権ユーザーで動く。
- `install_user.sh` は **常に非特権ユーザー** として `install.sh` の後に
  実行される。root インストールとユーザー設定を分離する必要がある
  特殊ケースのためだけに存在する（§5）。

プラグインの目的は、cocoon 本体のジェネレータを小さく保ったまま
`[apt].packages` で済まないツール群を追加できるようにすること。

## 2. 3 層 LayeredFS

プラグインは **project > user > embedded** の優先度で層的にロードされる。

| 層 | パス | 備考 |
|---|---|---|
| project  | `<workspace>/.cocoon/plugins/<id>/`              | 全層に勝つ。ディスク上に存在 |
| user     | `~/.cocoon/plugins/<id>/`                        | embedded に勝つ。ディスク上に存在 |
| embedded | `internal/plugin/catalog/<id>/` (cocoon ソースリポジトリ内、`go:embed` でバイナリにコンパイル) | バイナリに同梱。**単体バイナリでインストールしたユーザーのディスクには存在しない** |

同 id のディレクトリは **マージされない** — 最高優先度の層がそのまま勝つ。
どの層が勝っているかは `cocoon plugin list` の `SOURCE` 列、
あるいは `cocoon plugin show <id>` で確認できる。

embedded プラグインを改変したい場合のサポート手順は
**`cocoon plugin scaffold <new-id>` で新しい id の雛形を生成し、
そこにロジックを移植する** こと。cocoon リポジトリのクローンを
持っているなら `cp -r internal/plugin/catalog/<id> ~/.cocoon/plugins/<id>/`
で embedded ソースを overlay に直接コピーする近道もある。
単体バイナリでインストールした場合は embedded ソースがディスク上に
存在しないため、この近道を使うには cocoon を `git clone` するか、
ソース tarball（GitHub Release のソースアーカイブ等）を展開する必要がある。

## 3. ディレクトリ構成

```
plugins/<id>/
├── plugin.toml         # 必須
├── install.sh          # 任意（[install.env] / [install.build_args] /
│                       #   install_user.sh のいずれかで足りるなら省略可）
└── install_user.sh     # 任意（§5 のケースのみ）
```

`install.sh` / `install_user.sh` / `[install.build_args]` / `[install.env]`
の **どれか 1 つ** は必要 — 全部空ならジェネレータは何も出力しない。

## 4. `plugin.toml` スキーマ

| Section | Field | Type | Default | 必須 | 意味 |
|---|---|---|---|---|---|
| `[metadata]` | `name`            | string             | —     | ✓ | 表示名 |
| `[metadata]` | `description`     | string             | —     | ✓ | 説明文。慣習として上流 URL を括弧内に含める (例: `"… (https://example.com)"`)。`cocoon plugin scaffold` は新規生成時にこの慣習を強制するが、ランタイムローダーはチェックしない |
| `[metadata]` | `default`         | bool               | `false` |   | true なら `cocoon init` のデフォルト選択肢に含まれる |
| `[metadata]` | `conflicts`       | list of strings    | `[]`  |   | 同時に enable できない id 群 |
| `[apt]`      | `packages`        | list of strings    | `[]`  |   | `install.sh` の前に apt-get install されるパッケージ |
| `[install]`  | `requires_root`   | bool               | —     | ✓ | true なら `install.sh` を root で実行、false なら非特権ユーザー |
| `[install]`  | `build_args`      | list of strings    | `[]`  |   | プラグインが消費するビルド時変数名群 (例: `DOCKER_GID`)。ジェネレータは対応する `ARG <name>` 行を **`install.sh` の RUN の直前に** 出力し、その per-RUN env prefix に `<name>="${<name>}"` を載せるので、`install.sh` は `$<name>` を通常の環境変数として読める。**`install.sh` を持たないプラグインでは `ARG` 宣言が現状出力されないため**、`install_user.sh` のみ・`[install.env]` のみのプラグインは `build_args` に依存できない。`^[A-Z_][A-Z0-9_]*$` に一致すること |
| `[install]`  | `env`             | map<string,string> | `{}`  |   | install 後に出力される `ENV` 行。値内で先行 `ENV`/`ARG` を参照可 |
| `[install]`  | `volumes`         | list of strings    | `[]`  |   | `/home/${USERNAME}/<dir>` 形式のユーザー所有パス。各エントリは `mkdir -p` + `chown` され、永続化のため docker named volume として宣言される |
| `[version]`  | `version_capable` | bool               | —     | ✓ | true なら `install.sh` が `$PIN` および任意で `$CHECKSUM_AMD64` / `$CHECKSUM_ARM64` を受け取る（§7） |

strict unmarshal: 未知のトップレベルフィールドや既知セクション内の
未知キーは loud に拒否される。`unknown field "foo"` を見たら
`plugin.toml` 内で改名・削除されたフィールドが残っている合図 —
このページの最新スキーマと照合する。

## 5. `install.sh` と `install_user.sh` の使い分け

ほぼ全プラグインは `install.sh` だけで済む。`install_user.sh` を
書くべきは次の **両方** が成立する場合に限る:

1. `[install].requires_root = true`（`install.sh` が root で動く）かつ
2. プラグインがユーザー所有ファイル（`~/.bashrc`、
   `~/.config/<tool>/`、`~/.local/share/`、…）に書き込むか、
   非特権ユーザーで実行すべき初期化コマンド（`<tool> init`、
   `git clone ~/.<tool>`、`conda init bash`、…）を走らせる必要がある。

なぜ分けるか: root のまま `~/.bashrc` に書くと所有者が root になり、
コンテナ起動後にユーザーが編集できなくなる。境界をファイル単位で
明示するのが目的。

### 判断マトリクス

| 状況 | 選択 |
|---|---|
| `requires_root = false`、rc 編集なし | `install.sh` のみ |
| `requires_root = false`、rc 編集あり | `install.sh` のみ — 中で `$RC_FILE` に書く |
| `requires_root = true`、`apt-get` / `/usr/local/bin` のみ | `install.sh` のみ |
| `requires_root = true`、rc 編集 / `~/.config/<tool>` 書込 / `<tool> init` | `install.sh` **と** `install_user.sh` |

### 具体例

| プラグイン | `install.sh` の役割 | `install_user.sh` の役割 |
|---|---|---|
| starship（実在）              | `/usr/local/bin/starship` を root で配置                          | `$RC_FILE` に `eval "$(starship init …)"` を user で追記 |
| fzf（仮想例）                 | `apt-get install fzf` (root)                                       | `git clone https://github.com/junegunn/fzf.git ~/.fzf && ~/.fzf/install --bash` (user) |
| oh-my-zsh（仮想例）           | `apt-get install zsh` (root)                                       | 上流インストーラを実行して `~/.oh-my-zsh` を user で展開 |
| miniconda（仮想例）           | `/opt/conda` を root で展開                                       | `conda init bash` で `$RC_FILE` に複数行のブロックを user で追記 |

rc 編集も user 所有 config の書き込みも要らないなら `install_user.sh`
は不要。実際、現行 catalog で使っているのは starship 1 個だけ。

## 6. install スクリプトに渡される環境変数

`install.sh` も `install_user.sh` も `bash <<'COCOON_PLUGIN_EOF' … COCOON_PLUGIN_EOF`
の中で実行される。heredoc terminator がシングルクォートなので、本体は
BuildKit / `/bin/sh` を **そのまま透過** する — parse / heredoc 読込時点で
`${VAR}` の置換は **行われない**。**bash がスクリプトを実行する時点で、
自身の環境変数を使って `$VAR` を展開する。** プラグイン作者は `${USERNAME}`
や `$RC_FILE` などを普通に書けばよい — bash 実行時に解決される。

その bash の環境は次の 2 ソースから作られる:

- **Per-RUN env prefix**: ジェネレータが生成 Dockerfile の `bash …` 行に
  `NAME="value"` ペアを前置する (例: `RC_FILE="…" bash <<'EOF'`)。
  この変数バインドはその RUN ステップの中だけ有効。
- **Dockerfile `ARG` の env 昇格**: BuildKit は Dockerfile で宣言された
  `ARG` 値を後続 `RUN` 命令の実環境変数として昇格させる (内部用の
  特殊キーを除いて)。`ARG USERNAME` の宣言があれば `$USERNAME` は
  per-RUN prefix なしでも bash から参照できる。

| 変数 | ソース | 意味 |
|---|---|---|
| `RC_FILE`        | per-RUN env、常に | ユーザー login-shell の rc ファイル絶対パス（`/home/<user>/.bashrc`、`/home/<user>/.zshrc`、`~/.config/fish/config.fish`） |
| `RC_SYNTAX`      | per-RUN env、常に | `posix`（bash/zsh）または `fish`。rc 行を出すときの分岐に使う |
| `LOGIN_SHELL`    | per-RUN env、常に | `bash` / `zsh` / `fish` |
| `USERNAME`       | Dockerfile `ARG`、常に | コンテナ内の非特権ユーザー名（生成 Dockerfile 冒頭の `ARG USERNAME` を BuildKit が env に昇格させる） |
| `UID` / `GID`    | Dockerfile `ARG`、常に | 非特権ユーザーの数値 UID / GID（同じく `ARG` 由来） |
| `PIN`            | per-RUN env、`[version].version_capable = true` のときのみ | `workspace.toml` の `[plugins.versions.<id>].pin` の値。空なら upstream 最新を使う |
| `CHECKSUM_AMD64` | per-RUN env、`[version].version_capable = true` のときのみ | amd64 アーティファクトの `sha256`。空ならスクリプトは検証スキップ＋警告 |
| `CHECKSUM_ARM64` | 同上 | arm64 アーティファクトの `sha256` |
| `<BUILD_ARG>`    | per-RUN env (`ARG` 宣言も併発)、`[install].build_args` に列挙され **かつプラグインが `install.sh` を持つ** ときのみ | ジェネレータは `install.sh` の RUN の直前に `ARG <name>` 行を出し、その per-RUN prefix に `<name>="${<name>}"` を載せる。Dockerfile は prefix 行で値を置換する（例: `docker-cli` の `DOCKER_GID`） |

ホスト側での評価は一切行われない — bash はビルド環境内で本体を実行し、
その環境変数は上記 2 ソースから組み立てられる。

リテラル文字列 `COCOON_PLUGIN_EOF` を `install.sh` / `install_user.sh`
内で **行頭から行末まで一致する形で書いてはならない**。書くと
heredoc が早閉じしてビルドが壊れる。ジェネレータはこれを検出して
loud に失敗する（`ErrHeredocCollision`）。

## 7. バージョン対応プラグイン（`version_capable = true`）

versioned プラグインは次の契約を守る:

- `$PIN` を読みバージョンとして使う。空なら upstream 最新に
  fallback（curl リダイレクト・GitHub API など）。
- `$CHECKSUM_AMD64` / `$CHECKSUM_ARM64` が非空なら sha256 検証する。
  空のときは **明示的に WARNING を出力** する（無言でスキップしない）。
- `dpkg --print-architecture` 等でアーキを判定し、対応するアーティファクト
  と対応するチェックサム変数を選ぶ。

ユーザー側は `workspace.toml` の `[plugins.versions.<id>]` で pin を記録する:

```toml
[plugins.versions.go]
pin            = "1.23.4"
checksum_amd64 = "abc..."
checksum_arm64 = "def..."
```

`cocoon plugin pin <id> <ref> --write` がこのブロックを生成・挿入
（または既存ブロックを置換）する。フラグ詳細は `docs/commands.ja.md` 参照。

`version_capable = false` のプラグインは `$PIN` を完全に無視するため、
pin ブロックを書いても `gen` 時に意味を持たない。

## 8. Catalog ツアー

自作プラグインの参考にできる embedded プラグイン:

- **`go`** — `tarball` テンプレート + `[install.env]` 多用。
  `$PIN` + `$CHECKSUM_*` + アーキ分岐の典型。
- **`docker-cli`** — `[install].build_args` を使う唯一の catalog プラグイン
  （`DOCKER_GID` をホストから受ける）。host 由来の値を渡す参考に。
- **`proto`** — `curl-pipe` テンプレート。upstream installer に任せる
  最小構成だが `$PIN` で版指定はする。
- **`starship`** — catalog 内で唯一 `install_user.sh` を持つ。root → user
  境界の見本として両ファイルを併読する。
- **`lazygit`** — `tarball` テンプレート、`[install.env]` 無し。
  versioned プラグインの最小構成。

## 9. トラブルシューティング

- **`cocoon plugin show` / `gen` で `unknown field "<x>"`** —
  `plugin.toml` に改名・削除されたフィールドが残っている。§4 の
  最新スキーマと照合する。
- **`ErrHeredocCollision: plugin "x" contains the literal
  COCOON_PLUGIN_EOF`** — `install.sh` 内で heredoc terminator と
  完全一致する行がある。スクリプト内のマーカー名を変える。
- **`ErrNilPluginsFS`** — 内部実装エラー: caller が
  `WorkspaceContext` を作ったが layered plugin FS をワイヤしていない。
  cocoon 本体の統合コードを触っているときだけ表れる。
- **`cocoon plugin show <id>` で "not found in any layer"** —
  embedded catalog にも overlay にもその id が無い。ディレクトリ名と
  渡している id が一致しているか確認する。
- **volume パスが拒否される** — `[install].volumes` のエントリは
  `^/home/\$\{USERNAME\}/[^/]+$` に一致する必要がある。
  `/home/${USERNAME}/` 直下の単一セグメントのみ可で、ネストパスは不可。
  ネストが要るなら install スクリプト側で `mkdir -p $HOME/.cache/foo/bar`
  のように作る。

## 関連ドキュメント

- [`architecture.ja.md`](architecture.ja.md) — LayeredFS と inline-heredoc
  設計の why。
- [`configuration.ja.md`](configuration.ja.md) — `workspace.toml` の
  `[plugins]` / `[plugins.versions]` セクション。
- [`commands.ja.md`](commands.ja.md) — `cocoon plugin <subcmd>` リファレンス。
