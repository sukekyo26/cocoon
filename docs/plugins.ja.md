# プラグイン

> [!WARNING]
> cocoon は v0.x（alpha）開発段階です。お使いになる場合は、1.0 までにプラグイン契約（`plugin.toml` スキーマ・install スクリプトに渡される環境変数・バージョン pin の形式）が変更され得ること、各リリースに breaking change が含まれうることをご了承のうえご利用ください。詳細は [CHANGELOG](CHANGELOG.ja.md) と README の「プロジェクトステータス」を参照してください。

このページは **プラグイン作成者向け** の単一ソースドキュメント。
`plugin.toml` のスキーマ、`install.sh` / `install_user.sh` の使い分け、
スクリプトに渡される環境変数、バージョン pin の契約を網羅する。

既存プラグインを単に有効化したいだけなら、`workspace.toml` の
`[plugins].enable` に id を並べるだけでよい。このページの残りは
プラグインを書く・改修する人のためのもの。

## プラグインとは

プラグインは Dockerfile ジェネレータが取り込む自己完結したインストーラ。
必須は `plugin.toml` 1 ファイルだけで、追加で 2 つのシェルスクリプトを
持てる:

- `install.sh` は `docker build` 中に実行される。`[install].requires_root`
  に応じて root または非特権ユーザーで動く。
- `install_user.sh` は **常に非特権ユーザー** として `install.sh` の後に
  実行される。root インストールとユーザー設定を分離する必要がある
  特殊ケースのためだけに存在する（後述の「`install.sh` と `install_user.sh` の使い分け」を参照）。

プラグインの目的は、cocoon 本体のジェネレータを小さく保ったまま
`[apt].packages` で済まないツール群を追加できるようにすること。

## 3 層 LayeredFS

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

## ディレクトリ構成

```
plugins/<id>/
├── plugin.toml         # 必須
├── install.<name>.sh   # 必須 — [install.methods.<name>] 宣言ごとに 1 ファイル
└── install_user.sh     # 任意（plugin スコープ、選ばれた install.<name>.sh の実行後に走る）
```

すべてのプラグインは `plugin.toml` に少なくとも 1 つの
`[install.methods.<name>]` を宣言し、対応する `install.<name>.sh` を
配置する。**`install.sh` という名前は廃止** — リテラル `install.sh`
があるとロード時に reject され、エラーメッセージにリネーム手順が
出る。method を 1 つしか持たないプラグインも `[install.methods.<name>]`
を 1 つ宣言する（後述の「method 名のカテゴリ規約」から `<name>` を選ぶ）。

カテゴリ語彙は `cocoon plugin scaffold --template <name>` のフラグ値
としても使われる。

プラグインの **install snippet**（生成 Dockerfile 上の
`# Install …` コメント + RUN ブロック）が出力されるのは、次のいずれかが
存在するときだけ:

- `install.<name>.sh`（少なくとも 1 本 — loader 必須）
- `install_user.sh`
- `[install.env]`

`[install.build_args]` 単独では何も出力されない: `ARG` 宣言は hook の
直前にしか emit されないため、`build_args` だけのプラグインは
誰も参照しない `ARG` 名を持つだけになる。`build_args` は
`install.<name>.sh` または `install_user.sh` から参照されて初めて意味を持つ。

## method 名のカテゴリ規約

`[install.methods.<name>]` と対応する `install.<name>.sh` の `<name>`
は、catalog 全体で **同じ 4 カテゴリ語彙** に揃える。plugin 固有の名前
（例: `gh-cli`、`bun-sh`）は workspace.toml の `[plugins.methods]` を
読むユーザーに毎回 plugin.toml を見せる手間を強いる。

| カテゴリ | 選ぶ基準 |
|:---|:---|
| `binary`    | 単一 binary を /usr/local/bin 等に配置するだけ。tar/zip から「1 個だけ取り出す」も該当 (kubectl, helm, terraform, shellcheck) |
| `installer` | vendor の curl-to-bash インストーラを pipe する。失敗ドメインは vendor 固有 (bun.sh, sh.rustup.rs, mise.run, astral.sh, gh.io) |
| `apt`       | apt repository 登録 or .deb 直 install (docker-cli, github-cli, google-chrome) |
| `archive`   | 複数ファイルの tar/zip を展開してディレクトリツリーを作る (bin + lib + share)、または archive 内 installer 実行 (go, node, zig, nerd-fonts, aws-cli) |

