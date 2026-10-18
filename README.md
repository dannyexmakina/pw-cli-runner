# pwyr — Playwright YAML Runner

YAMLで宣言したブラウザ操作シナリオを Playwright で実行する CLI。シナリオ単位で並行実行し、スクリーンショット・PDF・実行結果（summary.json）を保存する。

Playwright で最も詰まりやすい「セレクタが存在するか・意図した要素を指しているか」を、`dryrun` のセレクタ診断で明示的に返す。

## 前提

- Playwright ドライバとブラウザが導入済みであること（playwright-go v0.6201.1 / Playwright 1.62.1 に対応する版）。pwyr 自身はダウンロードを行わない。

## 使い方

```sh
pwyr init                      # pwyr.yml と .gitignore を書き出す
pwyr check   [file]            # 静的検証（ブラウザは起動しない）
pwyr dryrun  [file]            # 全操作を実行、ファイルは書かない、セレクタ診断付き
pwyr run     [file]            # 本番実行
pwyr single  <シナリオID|番号> [file]   # 1シナリオだけ実行（--dryrun 可）
pwyr list    [file]            # 過去の実行のシナリオID一覧
```

`file` を省略すると `pwyr.yml` を読む。

`run` / `dryrun` は `--concurrency` `--headed` `--browser` `--output-dir` `--filter <regex>` で YAML の設定を上書きできる。

> `dryrun` は「ローカルにファイルを書かない」だけで、対象サイトへの操作（フォーム送信など）は実際に行われる。

## YAML

```yaml
name: 例
browser: chromium        # chromium | firefox | webkit
headless: true
timeout: 30s             # ステップ単位のタイムアウト
output_dir: ./runs
concurrency: 2
scenarios:
  - name: ログインできる
    group: "01.認証"     # 省略可。画像の保存先サブフォルダ
    steps:
      - goto: https://example.com/login
      - fill:
          selector: "#user"
          value: ${env.LOGIN_USER}
      - click: "role=button[name='ログイン']"
      - waitFor: "text=ダッシュボード"
      - screenshot: after-login.png
      - assert:
          url_contains: /dashboard
```

全アクションの例は [templates/01.yml](templates/01.yml)。

### アクション

| アクション | 引数 | 省略形 |
|---|---|---|
| `goto` | `url` | ○ |
| `click` / `hover` / `check` / `uncheck` / `waitFor` | `selector` | ○ |
| `fill` / `type` / `select` | `selector`, `value` | — |
| `press` | `selector`, `key` | — |
| `wait` | `duration` | ○ |
| `screenshot` | `path`, `full_page` | ○ |
| `pdf` | `path`（Chromium headless のみ） | ○ |
| `evaluate` | `name`, `script` | — |
| `extract` | `name`, `selector`, `attr`（`textContent` または属性名） | — |
| `assert` | 下表 | — |

| assert | 引数 |
|---|---|
| `url_contains` / `url_matches` | 文字列 / 正規表現 |
| `visible` / `hidden` | セレクタ |
| `text_contains` | `selector`, `value` |
| `equals` / `not_equals` | `left`, `right` |

### 変数

`${env.X}` `${scenario.name}` `${scenario.id}` `${scenario.group}` `${step.<name>}`（`extract` / `evaluate` の結果） `${run.id}`

## セレクタ診断

`dryrun` では、セレクタを取る全ステップで操作前に次を調べる（`run` では失敗時のみ）。

- `count`：一致した要素数（0件・複数件はその場で失敗）
- `visible` / `enabled` / `editable`
- `matched`：一致した要素のタグ・role・アクセシブルネーム・テキスト・主要属性・祖先パス
- `resolved`：`>>` 連鎖のどの段で0件になったか

```
[c41e9d07:ログインできる] click .box >> nth=0 >> button[type=submit] count=1 ✓ → button "ログイン"
[c41e9d07:ログインできる] click .menu a count=2 ✗
```

意図した要素かどうかは `matched` を見て人が判断する。

## 出力

```
runs/<実行ID>/
  summary.json
  scenarios/<シナリオID>/[<group>/]after-login.png
```

- 実行ID・シナリオIDは UUIDv7。
- summary.json と同じ内容を stdout に出す。進捗は stderr。
- 記録される `args` は変数展開前の値（秘匿値を残さないため）。

## 終了コード

| コード | 意味 |
|---|---|
| 0 | 成功 |
| 1 | シナリオ失敗あり |
| 2 | 使い方の誤り |
| 3 | YAML不正 |
| 4 | 実行環境エラー（ドライバ／ブラウザ未導入・起動失敗など） |
| 5 | 内部エラー |

## ビルド

```sh
go build -o pwyr .
```

Go 1.22 以上。
