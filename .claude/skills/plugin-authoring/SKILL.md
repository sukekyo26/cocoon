---
name: plugin-authoring
description: 'workspace-docker プラグインの新規作成・改修ガイド。plugins/<id>/{plugin.toml, install.sh} の二段構成、$PIN/$CHECKSUM_* 入力規約、テスト・CI 登録手順を扱う。Triggers: "create plugin", "new plugin", "add plugin", "plugin.toml", "install.sh", "version_capable", "プラグイン作成", "プラグイン追加", "新しいプラグイン".'
---

# plugin-authoring

workspace-docker のプラグインを新規追加または改修する際の手順とルールを定義する。
詳細仕様は `docs/plugins.md` / `docs/plugins.ja.md` を参照。本スキルはエージェント向けの作業手順に絞る。

## まずは scaffold から

新規追加なら `wsd plugin scaffold <id>` で雛形を作るのが最短。これにより本ドキュメント記載の
ディレクトリ構造・必須プロローグ・`$PIN`/`$CHECKSUM_*` 規約が自動で反映され、生成直後に
strict TOML 検証も走る。

```bash
# 対話モード (推奨)
wsd plugin scaffold my-tool

# 非対話モード (CI / 自動化)
wsd plugin scaffold my-tool \
  --template tarball --version-capable --requires-root \
  --name "My Tool" \
  --description "Short description (https://github.com/owner/repo)" \
  --non-interactive
```

`--template` の選び方:

- `curl-pipe` — 上流が `curl ... | bash` 形式の公式インストーラを提供（uv, proto, copilot-cli 風）
- `tarball` — GitHub Release tarball + sha256 検証（starship, go, lazygit 風）。`--version-capable` 必須
- `generic` — apt / .deb / 自作スクリプト用の最小骨組み（プロローグと TODO のみ）

scaffold 後に `install.sh` の上流 URL や固有ロジック、必要なら `[apt]` / `[install.env]` / `[install.build_args]` を手で追記する。下記は scaffold が生成しない手書き要素の参考も含めた完全仕様。

## ディレクトリ構造（必須）

```text
plugins/<id>/
├── plugin.toml       # メタデータ・apt 依存・env / build_args・version 契約
├── install.sh        # root で実行されるシェルスクリプト
└── install_user.sh   # 任意: ${USERNAME} で実行されるポストインストール
```

- `<id>` は kebab-case（例: `aws-sam-cli`, `google-chrome`）
- `<id>` がそのまま `workspace.toml` の `[plugins].enable` で使う名前になる

## plugin.toml の書き方

```toml
[metadata]
name = "Display Name"
description = "短い説明 (https://github.com/owner/repo)"   # URL 必須
default = false
# conflicts = ["other-plugin-id"]   # 任意

[apt]
packages = ["dep1", "dep2"]   # 任意

[install]
requires_root = true            # install.sh を root で実行するか
volumes = ["/home/${USERNAME}/.tool"]   # 任意。自動的に mkdir + chown される
# build_args = ["MY_ARG"]   # ARG として宣言、env で install.sh に渡す

# RUN 後に出力する ENV
[install.env]
MY_TOOL_HOME = "/home/${USERNAME}/.tool"
PATH         = "$MY_TOOL_HOME/bin:$PATH"

[version]
version_capable = false
```

### 必須ルール
- `description` には上流リンクを `(URL)` 形式で含める
- `requires_root = true` のとき install.sh で手動 `USER` 切替は書かない（ジェネレータが包む）
- `build_args` の名前は `^[A-Z_][A-Z0-9_]*$`

## install.sh の書き方

### 共通プロローグ（必須）

```bash
#!/usr/bin/env bash
set -euo pipefail
```

### ジェネレータが渡す入力 env