**catalog の plugin は必ずこの 4 つから選ぶ。** `~/.cocoon/plugins/<id>/`
や `<workspace>/.cocoon/plugins/<id>/` のカスタムプラグインは、上記 4
語彙に当てはまらない真に新しいインストール経路の場合に限り、validator
の文字種制約 (`^[a-z][a-z0-9_-]*$`) を満たす他の名前も技術的には使える
— ただしオーバーレイでも 4 語彙を優先することを強く推奨。それ以外は
レビューで指摘する対象。

`[install].volumes` のみを宣言したプラグイン（install hook も env も
無し）でも、生成成果物には影響する — `volumes` は別経路を通り、
install フェーズ冒頭の `mkdir -p` / `chown` ブロックと
`docker-compose.yml` の named volume 宣言を生む。プラグイン固有の
install snippet が出ないだけ。

## `plugin.toml` スキーマ

| Section | Field | Type | Default | 必須 | 意味 |
|---|---|---|---|---|---|
| `[metadata]` | `name`            | string             | —     | ✓ | 表示名 |
| `[metadata]` | `description`     | string             | —     | ✓ | 短い説明文。上流 URL を埋め込まない — 専用フィールド `url` を使う |
| `[metadata]` | `url`             | string             | —     | ✓ | 上流プロジェクト URL (`https://...`、空白不可)。`cocoon init` のバージョン入力プロンプト、`cocoon plugin show` の `url:` 行、`cocoon plugin list` の `URL` 列で表示される |
| `[metadata]` | `default`         | bool               | `false` |   | true なら `cocoon init` のデフォルト選択肢に含まれる |
| `[metadata]` | `conflicts`       | list of strings    | `[]`  |   | 同時に enable できない id 群 |
| `[apt]`      | `packages`        | list of strings    | `[]`  |   | `install.sh` の前に apt-get install されるパッケージ |
| `[install]`  | `requires_root`   | bool               | —     | ✓ | true なら `install.sh` を root で実行、false なら非特権ユーザー |
| `[install]`  | `build_args`      | list of strings    | `[]`  |   | プラグインが消費するビルド時変数名群。ジェネレータは `ARG <name>` 行をプラグインごとに 1 回 (`install.sh` / `install_user.sh` のうち先に走る方の直前) 出力し、両 hook の per-RUN env prefix に `<name>="${<name>}"` を載せる。これにより `install.sh` も `install_user.sh` も `$<name>` を通常の環境変数として読める。ARG のスコープは stage 全体なので、宣言は 1 回で両 RUN をカバーする。`^[A-Z_][A-Z0-9_]*$` に一致し、cocoon 予約 env 名 (`PIN` / `CHECKSUM_AMD64` / `CHECKSUM_ARM64` / `RC_FILE` / `RC_SYNTAX` / `LOGIN_SHELL` / `COCOON_INSTALL_METHOD` / `USERNAME`) と衝突しないこと — `build_args` ペアは framework value の後に RUN プレフィックスへ追加されるため、衝突すると silent shadow になる |
| `[install]`  | `env`             | map<string,string> | `{}`  |   | install 後に出力される `ENV` 行。値内で先行 `ENV`/`ARG` を参照可 |
| `[install]`  | `volumes`         | list of strings    | `[]`  |   | `/home/${USERNAME}/<dir>` 形式のユーザー所有パス。各エントリは `mkdir -p` + `chown` され、永続化のため docker named volume として宣言される |
| `[install]`  | `default_method`  | string             | —     | ✓ | `workspace.toml` の `[plugins.methods]` で指定が無いときに採用される method 名。`[install.methods]` のキー名と一致する必要がある。`[install.methods]` 必須化のためこのフィールドも必須 (method 1 つだけのプラグインも 1 エントリ宣言する) |
| `[install.methods.<name>]` | `description` | string | — | ✓ | `cocoon init` の method ピッカーに表示される一行説明。`<name>` は `^[a-z][a-z0-9_-]*$` に一致し、対応する `install.<name>.sh` がディスクに存在しなければならない。**少なくとも 1 エントリ必須** — legacy `install.sh` は廃止。`<name>` は上記の 4 カテゴリ語彙から選ぶ |
| `[install.extra_versions.<key>]` | `env`, `default` | inline table | — |   | ユーザーが上書き可能なサブコンポーネント版を宣言する。`<key>` は `^[a-z][a-z0-9_]*$`；`env` は install スクリプトに渡る変数名（`^[A-Z_][A-Z0-9_]*$`、予約 env 名や `build_args` と衝突不可）；`default` は `[plugins.versions].<id>` で key が未指定のときに使われる値。詳細は「サブコンポーネントバージョン」節を参照 |
| `[version]`  | `version_capable` | bool               | —     | ✓ | true なら install スクリプトが `$PIN` を受け取りバージョン固定できる（後述の「バージョン対応プラグイン」を参照） |
| `[version]`  | `verify`          | string             | `"checksum"` |   | 整合性検証方式: `"checksum"`（install スクリプトが `$CHECKSUM_AMD64` / `$CHECKSUM_ARM64` を検証）または `"pgp"`（スクリプトが同梱署名鍵で in-script 検証し、workspace 側 checksum を取らない）。`version_capable = true` のときのみ意味を持つ |

