---
applyTo: "**/*.go"
---

# テスト網羅性ルール

cocoon の Go テストは「動くか」より**契約遵守 / 失敗分類 / 入力多様性 / メタデータ / CLI 配線**の 5 軸で網羅する。`just ci` がグリーンでも、5 軸のどれかが薄ければ Copilot レビューで露出する (PR #9 で 10 件指摘の根因はこの偏り)。

CLAUDE.md の「コミット前に必ずローカルでテストとリントを通す」を満たす前提で、本ルールは**何を**テストするかを規定する。

---

## 着手前チェックリスト

公開関数 (新規 / 既存改修) ごとに以下 5 軸すべてを「該当 / N/A」で明示してから実装する。テスト省略は OK だが「N/A」と判断した理由を頭の中で言語化できなければ漏れている可能性が高い。

### 1. Contract test — docstring が約束した挙動を 1 行ずつ assert

- **失敗モード**: docstring と実装が乖離する。レビュアーは docstring を読むので「verbatim 保持」「絶対パス返却」のような claim は必ず突かれる。
- **ルール**: 公開関数の doc コメントに書かれている断定文 1 つにつき、最低 1 つの test を対応させる。
- **悪い例**: `func UpsertPinBlock(...) error // Comments outside the target block are preserved verbatim.` → 末尾改行 / 末尾空行を保持するか確認する test が無い。
- **良い例**: `TestUpsertPinBlockPreservesNoTrailingNewline` / `TestUpsertPinBlockPreservesTrailingBlankLines` のように claim ごとにテスト名を切る。

### 2. Error class test — `ErrUsage` / `ErrFailure` の分類を `errors.Is` で確認

- **失敗モード**: システムエラー (Getwd 失敗、stat 失敗) を usage error にすり替えると、real failure の context が消えて debugging を阻害する。
- **ルール**: `err != nil` だけで満足しない。少なくとも `errors.Is(err, plugincli.ErrUsage)` 等で class を assert する。
- **悪い例**: `if err == nil { t.Fatal("want error") }`
- **良い例**: `if !errors.Is(err, plugincli.ErrUsage) { t.Fatalf("err = %v, want ErrUsage", err) }`
- **3 状態の関数**: `(value, nil)` / `("", ErrXxxNotFound)` / `("", wrapped err)` のような sentinel + system error 設計には、3 ケース全て test を書く。

### 3. Input variation test — regex / parser は入力カテゴリを列挙して table-driven

- **失敗モード**: regex を一形式しか試さず、似たカテゴリで通用する別形式を見逃す。
- **ルール**: 受理 / 拒否対象を category 単位で列挙し `t.Run(tc.name, ...)` で sub-test 化する。
- **悪い例**: `inline-table` 形式 (`go = { pin = "..." }`) だけ test → bare-string (`go = "..."`)、array (`go = [..]`) を見逃す。
- **良い例**: PR #9 の `TestUpsertPinBlockRejectsVersionsKeyAssign` (3 形式 table-driven)。

### 4. Metadata test — perm / 改行 / 文字エンコーディング / 境界サイズ

- **失敗モード**: 内容しか見ず、ファイル mode が 0o600 → 0o644 に勝手に緩む / 末尾改行が消える / multi-blank tail が 1 個に正規化される。
- **ルール**: ファイル I/O が絡む関数では「内容 == 期待値」だけでなく以下を確認:
  - `os.Stat().Mode().Perm()` が input と一致するか
  - 末尾改行の有無が input と一致するか
  - 空ファイル / 1 行ファイル / 末尾空行が複数あるファイルなど **shape を変えた fixture** を最低 3 種用意
- **golden file の落とし穴**: 全 golden が「末尾改行あり、middle に空行 1 個」のような同じ shape だと、shape 依存のバグが invisible になる。

### 5. CLI integration test — cobra 配線を最低 1 ケース実走

- **失敗モード**: 内部関数の unit test だけで満足し、`cmd.Flags().Changed`、引数解析、stdout / stderr 出力、`SilenceUsage` の挙動など CLI 配線レベルのバグを見逃す。
- **ルール**: 新規 / 改修 flag や subcommand には、`runCmd(t, ...)` 経由で CLI を実走する test を最低 1 ケース。
- **テクニック**:
  - `withIsolatedHome(t)` で `~/.cocoon/` を tempdir に隔離 (`t.Setenv("HOME", ...)`)
  - `t.Chdir(dir)` で `config.Discover` を制御 (要 `//nolint:paralleltest`)
  - 失敗ケースは `errors.Is(err, plugincli.ErrUsage)` で class を assert + stderr / err.Error() の文言断片を `strings.Contains` で確認
- **良い例**: `internal/cli/plugin/pin_test.go` の 7 シナリオ (default / write happy / append-after / replace / refusal / no-workspace / inline-form)。

---

## 推奨パターン

- **Table-driven + sub-test**: `cases := []struct{...}` + `for _, tc := range cases { t.Run(tc.name, func(t *testing.T) { t.Parallel(); ... }) }`
- **Golden file + `-update-golden`**: `flag.Bool("update-golden", false, ...)`、CI では未指定で diff、ローカルで再生成。`internal/plugin/mutator_test.go` パターン参照。
- **Sentinel error**: `var ErrXxx = errors.New(...)` を package で公開し、test は `errors.Is(err, ErrXxx)` で照合 (`errors.New` の inline 比較禁止)。
- **`t.Parallel` 既定**: ファイルシステム独立な test は `t.Parallel()` を冒頭で呼ぶ。`t.Chdir` / `t.Setenv` 利用時は `//nolint:paralleltest // <理由>` で抑制。
- **境界 fixture の shape 多様化**: golden を作るときは「末尾改行あり / なし」「空ファイル / 1 行 / 複数行」「セクション直前にコメント / 空行 / 何もなし」など最低 2 軸を変えたケースを混ぜる。

---

## アンチパターン (= レビューで突かれる)

- ❌ happy path しか書かない (失敗ケースが N/A になっていないか確認)
- ❌ `if err != nil` で error 検出を済ませ、class を見ない
- ❌ 全 golden が同じ shape (末尾改行・空行・行数が均質)
- ❌ 「unit で covered だから CLI も大丈夫」(配線バグを見逃す)
- ❌ docstring の "verbatim" "absolute" "preserves" の語に対する直接 assert が無い
- ❌ regex を `\{` で限定する等狭く書いて、近隣の category に通用するか考えない

---

## PR 投稿前のセルフ監査

`gh pr create` 直前に、新規 / 改修した公開関数を列挙し、5 軸 × 関数数の表で「test の有無」を埋める。半分以上 N/A なら、本当に N/A か再考する。

```
| 関数                | Contract | ErrClass | InputVar | Metadata | CLI |
|--------------------|----------|----------|----------|----------|-----|
| UpsertPinBlock     | ✅       | ✅       | ✅       | ✅       | ✅  |
| resolvePluginsDir  | N/A      | ✅       | N/A      | N/A      | ✅  |
| renderAndWrite     | N/A      | N/A      | N/A      | N/A      | ✅  |
```

監査表に偽の ✅ を埋めるくらいなら N/A で出した方が後で剥がれない。レビューで突かれる前に自覚的に N/A にしておけば、コメント返信で「意図的に scope 外」と説明できる。
