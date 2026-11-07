# XPath Copier (同梱Chrome拡張)

右クリックした要素の絶対XPathをクリップボードにコピーするだけの最小拡張。
pw-cli-runnerの宣言ファイルに貼り付けるセレクタを取得するためのペアツール。

## インストール

1. Chromeで `chrome://extensions/` を開く
2. 右上の「デベロッパーモード」をオン
3. 「パッケージ化されていない拡張機能を読み込む」でこの `chrome-extension/` フォルダを選択

## 使い方

ページ上の任意の要素を右クリック → 「XPathをコピー」

クリップボードに `/html/body/div[1]/.../button[3]` 形式の絶対XPathが入る。

## 宣言ファイルへの貼り付け

pw-cli-runner側は追加の変更なしで絶対XPathをそのまま受け入れる。
`xpath=` プレフィックスを付けた形で、セレクタフィールドに貼る。

```yaml
- action: click
  args:
    selector: "xpath=/html/body/div[1]/main/button[3]"
```

短縮形:

```yaml
- click: "xpath=/html/body/div[1]/main/button[3]"
```

### YAMLでの注意点

**必ずダブルクオートで囲む。**

絶対XPathは `[1]` `[3]` のような角括弧を含む。YAMLはクオート無しだと
角括弧をフロースタイル(配列リテラル)として解釈しようとして構文エラーになる、
もしくは意図せぬ値になる。

```yaml
# NG: 角括弧がYAMLに食われる
selector: xpath=/html/body/div[1]/button[3]

# OK: ダブルクオートで保護
selector: "xpath=/html/body/div[1]/button[3]"
```

シングルクオートでもエスケープ上は問題ないが、他のセレクタ記法で
将来エスケープが必要になった時のことを考えるとダブルクオート統一が無難。

## 設計方針

- 判断しない。指差しを翻訳するだけ
- 相対化・意味付け・role解析はしない
- 出力は生のXPath文字列のみ
- iframe越えは右クリックしたフレーム内のXPathだけ出す
- Shadow DOMは未対応

機械から機械への受け渡しなので、人間向けの可読性は捨てて正確性に振る。

## 既知の制約

- iframeをまたいだ合成XPathは出ない
- Shadow DOM内の要素は正しく取れない可能性がある
- SVG内の要素で `tagName` の大文字小文字が問題になる可能性がある