strict unmarshal: 未知のトップレベルフィールドや既知セクション内の
未知キーは loud に拒否される。`unknown field "foo"` を見たら
`plugin.toml` 内で改名・削除されたフィールドが残っている合図 —
このページの最新スキーマと照合する。

## `install.sh` と `install_user.sh` の使い分け

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

## 複数インストール方式（`[install.methods]`）

すべてのプラグインは少なくとも 1 つの `[install.methods.<name>]` を
宣言する。複数のインストール経路を提供したいプラグインは、単に
複数エントリを宣言すればよい。宣言した `<name>` ごとに対応する
`install.<name>.sh` がディスクに必要 (loader が enforce)。

```toml
# plugin.toml — copilot-cli は 2 method
[install]
requires_root = false
default_method = "installer"

[install.methods.installer]
description = "Install via the upstream gh.io install script (default)"

[install.methods.binary]
description = "Direct binary from GitHub Releases (Zscaler-friendly)"
```

対応する `install.installer.sh` / `install.binary.sh` を置けば、2 つの
インストール経路が選べるようになる。選択は `workspace.toml` に保存される:

```toml
[plugins.methods]
copilot-cli = "binary"  # ここに無いプラグインは default_method を使う
```

`cocoon init` は宣言された method が **2 つ以上** あるプラグインに対して
のみ per-plugin の method ピッカーを表示する (1 つしか持たないプラグインは
選択肢が無いのでサイレントにスキップ)。ピッカーの初期選択行はプラグイン
の `default_method` なので、推奨どおりで良ければそのまま Enter で確定。

選ばれたスクリプトには通常の env (`$PIN` / `$CHECKSUM_AMD64` /
`$CHECKSUM_ARM64` / `$RC_FILE` / `$RC_SYNTAX` / `$LOGIN_SHELL` /
build-arg 経由の値) に加えて、次の値が渡される:

| 変数 | 意味 |
|---|---|
| `$COCOON_INSTALL_METHOD` | 選ばれた method 名（例: `"binary"`）。全プラグインが `[install.methods]` を宣言するため常にセットされる |

**複数 method を持つプラグイン**のスクリプトは
`: "${COCOON_INSTALL_METHOD:?missing}"` で fail-fast し、さらに自分の期待値と
一致しなければ即 exit する作りにしておくと、スクリプトを rename した後に
`workspace.toml` 側の旧参照が残っていても誤ったアーティファクトを引っ張る前
に検出できる。**シングル method プラグイン**はこのチェック不要 — loader の
`[install.methods]` 必須化により `$COCOON_INSTALL_METHOD` は必ずセットされ、
間違える対象の sibling スクリプトも存在しない。実装例は
`internal/plugin/catalog/copilot-cli/install.installer.sh` と
`install.binary.sh`。

**pin / checksum のスコープ。** pin は `[plugins.versions]` 配下に
書かれ、**プラグイン単位（method 別ではない）** で保持される — catalog
の `plugin.toml` を method 別 checksum で肥大化させないという catalog
側のポリシー。method を切り替えるときは新しいアーティファクトに合う
`checksum_amd64` / `checksum_arm64` を `workspace.toml` 側で更新する
（`cocoon plugin pin --method <name>` で pin と method を同時に書き換え可）。
さもないと install スクリプトの `sha256sum -c -` が落ちる。

