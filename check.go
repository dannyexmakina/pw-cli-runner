package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var nameRe = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

func checkConfig(c *Config) error {
	var errs []error
	addf := func(format string, args ...any) {
		errs = append(errs, fmt.Errorf(format, args...))
	}

	if !contains(browsers, c.Browser) {
		addf(errBrowserUnsupported, c.Browser, strings.Join(browsers, ", "))
	}
	if c.Concurrency < 1 {
		addf(errConcurrency)
	}
	if c.TimeoutDuration <= 0 {
		addf(errTimeoutPositive)
	}
	if len(c.Scenarios) == 0 {
		addf(errScenariosRequired)
	}

	for i, s := range c.Scenarios {
		at := fmt.Sprintf("scenarios[%d]", i)
		if s.Name == "" {
			addf(errScenarioNameRequired, at)
		}
		if err := checkGroup(s.Group); err != nil {
			addf("%s.group: %v", at, err)
		}
		if len(s.Steps) == 0 {
			addf(errStepsRequired, at)
		}
		defined := map[string]bool{}
		for j, st := range s.Steps {
			where := fmt.Sprintf("%s.steps[%d] %s (line %d)", at, j, st.Action, st.Line)
			for _, e := range checkStep(st, defined) {
				addf("%s: %s", where, e)
			}
			if st.Action == "extract" || st.Action == "evaluate" {
				defined[st.Args["name"]] = true
			}
		}
	}
	return joinErrors(errs)
}

func checkStep(st Step, defined map[string]bool) []string {
	var errs []string
	add := func(format string, args ...any) {
		errs = append(errs, fmt.Sprintf(format, args...))
	}

	for _, a := range st.Args {
		for _, r := range collectVars(a) {
			if e := checkVar(r, defined); e != "" {
				add("%s", e)
			}
		}
	}

	if sel, ok := st.Selector(); ok {
		if e := checkSelector(sel); e != "" {
			add("%s", e)
		}
	}
	isStatic := func(s string) bool { return len(collectVars(s)) == 0 }

	switch st.Action {
	case "goto":
		if st.Args["url"] == "" {
			add(errEmpty, "url")
		}
	case "wait":
		if d := st.Args["duration"]; isStatic(d) {
			if _, err := time.ParseDuration(d); err != nil {
				add(errInvalidDuration, d)
			}
		}
	case "screenshot", "pdf":
		if p := st.Args["path"]; isStatic(p) {
			if err := checkFileName(p); err != nil {
				add("path: %v", err)
			}
		}
	case "evaluate", "extract":
		if !nameRe.MatchString(st.Args["name"]) {
			add(errInvalidVarName, st.Args["name"])
		}
		if st.Action == "evaluate" && strings.TrimSpace(st.Args["script"]) == "" {
			add(errEmpty, "script")
		}
		if st.Action == "extract" && st.Args["attr"] == "" {
			add(errEmpty, "attr")
		}
	case "assert":
		if st.Assert == "url_matches" && isStatic(st.Args["value"]) {
			if _, err := regexp.Compile(st.Args["value"]); err != nil {
				add(errInvalidRegexp, err)
			}
		}
	}
	return errs
}

func checkVar(r varRef, defined map[string]bool) string {
	switch r.NS {
	case "env":
		if _, ok := os.LookupEnv(r.Key); !ok {
			return fmt.Sprintf(errEnvUndefined, r.Key)
		}
		return ""
	case "scenario":
		if contains([]string{"name", "id", "group"}, r.Key) {
			return ""
		}
	case "step":
		if defined[r.Key] {
			return ""
		}
		return fmt.Sprintf(errStepVarUndefined, r.Key)
	case "run":
		if r.Key == "id" {
			return ""
		}
	}
	return fmt.Sprintf(errUnknownVar, r.NS, r.Key)
}

func checkSelector(sel string) string {
	if strings.TrimSpace(sel) == "" {
		return fmt.Sprintf(errEmpty, "selector")
	}
	for _, part := range splitChain(sel) {
		if strings.TrimSpace(part) == "" {
			return fmt.Sprintf(errSelectorChain, sel)
		}
	}
	return ""
}

// checkGroup と checkFileName は出力先パスの1要素として安全かを見る。
func checkGroup(g string) error {
	if g == "" {
		return nil
	}
	return checkFileName(g)
}

func checkFileName(p string) error {
	if p == "" {
		return fmt.Errorf(errEmpty, "path")
	}
	if strings.ContainsAny(p, `/\`) || strings.Contains(p, "..") || filepath.IsAbs(p) {
		return fmt.Errorf(errUnsafeName, p)
	}
	return nil
}

// checkPDF は実効設定で pdf が使えるかを見る（Chromium headless 限定）。
func checkPDF(c *Config, browser string, headless bool) error {
	if browser == "chromium" && headless {
		return nil
	}
	var errs []error
	for i, s := range c.Scenarios {
		for j, st := range s.Steps {
			if st.Action == "pdf" {
				errs = append(errs, fmt.Errorf(errPDFUnsupported, i, j, st.Line, browser, headless))
			}
		}
	}
	return joinErrors(errs)
}

func joinErrors(errs []error) error {
	return errors.Join(errs...)
}
