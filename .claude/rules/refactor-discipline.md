---
paths:
  - "**/*"
---

# リファクタリング・規約変更ディシプリン

挙動・契約を変更したとき「実装は変えたが説明だけ古いまま」になるドリフトを潰す。レビューで指摘されるパターンの過半数がこれ。

## 1. 旧挙動を表す語の追跡

実装で X を変えたら、旧挙動を表す動詞・形容詞を `rg` で全リポジトリ検索する:

- `always` / `常時` / `unconditional` / `before any` / `regardless` 等
- 削除・改名した識別子そのもの (`rg <oldName>`)

ヒット先は次の全カテゴリで更新:

- 関数 docstring・関数本体内の inline コメント
- 同パッケージ・他パッケージの呼び出し側コード
- テスト関数宣言コメント・テスト本体の inline コメント
- `docs/` 配下のドキュメント・anchor リンク
- `CHANGELOG.md` / `docs/CHANGELOG.ja.md`
- PR description (`gh pr edit --body-file`)

## 2. 削除した識別子の audit

関数のパラメータ・戻り値・フィールド・テンプレート変数を消したら、`rg` で他参照が dead 化していないか確認する:

- Go template の `{{ with .X }}` も忘れずに
- struct field 削除時の i18n key・config schema・golden fixture
- 戻り値削減時の caller 側 `_, _ = f()` 等

## 3. Edit ツールでの境界破壊を防ぐ

markdown / 設定ファイルにサブセクションを挿入する Edit 操作で、`old_string` に直前の見出しや区切り (`---`) を含めずに insert すると、隣接する後続見出しが脱落しやすい。

- 挿入境界の **両側** を `old_string` に含めて、`new_string` でも明示的に維持する
- 編集後にファイルを通読し、見出し階層・anchor が壊れていないか確認

## 4. 内部矛盾の事前チェック

「無効時は X が出ない」と「X は常時設定される」のような相反する文を 1 ファイル内に並べていないか、commit 前に該当ファイルを通読する。`rg` だけでは検出できない、書き換えた段落と離れた段落の意味衝突は読み直しでしか拾えない。

## 5. i18n キーは catalog 定義必須

`cat.Msg("...")` で呼ぶキーが catalog に無いと、`Msg` の fallback でキー名がそのまま UI に出る。commit 前に「呼出キーの集合」と「catalog 定義キーの集合」を差分チェックし、未定義キーをゼロにする。

## 6. ユーザー向け snippet で guessed default 禁止

migration error / placeholder / コピー前提の案内テキストには、**実値か明示プレースホルダ**のみ書く。「もっともらしい default」を埋めると、片方しか設定していないユーザーに不整合な組合せを copy & paste させてしまう。

## 7. 設計変更時は commit 前 rg sweep

設計を変えたら commit 前に旧用語を `rg` 一巡。`just ci` グリーンは弱い信号、旧用語ゼロが強い信号。

対象: docstring / inline コメント / error message / i18n string / test 関数名・失敗メッセージ / docs / CHANGELOG / fixture / workflow yml。