**3 層オーバーレイ。** 現状の method 解決は **同一 layer 内で完結** する
方式のみ対応 — `plugin.toml` と全 `install.<name>.sh` が同じ layer
（embedded / user / project）に揃っている必要がある。method スクリプト
だけ別 layer に置く運用は未対応。

## install スクリプトに渡される環境変数

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
| `PIN`            | per-RUN env、`[version].version_capable = true` のときのみ | `workspace.toml` の `[plugins.versions]` セクション内 `<id> = { pin = "..." }` エントリの値。空なら upstream 最新を使う |
| `CHECKSUM_AMD64` | per-RUN env、`version_capable = true` かつ `verify = "checksum"` のときのみ | amd64 アーティファクトの `sha256`。空ならスクリプトは検証スキップ＋警告。`verify = "pgp"` プラグインには渡されない |
| `CHECKSUM_ARM64` | 同上 | arm64 アーティファクトの `sha256` |
| `<BUILD_ARG>`    | per-RUN env (`ARG` 宣言も併発)、`[install].build_args` に列挙されたときのみ | ジェネレータは `ARG <name>` 行をプラグインごとに 1 回 (先に走る hook の直前) 出力し、両 hook の per-RUN prefix に `<name>="${<name>}"` を載せる。Dockerfile は各 prefix 行で値を置換する。catalog プラグインで現在宣言するものは無く、自作プラグイン向けの機構 |
| `<EXTRA_ENV>`    | per-RUN env、`[install.extra_versions]` で宣言されたときのみ | 宣言された subcomponent 版ごとに 1 つの env が出る。プラグイン側で env 名と default を宣言し、ユーザーは `workspace.toml` の `[plugins.versions].<id>` インラインテーブルに同じキー名を書いて値を上書きできる（例: `android-sdk = { pin = "...", api_level = "36" }`）。予約 env 名（上記）と衝突する env 名は宣言できない。詳細は後述の「サブコンポーネントバージョン」節を参照 |

ホスト側での評価は一切行われない — bash はビルド環境内で本体を実行し、
その環境変数は上記 2 ソースから組み立てられる。

リテラル文字列 `COCOON_PLUGIN_EOF` を `install.sh` / `install_user.sh`
内で **行頭から行末まで一致する形で書いてはならない**。書くと
heredoc が早閉じしてビルドが壊れる。ジェネレータはこれを検出して
loud に失敗する（`ErrHeredocCollision`）。

## バージョン対応プラグイン（`version_capable = true`）

versioned プラグインは次の契約を守る:

- `$PIN` を読みバージョンとして使う。空なら upstream 最新に
  fallback（curl リダイレクト・GitHub API など）。
- `dpkg --print-architecture` 等でアーキを判定し、対応するアーティファクト
  を選ぶ。
- ダウンロードを検証する。方式は `[version].verify` で選ぶ:
  - `verify = "checksum"`（既定）— アーキに対応する checksum 変数
    （amd64 なら `$CHECKSUM_AMD64`、arm64 なら `$CHECKSUM_ARM64`。
    アーキ非依存アセットはどちらを参照してもよい）が非空なら sha256
    検証し、空のときは **明示的に WARNING を出力** する（無言でスキップしない）。
  - `verify = "pgp"` — SHA256 を公開せず detached 署名のみ配布する
    upstream 向け（例: AWS CLI）。スクリプトは install スクリプトに
    同梱した署名鍵で in-script 検証する。`$CHECKSUM_AMD64` /
    `$CHECKSUM_ARM64` は渡されず、そのプラグインの `[plugins.versions]`
    に `checksum_amd64` / `checksum_arm64` を書くと `gen` 時に拒否される。

ユーザー側は `workspace.toml` の `[plugins.versions]` セクションに
1 エントリ 1 行の inline-table で pin を記録する。checksum は
`verify = "checksum"` プラグインにのみ付ける:

```toml
[plugins.versions]
go = { pin = "1.23.4", checksum_amd64 = "abc...", checksum_arm64 = "def..." }
aws-cli = { pin = "2.34.48" }   # verify = "pgp" — checksum フィールドは付けない
```

`cocoon plugin pin <id> <ref> --write` がこの行を upsert する。
フラグ詳細は `docs/commands.ja.md` 参照。

`version_capable = false` のプラグインは pin できない。`cocoon gen` は
そのプラグインの `[plugins.versions]` エントリを拒否し、`cocoon plugin pin`
もエントリを出力しない。

### サブコンポーネントバージョン（`[install.extra_versions]`）

