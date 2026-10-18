package main

const (
	errInvalidTimeout = "timeout: %w"

	errStepShape     = "ステップは `アクション名: 値` の1キーで書く"
	errUnknownAction = "%q は未知のアクション"
	errNoShorthand   = "%s: 省略形は使えない（%v をオブジェクトで指定する）"
	errUnknownArg    = "%s: 未知の引数 %q"
	errArgNotScalar  = "%s.%s: 文字列で指定する"
	errArgNotBool    = "%s.%s: true / false で指定する"
	errArgRequired   = "%s.%s: 必須"
	errAssertShape   = "assert: `種別: 引数` の1キーで書く"
	errUnknownAssert = "assert: %q は未知の種別"
	errAssertArg     = "assert.%s: 引数の形が違う"

	errBrowserUnsupported   = "browser: %q は未対応（対応: %s）"
	errConcurrency          = "concurrency: 1以上を指定する"
	errTimeoutPositive      = "timeout: 正の値を指定する"
	errScenariosRequired    = "scenarios: 1つ以上必要"
	errScenarioNameRequired = "%s.name: 必須"
	errStepsRequired        = "%s.steps: 1つ以上必要"
	errEmpty                = "%s: 空にできない"
	errInvalidDuration      = "duration: %q を時間として解釈できない（例: 1s, 500ms）"
	errInvalidVarName       = "name: %q は使えない（英数字と _ のみ）"
	errInvalidRegexp        = "正規表現が不正: %v"
	errSelectorChain        = "selector: %q の `>>` 連鎖に空の段がある"
	errUnsafeName           = "%q はパス区切り文字や .. を含められない"
	errPDFUnsupported       = "scenarios[%d].steps[%d] pdf (line %d): pdf は Chromium の headless でのみ使える（実効設定: browser=%s, headless=%t）"

	errEnvUndefined     = "${env.%s}: 環境変数が未定義"
	errStepVarUndefined = "${step.%s}: それ以前の extract / evaluate で定義されていない"
	errUnknownVar       = "${%s.%s}: 未知の変数"

	errFilterNoMatch  = "--filter %q に一致するシナリオがない"
	errDriverMissing  = "Playwright ドライバ／ブラウザが使えない（Playwright 公式の手順で事前に導入する）: %v"
	errBrowserLaunch  = "%s を起動できない: %s"
	errBrowserCrashed = "実行中にブラウザとの接続が切れた"
	errInterrupted    = "中断された"

	errSelectorNoMatch   = "セレクタ %q に一致する要素がない"
	errSelectorNoMatchAt = "セレクタ %q に一致する要素がない（%q の段で0件）"
	errSelectorMulti     = "セレクタ %q が %d 件に一致した（1件に絞る必要がある）"

	errAssertURLContains  = "URL %q が %q を含まない"
	errAssertURLMatches   = "URL %q が %q にマッチしない"
	errAssertState        = "%q が %s にならなかった"
	errAssertTextContains = "テキスト %q が %q を含まない"
	errAssertEquals       = "%q と %q が一致しない"
	errAssertNotEquals    = "両辺が %q で一致した"

	errOutputDirMissing   = "%s が存在しない"
	errIndexOutOfRange    = "シナリオ番号 %d は範囲外（1〜%d）"
	errScenarioIDNotFound = "シナリオID %s が %s 配下の実行記録に見つからない"
	errScenarioIDStale    = "シナリオID %s の記録（番号 %d・名前 %q）が今のYAMLと一致しない（番号で指定し直す）"

	errFileExists = "%s は既に存在する"
)
