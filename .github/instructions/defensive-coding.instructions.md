---
applyTo: "**/*.go"
---

# 防御的コーディング・ルール

PR レビューで繰り返し指摘される 7 パターン (silent failure / 入口の nil panic / unexported sentinel / package shadowing / 衝突未検出 / doc-code 不整合 / 生成シェル) を実装前に潰す。`just ci` グリーンでも実装の堅牢性が欠ければレビューで指摘される。

公開関数（新規 / 既存改修）ごとに該当軸を「該当 / N/A」で判定してから書く。N/A 理由を言語化できなければ漏れている可能性が高い。

## 1. Silent failure を作らない

- ヘルパが `false` / `""` / `nil` を返す条件と「エラー」条件を分離する。`os.Stat` 流に倣い、`errors.Is(err, fs.ErrNotExist)` 等のセンチネルだけを「ない」に潰す。それ以外は `(zero, err)` で伝播。
- 「emit する / しない」を返す bool は、判定と実 emit を同じ条件で揃える（「emit=true 返したのに何も出ない」契約違反を作らない）。
- 入力 / read / stat エラーを `false` で握り潰さない。Dockerfile 等の生成物が「半端」になるのが silent failure の最悪形。

## 2. fail-fast を入口で

- 関数の前提（`fsys != nil`、`ctx != nil` 等）は **入り口で 1 回** 確認する。下流のヘルパで重複しない。
- 「依存が空 (enable list 0 件) なら no-op で正常通過、空でないのに依存リソースが nil なら sentinel error」を入口ロジックに。
- 早期 return は `errors.Is` で識別できる exported sentinel (3 軸を参照) を返す。

## 3. Sentinel error は exported

- `var ErrFoo = errors.New(...)` で **必ず exported**。`errFoo` (unexported) は外部 package から `errors.Is` で識別できない。
- doc コメントが「callers can identify via errors.Is」と書くなら exported 必須。コメントとアクセス修飾子の不一致は DOC バグ。
- 構築は `fmt.Errorf("ctx: %w", ErrFoo)` で wrap する。直接 `fmt.Errorf("...")` は err113 lint で蹴られる。
- 同じ意味の sentinel を複数 package で定義しない。下位 package のものを再利用する。
- **wrap は呼び出し連鎖で 1 回だけ**。caller が `fmt.Errorf("%w: %w", ErrFailure, err)` で wrap する設計なら、helper は `ErrFailure` で wrap せず生のエラー (or package-private sentinel) を返す。両方 wrap すると `gen failed: gen failed: …` の二重 prefix になる。

## 4. import package と shadow しない

- 標準 package (`path`, `time`, `os`, `fs`, `errors`, `bytes`, `strings`, `context`, `url` 等) を import している場合、同名の param / 局所変数を作らない。
- `func Load(path string)` のように衝突する場合は `tomlPath` / `filePath` 等に rename。`now`、`cause`、`reader` 等も同様。
- どうしても変えにくい場合は import alias (`stdpath "path"`) を検討。

## 5. User input は untrusted

- 第三者 plugin (`<project>/.cocoon/plugins/`, `~/.cocoon/plugins/`) の install.sh / plugin.toml / shell rc / 環境変数値は完全に user input。
- heredoc terminator / sentinel-line / トークン等の固定 delimiter は **入力 scan で衝突を検出して fail-fast**。動的に unique 化するより検出のほうが reproducibility が高い。
- ファイル名 / プラグイン id / TOML key を path に組み込む場合は `..`、`/`、`:`、改行を reject。

## 6. doc コメントは契約として強制力を持つ

- exported / unexported と doc の "callers can ..." 記述を一致。乖離は即 DOC バグ。
- 「emit=true を返したら何かを emit する」「verbatim 保持」等の claim は test で pin する (testing.instructions.md §1)。
- claim と実装が乖離した時は **どちらかを直す**。「コメントだけ残して実装は変えない」は禁止。

## 7. 生成シェル / Compose / Dockerfile の規約

Go テンプレートに埋め込むシェル断片や Compose interpolation には次を一律に適用:

- **apt-related RUN は cache mount を伴う**: `--mount=type=cache,target=/var/cache/apt,sharing=locked` + `/var/lib/apt`。既存 block と新規 block で齟齬を作らない (cache が無いと毎ビルドで apt-get update が走り、`/var/lib/apt/lists/*` がイメージ層に焼き付く)。
- **必須 env var は `${VAR:?msg}` 形式**: `${HOME}` / `${UID}` 等が unset で empty に展開され `/.cocoon/certs` のような壊れたパスを silently 作るのを防ぐ。Compose interpolation も POSIX shell も同じ構文を解釈。
- **shell パイプラインで exit status を握り潰さない**: `find … 2>/dev/null | head -n 1` は head の exit が支配する。`find … -print -quit` 等で一次コマンドの exit status を直接取れる形に書く。`2>/dev/null` の付与は本物のエラー表示も殺すので最小限に。

## 実装前セルフ監査

新規 / 改修関数ごとに次表を埋める。半分以上 N/A なら再考。

| 関数の類型 | Silent | FailFast | Sentinel | Shadow | Untrusted | Doc | GenShell |
|------|--------|----------|----------|--------|-----------|-----|----------|
| 公開生成器 (外部入力を扱う) | ✅ | ✅ | ✅ | N/A | ✅ | ✅ | ✅ |
| 内部読み込みヘルパ | ✅ | N/A | N/A | N/A | N/A | ✅ | N/A |
| 整形ヘルパ (pure func) | N/A | N/A | N/A | N/A | N/A | N/A | N/A |

偽の ✅ より N/A の方が後で剥がれない。N/A は理由を頭の中で言語化できる場合のみ。
