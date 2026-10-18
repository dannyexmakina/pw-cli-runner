package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"sync"
	"time"

	"github.com/mxschmitt/playwright-go"
)

const (
	modeRun    = "run"
	modeDryrun = "dryrun"

	playwrightModule = "github.com/mxschmitt/playwright-go"
)

var version = "dev"

// runOptions は YAML 値をフラグで上書きした実効設定。
type runOptions struct {
	mode        string
	browser     string
	headless    bool
	outputDir   string
	concurrency int
	filter      *regexp.Regexp
	only        int // 0 以外なら、この番号（1始まり）のシナリオだけ実行する
	flags       []string
}

func (o runOptions) write() bool { return o.mode == modeRun }

func execute(ctx context.Context, cfg *Config, opts runOptions, stdout io.Writer, p *progressPrinter) error {
	if err := checkConfig(cfg); err != nil {
		return exitErr(exitInvalidYAML, err)
	}
	if !contains(browsers, opts.browser) {
		return exitErr(exitUsage, fmt.Errorf(errBrowserUnsupported, opts.browser, "chromium, firefox, webkit"))
	}
	if err := checkPDF(cfg, opts.browser, opts.headless); err != nil {
		return exitErr(exitInvalidYAML, err)
	}
	var targets []int // 実行するシナリオの番号（1始まり）
	for i, s := range cfg.Scenarios {
		if opts.only != 0 && i+1 != opts.only {
			continue
		}
		if opts.filter != nil && !opts.filter.MatchString(s.Name) {
			continue
		}
		targets = append(targets, i+1)
	}
	if len(targets) == 0 {
		return exitErr(exitUsage, fmt.Errorf(errFilterNoMatch, opts.filter))
	}

	pw, err := playwright.Run(&playwright.RunOptions{Verbose: false, Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		return exitErr(exitEnv, fmt.Errorf(errDriverMissing, err))
	}
	defer pw.Stop()

	bt := map[string]playwright.BrowserType{"chromium": pw.Chromium, "firefox": pw.Firefox, "webkit": pw.WebKit}[opts.browser]
	browser, err := bt.Launch(playwright.BrowserTypeLaunchOptions{Headless: playwright.Bool(opts.headless)})
	if err != nil {
		return exitErr(exitEnv, fmt.Errorf(errBrowserLaunch, opts.browser, firstLine(err.Error())))
	}

	sum := &Summary{
		RunID:      newV7(),
		Mode:       opts.mode,
		YAML:       cfg.Path,
		YAMLSHA256: cfg.SHA256,
		Env:        buildEnv(opts.flags),
		Browser:    opts.browser,
		StartedAt:  jsonTime(time.Now()),
	}
	runDir := filepath.Join(opts.outputDir, sum.RunID)
	if opts.write() {
		if err := os.MkdirAll(runDir, 0o755); err != nil {
			browser.Close()
			return exitErr(exitEnv, err)
		}
	}

	results := make([]ScenarioResult, len(targets))
	sem := make(chan struct{}, opts.concurrency)
	var wg sync.WaitGroup
	for i, n := range targets {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = runScenario(ctx, browser, cfg, opts, sum.RunID, runDir, n, cfg.Scenarios[n-1], p)
		}()
	}
	wg.Wait()

	crashed := !browser.IsConnected()
	browser.Close()

	finished := time.Now()
	sum.FinishedAt = jsonTime(finished)
	sum.DurationMS = finished.Sub(time.Time(sum.StartedAt)).Milliseconds()
	sum.Scenarios = results
	sum.Status = statusPass
	sum.Totals.Scenarios = len(results)
	for _, r := range results {
		if r.Status == statusPass {
			sum.Totals.Passed++
		} else {
			sum.Totals.Failed++
			sum.Status = statusFail
		}
	}

	b, err := marshalSummary(sum)
	if err != nil {
		return exitErr(exitInternal, err)
	}
	if opts.write() {
		if err := os.WriteFile(filepath.Join(runDir, "summary.json"), b, 0o644); err != nil {
			return exitErr(exitEnv, err)
		}
	}
	if _, err := stdout.Write(b); err != nil {
		return exitErr(exitEnv, err)
	}

	switch {
	case crashed:
		return exitErr(exitEnv, fmt.Errorf(errBrowserCrashed))
	case sum.Status == statusFail:
		return exitErr(exitScenarioFailed, nil)
	}
	return nil
}

