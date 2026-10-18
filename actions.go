package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mxschmitt/playwright-go"
)

// stepError は summary の error.kind を伴う失敗。
type stepError struct {
	kind string
	msg  string
}

func (e *stepError) Error() string { return e.msg }

func failf(kind, format string, args ...any) error {
	return &stepError{kind: kind, msg: fmt.Sprintf(format, args...)}
}

// classify は Playwright 由来のエラーを種別に振り分ける。
func classify(err error, fallback string) *stepError {
	var se *stepError
	if errors.As(err, &se) {
		return se
	}
	if errors.Is(err, playwright.ErrTimeout) {
		return &stepError{kind: kindTimeout, msg: firstLine(err.Error())}
	}
	return &stepError{kind: fallback, msg: firstLine(err.Error())}
}

func firstLine(s string) string {
	s, _, _ = strings.Cut(strings.TrimSpace(s), "\n")
	return s
}

// scenarioRun は1シナリオ分の実行状態。
type scenarioRun struct {
	ctx     context.Context
	page    playwright.Page
	vars    *vars
	dir     string // 画像の保存先（run ディレクトリからの相対）
	runDir  string
	write   bool // false なら dryrun
	timeout time.Duration
}

// exec は1ステップを実行し、summary の output を返す。
func (r *scenarioRun) exec(st Step, args map[string]string) (any, error) {
	page := r.page
	loc := func() playwright.Locator { return page.Locator(args["selector"]) }

	switch st.Action {
	case "goto":
		if _, err := page.Goto(args["url"]); err != nil {
			return nil, classify(err, kindNavigation)
		}
	case "click":
		return nil, wrap(loc().Click())
	case "fill":
		return nil, wrap(loc().Fill(args["value"]))
	case "type":
		return nil, wrap(loc().PressSequentially(args["value"]))
	case "press":
		return nil, wrap(loc().Press(args["key"]))
	case "hover":
		return nil, wrap(loc().Hover())
	case "check":
		return nil, wrap(loc().Check())
	case "uncheck":
		return nil, wrap(loc().Uncheck())
	case "select":
		return r.selectOption(loc(), args["value"])
	case "wait":
		d, err := time.ParseDuration(args["duration"])
		if err != nil {
			return nil, failf(kindInternal, errInvalidDuration, args["duration"])
		}
		select {
		case <-time.After(d):
		case <-r.ctx.Done():
			return nil, failf(kindInternal, errInterrupted)
		}
	case "waitFor":
		return nil, wrap(loc().WaitFor())
	case "screenshot":
		return r.save(args["path"], func(path *string) error {
			_, err := page.Screenshot(playwright.PageScreenshotOptions{Path: path, FullPage: playwright.Bool(st.FullPage)})
			return err
		})
	case "pdf":
		return r.save(args["path"], func(path *string) error {
			_, err := page.PDF(playwright.PagePdfOptions{Path: path})
			return err
		})
	case "evaluate":
		v, err := page.Evaluate(args["script"])
		if err != nil {
			return nil, classify(err, kindInternal)
		}
		s, err := stringify(v)
		if err != nil {
			return nil, failf(kindInternal, "%v", err)
		}
		r.vars.step[args["name"]] = s
		return s, nil
	case "extract":
		var v string
		var err error
		if args["attr"] == "textContent" {
			v, err = loc().TextContent()
		} else {
			v, err = loc().GetAttribute(args["attr"])
		}
		if err != nil {
			return nil, classify(err, kindSelector)
		}
		r.vars.step[args["name"]] = v
		return v, nil
	case "assert":
		return nil, r.assert(st.Assert, args)
	default:
		return nil, failf(kindInternal, errUnknownAction, st.Action)
	}
	return nil, nil
}

func wrap(err error) error {
	if err == nil {
		return nil
	}
	return classify(err, kindSelector)
}

func (r *scenarioRun) selectOption(loc playwright.Locator, value string) (any, error) {
	if _, err := loc.SelectOption(playwright.SelectOptionValues{ValuesOrLabels: &[]string{value}}); err != nil {
		return nil, classify(err, kindSelector)
	}
	v, err := loc.Evaluate(`el => { const o = el.selectedOptions[0]; return o ? { value: o.value, label: o.label } : null }`, nil)
	if err != nil {
		return nil, classify(err, kindSelector)
	}
	return v, nil
}

// save は dryrun ではファイルを書かずに撮影だけ行う。
func (r *scenarioRun) save(name string, shoot func(path *string) error) (any, error) {
	if err := checkFileName(name); err != nil {
		return nil, failf(kindInternal, "path: %v", err)
	}
	rel := filepath.ToSlash(filepath.Join(r.dir, name))
	var path *string
	if r.write {
		full := filepath.Join(r.runDir, r.dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return nil, failf(kindInternal, "%v", err)
		}
		path = &full
	}
	if err := shoot(path); err != nil {
		return nil, classify(err, kindInternal)
	}
	if !r.write {
		return nil, nil
	}
	return rel, nil
}

func (r *scenarioRun) assert(kind string, args map[string]string) error {
	page := r.page
	switch kind {
	case "url_contains":
		if u := page.URL(); !strings.Contains(u, args["value"]) {
			return failf(kindAssert, errAssertURLContains, u, args["value"])
		}
	case "url_matches":
		re, err := regexp.Compile(args["value"])
		if err != nil {
			return failf(kindInternal, errInvalidRegexp, err)
		}
		if u := page.URL(); !re.MatchString(u) {
			return failf(kindAssert, errAssertURLMatches, u, args["value"])
		}
	case "visible", "hidden":
		state := playwright.WaitForSelectorStateVisible
		if kind == "hidden" {
			state = playwright.WaitForSelectorStateHidden
		}
		err := page.Locator(args["selector"]).WaitFor(playwright.LocatorWaitForOptions{State: state})
		if errors.Is(err, playwright.ErrTimeout) {
			return failf(kindAssert, errAssertState, args["selector"], kind)
		}
		return wrap(err)
	case "text_contains":
		t, err := page.Locator(args["selector"]).TextContent()
		if err != nil {
			return classify(err, kindSelector)
		}
		if !strings.Contains(t, args["value"]) {
			return failf(kindAssert, errAssertTextContains, truncate(t, 80), args["value"])
		}
	case "equals":
		if args["left"] != args["right"] {
			return failf(kindAssert, errAssertEquals, args["left"], args["right"])
		}
	case "not_equals":
		if args["left"] == args["right"] {
			return failf(kindAssert, errAssertNotEquals, args["left"])
		}
	}
	return nil
}

// stringify は evaluate の戻り値を保持用の文字列にする。
func stringify(v any) (string, error) {
	if s, ok := v.(string); ok {
		return s, nil
	}
	b, err := json.Marshal(v)
	return string(b), err
}