主アーティファクト（`$PIN` で固定）の他に、別バージョンを後付けで取得
するプラグインもある。Android SDK の `cmdline-tools` (pin) +
`platforms;android-<level>` / `build-tools;<ver>`（別々に選びたい）が
典型例。

サブコンポーネントをユーザーが上書き可能な「つまみ」として公開するには
`[install.extra_versions]` で宣言する:

```toml
[install.extra_versions]
api_level   = { env = "ANDROID_SDK_API_LEVEL",   default = "35" }
build_tools = { env = "ANDROID_SDK_BUILD_TOOLS", default = "35.0.0" }
```

- **キー名**（`api_level`, `build_tools`）— ユーザーが
  `[plugins.versions].<id>` 内に書く名前。`^[a-z][a-z0-9_]*$` に一致。
  `pin` / `checksum_amd64` / `checksum_arm64` は `[plugins.versions]`
  が消費する予約キーなので、extra キーとして宣言すると拒否される
  （宣言しても上書き不可能な no-op になるため）。
- **`env`** — install スクリプトが読む環境変数名。`^[A-Z_][A-Z0-9_]*$`、
  予約 env 名（前述）や `[install].build_args` の名前、`extra_versions`
  内の他の `env` と衝突不可。
- **`default`** — 必須・非空。`workspace.toml` で未指定時に使われる値。
  install スクリプトは env がセットされている前提で書く
  （`: "${ANDROID_SDK_API_LEVEL:?...}"` などで fail-fast）。
  `default` も workspace 側の override も `"` / `\` / `\n` / `\r` /
  `$` / backtick を含む値は拒否される（値は Dockerfile の RUN
  プレフィックス `KEY="..."` に展開されるため、前 4 文字は shell
  quoting を破壊し、`$` / backtick は parameter / command substitution
  を引き起こしてリテラルなバージョン文字列として渡らなくなる）。

ユーザー側 workspace.toml では既存の `[plugins.versions]` と同じ書き方:

```toml
[plugins.versions]
android-sdk = { pin = "14742923", api_level = "36", build_tools = "36.0.0" }
```

`[install.extra_versions]` で宣言されていないキーが書かれていると
`cocoon gen` は `ErrUnknownExtraVersion` で拒否する（typo
（例: `api_levle = "..."`）が default に落ちないようにするため）。
`cocoon plugin pin <id> <ref> --write` は既存行の extra キーを保全し、
`pin` / `checksum_*` のみ書き換える。

## Catalog ツアー

自作プラグインの参考にできる embedded プラグイン:

- **`go`** — `archive` メソッド + `[install.env]` 多用。
  `$PIN` + `$CHECKSUM_*` + アーキ分岐の典型。
- **`docker-cli`** — `apt` メソッド。サードパーティ apt リポジトリ
  (keyring + `sources.list.d`) を追加してインストールする。apt リポジトリ系
  プラグインの参考に。既定では無効。利用時は明示的に有効化し、
  クライアントがホスト daemon に到達できるよう `[container].docker_socket = true`
  も設定すること。
- **`proto`** — `installer` メソッド。upstream installer に任せる
  最小構成だが `$PIN` で版指定はする。
- **`starship`** — `binary` メソッド、かつ catalog 内で唯一
  `install_user.sh` を持つ。root → user 境界の見本として両ファイルを併読する。
- **`lazygit`** — `binary` メソッド、`[install.env]` 無し。
  versioned プラグインの最小構成。
- **`copilot-cli`** — 2 メソッド (`installer` + `binary`) を持つ唯一の
  catalog プラグイン。`install.installer.sh` / `install.binary.sh` を
  併読すると、multi-method 専用の `$COCOON_INSTALL_METHOD` fail-fast
  パターンが見られる。
- **`android-sdk`** — `archive` メソッド。ZIP を取得した上で同じ RUN 内で
  追加のインストーラ (`sdkmanager`) を走らせる例。`[install.extra_versions]`
  の参考にもなる: `commandline-tools` の `pin` とは独立に platform /
  build-tools のバージョンをユーザーが指定できるよう `api_level` /
  `build_tools` を宣言している。

## トラブルシューティング

- **`cocoon plugin show` / `gen` で `unknown field "<x>"`** —
  `plugin.toml` に改名・削除されたフィールドが残っている。前述の
  「`plugin.toml` スキーマ」と照合する。
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