func runScenario(ctx context.Context, browser playwright.Browser, cfg *Config, opts runOptions, runID, runDir string, index int, s Scenario, p *progressPrinter) ScenarioResult {
	id := newV7()
	start := time.Now()
	res := ScenarioResult{ID: id, Index: index, Name: s.Name, Status: statusPass, Extracted: map[string]string{}}
	if s.Group != "" {
		res.Group = &s.Group
	}
	for i, st := range s.Steps {
		res.Steps = append(res.Steps, StepResult{Index: i + 1, Action: st.Action, Args: st.Raw, Status: statusSkipped})
	}
	label := fmt.Sprintf("%s:%s", id[len(id)-8:], s.Name)
	fail := func(step int, e *stepError) {
		res.Status = statusFail
		res.Error = &ScenarioError{Step: step, Kind: e.kind, Message: e.msg}
	}
	defer func() {
		res.DurationMS = time.Since(start).Milliseconds()
		if res.Status == statusPass {
			p.Printf(label, "PASS (%.1fs)", time.Since(start).Seconds())
		} else {
			p.Printf(label, "FAIL at step %d: %s", res.Error.Step, res.Error.Message)
		}
	}()

	bctx, err := browser.NewContext()
	if err != nil {
		fail(0, classify(err, kindInternal))
		return res
	}
	defer bctx.Close()
	page, err := bctx.NewPage()
	if err != nil {
		fail(0, classify(err, kindInternal))
		return res
	}
	page.SetDefaultTimeout(float64(cfg.TimeoutDuration.Milliseconds()))

	r := &scenarioRun{
		ctx:     ctx,
		page:    page,
		vars:    &vars{runID: runID, scenario: s, id: id, step: res.Extracted},
		dir:     filepath.Join("scenarios", id, s.Group),
		runDir:  runDir,
		write:   opts.write(),
		timeout: cfg.TimeoutDuration,
	}

	for i, st := range s.Steps {
		if ctx.Err() != nil {
			fail(i+1, &stepError{kind: kindInternal, msg: errInterrupted})
			break
		}
		sr := &res.Steps[i]
		t0 := time.Now()
		out, diag, err := r.step(st, label, p)
		sr.DurationMS = time.Since(t0).Milliseconds()
		sr.Diagnosis = diag
		if err != nil {
			e := classify(err, kindInternal)
			sr.Status = statusFail
			sr.Error = &e.msg
			fail(i+1, e)
			break
		}
		sr.Status = statusPass
		sr.Output = out
	}
	return res
}

// step は1ステップを診断込みで実行する。
func (r *scenarioRun) step(st Step, label string, p *progressPrinter) (any, *Diagnosis, error) {
	args, err := r.vars.expandArgs(st.Args)
	if err != nil {
		return nil, nil, failf(kindInternal, "%v", err)
	}
	_, hasSel := st.Selector()
	detail := stepDetail(st)
	dry := !r.write

	var diag *Diagnosis
	if dry && hasSel && st.Action != "waitFor" {
		// dryrun: 操作前に診断する。自動待機に合わせ、まず出現を待つ。
		if st.Assert != "hidden" {
			r.page.Locator(args["selector"]).First().WaitFor(playwright.LocatorWaitForOptions{State: playwright.WaitForSelectorStateAttached})
		}
		diag = diagnose(r.page, args["selector"])
		p.Printf(label, "%s %s %s", st.Action, detail, diag.Short())
		if !diag.OK() && st.Assert != "hidden" {
			return nil, diag, failf(kindSelector, "%s", diag.Message(args["selector"]))
		}
	} else if !(dry && hasSel) {
		p.Printf(label, "%s %s", st.Action, detail)
	}

	out, err := r.exec(st, args)

	if dry && st.Action == "waitFor" {
		diag = diagnose(r.page, args["selector"])
		p.Printf(label, "%s %s %s", st.Action, detail, diag.Short())
	}
	if err != nil && hasSel && diag == nil {
		// run: 失敗時のみ診断する
		diag = diagnose(r.page, args["selector"])
		p.Printf(label, "%s %s %s", st.Action, detail, diag.Short())
	}
	if err != nil && diag != nil && !diag.OK() && st.Assert != "hidden" {
		err = failf(kindSelector, "%s", diag.Message(args["selector"]))
	}
	return out, diag, err
}

// stepDetail は進捗表示用の要約。値は秘匿値を含み得るため展開前のものを使い、fill 等の value は出さない。
func stepDetail(st Step) string {
	if sel, ok := st.Selector(); ok {
		if st.Assert != "" {
			return st.Assert + " " + sel
		}
		return sel
	}
	switch st.Action {
	case "goto":
		return st.Args["url"]
	case "wait":
		return st.Args["duration"]
	case "screenshot", "pdf":
		return st.Args["path"]
	case "evaluate":
		return st.Args["name"]
	case "assert":
		return st.Assert
	}
	return ""
}

func buildEnv(flags []string) SummaryEnv {
	env := SummaryEnv{Pwyr: version, OS: runtime.GOOS, Arch: runtime.GOARCH, Flags: flags}
	if env.Flags == nil {
		env.Flags = []string{}
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, d := range info.Deps {
			if d.Path == playwrightModule {
				env.PlaywrightGo = d.Version
			}
		}
	}
	return env
}