| 変数 | 設定タイミング | 用途 |
|:-----|:---|:---|
| `USERNAME` | 常に | コンテナのユーザー名 |
| `PIN` | `version_capable=true` のみ | バージョンピン（空なら latest） |
| `CHECKSUM_AMD64` | `version_capable=true` のみ | sha256（空なら検証スキップ） |
| `CHECKSUM_ARM64` | `version_capable=true` のみ | sha256（空なら検証スキップ） |
| `<build_args>` | `[install].build_args` に列挙したもの | 任意 |

### `version_capable=true` の典型パターン

```bash
ARCH="$(dpkg --print-architecture)"
case "$ARCH" in
  amd64) CHECKSUM="$CHECKSUM_AMD64" ;;
  arm64) CHECKSUM="$CHECKSUM_ARM64" ;;
esac

if [ -n "$PIN" ]; then
  VERSION="$PIN"
else
  VERSION=$(curl -fsSL https://example.com/latest | jq -r .version)
fi

curl -fsSL "https://example.com/v${VERSION}-${ARCH}.tar.gz" -o /tmp/x.tar.gz
if [ -n "$CHECKSUM" ]; then
  echo "${CHECKSUM}  /tmp/x.tar.gz" | sha256sum -c -
fi
tar -C /usr/local -xzf /tmp/x.tar.gz
rm /tmp/x.tar.gz
```

### root プラグインのお決まりクリーンアップ

apt を使った場合は最後に必ずクリーンアップする（イメージ肥大化防止）:

```bash
apt-get clean
rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*
```

### `curl | sh` パターンの扱い

- `version_capable = "pinned"` でバージョン固定可能なら、可能なら SHA256 ピンを推奨
- 上流が随時更新する公式インストーラ（uv, proto, copilot-cli 等）は SHA256 固定しない（HTTPS+TLS で十分）

## 既存プラグインを参考にする

| 例 | 学べること |
|:---|:---|
| `plugins/go/` | `version_capable`, ARCH 切替, 公式 tar の取得, `[install.env]` |
| `plugins/docker-cli/` | `build_args = ["DOCKER_GID"]`, GID マッピング |
| `plugins/proto/` | `[install.env]` で `PROTO_HOME` / `PATH` 出力 |
| `plugins/starship/` | `install_user.sh` で `~/.bashrc` を編集 |
| `plugins/zig/` | `jq` 依存, 上流 index.json 経由でアセット名を解決 |
| `plugins/lazygit/` | GitHub API を使わない latest 取得 (`--location-trusted` redirect) |

## 必須テスト

`tests/unit/plugins/test_<id>.sh` を `test_go.sh` をテンプレに作成する。

```bash
#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/plugin_test_helper.sh"

PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
init_test "test_<id>"
load_plugins_helpers

test_<id>_install() {
  section "<id> install snippet"
  local out
  out=$(generate_plugin_installs <id>)
  assert_contains "binds plugin dir" "$out" "source=plugins/<id>"
  assert_contains "runs install.sh"  "$out" "bash /tmp/plugin/install.sh"
  # その他、install.sh が含むべき重要な処理
}

test_<id>_install
print_results
```

ヘルパー `generate_plugin_installs` は生成 RUN 行に加えて `install.sh` の内容も連結して返すので、`assert_contains` で install.sh の処理を直接検査できる。

## CI への追加

`.github/workflows/ci.yml` の `docker-build` ジョブ内のプラグインリストに `<id>` を追加する。
追加しないと CI でビルドされないため、品質が保証されない。

## CHANGELOG への記載

`changelog` スキルの手順に従い、`CHANGELOG.md` と `docs/CHANGELOG.ja.md` の両方の `[Unreleased]` に追記する。
プラグイン追加は `### Added`、既存改変は `### Changed`、削除は `### Removed`。

## 検証コマンド

```bash
# Lint
shellcheck plugins/<id>/install.sh

# プラグイン単体
bash tests/unit/plugins/test_<id>.sh

# 全体
bash tests/run_all.sh   # Suite Summary が "All test suites passed!" であること
```

`tests/structure/test_snapshot.sh` のスナップショットが差分になる場合は、内容を確認したうえで
`bash tests/structure/test_snapshot.sh --update` で更新する。
