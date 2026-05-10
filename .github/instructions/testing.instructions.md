---
applyTo: "**/*.go"
---

# テスト網羅性ルール

cocoon の Go テストは **契約 / 失敗分類 / 入力多様性 / メタデータ / CLI 配線** の 5 軸で網羅する。`just ci` グリーンでも軸が欠ければレビューで指摘される。

公開関数（新規 / 既存改修）ごとに 5 軸を「該当 / N/A」で判定してから実装する。N/A 理由を言語化できなければ漏れている可能性が高い。

## 1. Contract — docstring の断定文ごとに assert

doc コメントの claim（「verbatim 保持」「絶対パス返却」など）1 つにつき test を 1 つ。claim 単位でテスト名を切る。

例: `TestUpsertPinBlockPreservesNoTrailingNewline` / `TestUpsertPinBlockPreservesTrailingBlankLines`

## 2. Error class — `errors.Is` で分類を確認

`err != nil` だけでは不可。sentinel error は `errors.Is` で照合する。

```go
if !errors.Is(err, plugincli.ErrUsage) {
    t.Fatalf("err = %v, want ErrUsage", err)
}
```

`(value, nil)` / `("", ErrXxxNotFound)` / `("", wrapped err)` の 3 状態を返す関数は 3 ケース全部書く。システムエラーを usage error にすり替えると debugging context が消える。

## 3. Input variation — regex / parser は category 単位で table-driven

受理 / 拒否を形式ごとに列挙し `t.Run(tc.name, ...)` で sub-test 化する。

例: TOML 値検査では `inline-table`（`go = { pin = "..." }`）/ `bare-string`（`go = "..."`）/ `array`（`go = [..]`）を全形式試す。一形式だけだと近隣 category を見逃す。

## 4. Metadata — perm / 改行 / 境界 shape

ファイル I/O 関数では内容比較に加えて:

- `os.Stat().Mode().Perm()` が input と一致
- 末尾改行の有無が input と一致
- fixture shape を最低 3 種混ぜる（空ファイル / 1 行 / 末尾空行複数）

golden が全て同じ shape（末尾改行あり・middle に空行 1 個など）だと shape 依存バグが invisible になる。

## 5. CLI integration — cobra 配線を最低 1 ケース実走

新規 / 改修の flag・subcommand には `runCmd(t, ...)` 経由の test を最低 1 ケース。unit だけでは `cmd.Flags().Changed`、`SilenceUsage`、stdout/stderr 配線のバグを拾えない。

- `withIsolatedHome(t)` で `~/.cocoon/` を tempdir 隔離（`t.Setenv("HOME", ...)`）
- `t.Chdir(dir)` で `config.Discover` を制御（要 `//nolint:paralleltest`）
- 失敗ケースは `errors.Is` で class を assert + stderr / `err.Error()` を `strings.Contains` で確認

参考: `internal/cli/plugin/pin_test.go`（7 シナリオ）。

## 推奨パターン

- **Table-driven + sub-test**: `for _, tc := range cases { t.Run(tc.name, func(t *testing.T) { t.Parallel(); ... }) }`
- **Golden file**: `flag.Bool("update-golden", false, ...)` で再生成可能に。CI は未指定で diff。`internal/plugin/mutator_test.go` 参照
- **Sentinel error 公開**: `var ErrXxx = errors.New(...)` を export、test は `errors.Is` で照合（`errors.New` の inline 比較禁止）
- **`t.Parallel` 既定**: FS 独立 test は冒頭で呼ぶ。`t.Chdir` / `t.Setenv` 利用時は `//nolint:paralleltest // <理由>` で抑制
- **Shape 多様化**: golden 作成時は「末尾改行あり / なし」「空 / 1 行 / 複数行」「セクション直前がコメント / 空行 / なし」など最低 2 軸を変えて混ぜる

## アンチパターン

- happy path のみ書き失敗ケースを書かない
- `err != nil` で済ませ class を見ない
- 全 golden が同じ shape
- unit が通れば CLI も OK と判断する
- docstring の "verbatim" / "absolute" / "preserves" に直接 assert が無い
- regex を `\{` 等に限定して近隣 category を試さない

## PR 前セルフ監査

`gh pr create` 直前に、新規 / 改修した公開関数を 5 軸 × 関数数の表で埋める。半分以上 N/A なら再考する。偽の ✅ より N/A の方が後で剥がれない。

| 関数               | Contract | ErrClass | InputVar | Metadata | CLI |
|--------------------|----------|----------|----------|----------|-----|
| UpsertPinBlock     | ✅       | ✅       | ✅       | ✅       | ✅  |
| resolvePluginsDir  | N/A      | ✅       | N/A      | N/A      | ✅  |
| renderAndWrite     | N/A      | N/A      | N/A      | N/A      | ✅  |
