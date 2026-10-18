package main

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeYAML(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "s.yml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func load(t *testing.T, body string) (*Config, error) {
	t.Helper()
	cfg, err := loadConfig(writeYAML(t, body))
	if err != nil {
		return nil, err
	}
	return cfg, checkConfig(cfg)
}

func TestTemplateIsValid(t *testing.T) {
	t.Setenv("LOGIN_USER", "u")
	t.Setenv("LOGIN_PASS", "p")
	cfg, err := loadConfig("templates/01.yml")
	if err != nil {
		t.Fatal(err)
	}
	if err := checkConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if err := checkPDF(cfg, "chromium", true); err != nil {
		t.Fatal(err)
	}
}

func TestParseStepShorthandAndObject(t *testing.T) {
	cfg, err := load(t, `
scenarios:
  - name: a
    steps:
      - goto: https://example.com
      - click: "button.primary"
      - screenshot:
          path: s.png
          full_page: true
      - assert:
          url_contains: /ok
      - assert:
          equals:
            left: ""
            right: ""
`)
	if err != nil {
		t.Fatal(err)
	}
	st := cfg.Scenarios[0].Steps
	if st[0].Args["url"] != "https://example.com" || st[1].Args["selector"] != "button.primary" {
		t.Fatalf("shorthand: %+v %+v", st[0].Args, st[1].Args)
	}
	if !st[2].FullPage || st[2].Args["path"] != "s.png" {
		t.Fatalf("screenshot: %+v", st[2])
	}
	if st[3].Assert != "url_contains" || st[3].Args["value"] != "/ok" {
		t.Fatalf("assert: %+v", st[3])
	}
	if st[4].Assert != "equals" {
		t.Fatalf("equals with empty strings should parse: %+v", st[4])
	}
	if cfg.Browser != "chromium" || !cfg.IsHeadless() || cfg.Concurrency != 1 || cfg.OutputDir != "./runs" {
		t.Fatalf("defaults: %+v", cfg)
	}
}

func TestCheckErrors(t *testing.T) {
	cases := map[string]struct {
		body string
		want string
	}{
		"unknown action":   {"- tap: x", "未知のアクション"},
		"no shorthand":     {"- fill: x", "省略形は使えない"},
		"missing arg":      {"- press:\n          selector: x", "press.key: 必須"},
		"unknown arg":      {"- click:\n          selector: x\n          force: true", "未知の引数"},
		"evaluate no name": {"- evaluate:\n          script: 1", "evaluate.name: 必須"},
		"bad duration":     {"- wait: soon", "時間として解釈できない"},
		"bad regexp":       {"- assert:\n          url_matches: \"(\"", "正規表現が不正"},
		"unsafe path":      {"- screenshot: ../x.png", "パス区切り文字"},
		"empty chain":      {"- click: \"a >>  >> b\"", "空の段"},
		"undefined step":   {"- assert:\n          equals:\n            left: ${step.x}\n            right: a", "${step.x}"},
		"undefined env":    {"- goto: ${env.PWYR_TEST_UNDEFINED}", "環境変数が未定義"},
		"unknown var":      {"- goto: ${foo.bar}", "未知の変数"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := load(t, "scenarios:\n  - name: a\n    steps:\n      "+c.body+"\n")
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want %q, got %v", c.want, err)
			}
		})
	}
}

func TestStepVarDefinedByEarlierStep(t *testing.T) {
	_, err := load(t, `
scenarios:
  - name: a
    steps:
      - evaluate:
          name: n
          script: "1+1"
      - assert:
          equals:
            left: ${step.n}
            right: "2"
`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCheckGroup(t *testing.T) {
	for _, g := range []string{"a/b", `a\b`, "..", "x..y"} {
		_, err := load(t, "scenarios:\n  - name: a\n    group: \""+strings.ReplaceAll(g, `\`, `\\`)+"\"\n    steps:\n      - goto: x\n")
		if err == nil || !strings.Contains(err.Error(), "group") {
			t.Errorf("group %q: got %v", g, err)
		}
	}
}

func TestCheckPDF(t *testing.T) {
	cfg, err := load(t, "scenarios:\n  - name: a\n    steps:\n      - pdf: a.pdf\n")
	if err != nil {
		t.Fatal(err)
	}
	if err := checkPDF(cfg, "chromium", true); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		browser  string
		headless bool
	}{{"firefox", true}, {"chromium", false}} {
		if err := checkPDF(cfg, c.browser, c.headless); err == nil {
			t.Errorf("%+v: want error", c)
		}
	}
}

func TestSplitChain(t *testing.T) {
	cases := map[string][]string{
		".box >> nth=0 >> button[type=submit]": {".box", "nth=0", "button[type=submit]"},
		`text="a >> b" >> span`:                {`text="a >> b"`, "span"},
		"#single":                              {"#single"},
	}
	for in, want := range cases {
		if got := splitChain(in); !reflect.DeepEqual(got, want) {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
	}
}

func TestExpand(t *testing.T) {
	t.Setenv("PWYR_X", "env")
	v := &vars{runID: "R", id: "S", scenario: Scenario{Name: "n", Group: "g"}, step: map[string]string{"k": "v"}}
	got, err := v.expand("${env.PWYR_X}/${scenario.name}/${scenario.id}/${scenario.group}/${step.k}/${run.id}")
	if err != nil || got != "env/n/S/g/v/R" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestDiagnosisShort(t *testing.T) {
	d := &Diagnosis{Count: 1, Matched: &Matched{Tag: "button", Role: "button", Name: "ログイン"}}
	if got := d.Short(); got != `count=1 ✓ → button "ログイン"` {
		t.Errorf("got %q", got)
	}
	d = &Diagnosis{Count: 1, Matched: &Matched{Tag: "div", Text: "🔍"}}
	if got := d.Short(); got != `count=1 ✓ → div "🔍"` {
		t.Errorf("got %q", got)
	}
	d = &Diagnosis{Count: 2}
	if got := d.Short(); got != "count=2 ✗" {
		t.Errorf("got %q", got)
	}
	d = &Diagnosis{Count: 0, Resolved: []ChainResult{{".box", 3}, {"button", 0}}}
	if got := d.Message(".box >> button"); !strings.Contains(got, `"button" の段で0件`) {
		t.Errorf("got %q", got)
	}
}

func TestStringify(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{{"s", "s"}, {float64(3), "3"}, {true, "true"}, {nil, "null"}, {[]any{1, "a"}, `[1,"a"]`}}
	for _, c := range cases {
		if got, _ := stringify(c.in); got != c.want {
			t.Errorf("%v: got %q want %q", c.in, got, c.want)
		}
	}
}

func TestListScenarios(t *testing.T) {
	dir := t.TempDir()
	g := "01.認証"
	for _, s := range []*Summary{
		{RunID: "019f-b", Scenarios: []ScenarioResult{{ID: "s3", Index: 3, Name: "c", Status: statusFail}}},
		{RunID: "019f-a", Scenarios: []ScenarioResult{{ID: "s1", Index: 1, Name: "a", Group: &g, Status: statusPass}, {ID: "s2", Index: 2, Name: "b", Status: statusPass}}},
	} {
		if err := writeSummary(filepath.Join(dir, s.RunID, "summary.json"), s); err != nil {
			t.Fatal(err)
		}
	}
	var b strings.Builder
	if err := listScenarios(dir, &b); err != nil {
		t.Fatal(err)
	}
	want := "019f-a\ts1\t1\tpass\t01.認証\ta\n019f-a\ts2\t2\tpass\t-\tb\n019f-b\ts3\t3\tfail\t-\tc\n"
	if b.String() != want {
		t.Fatalf("got\n%s", b.String())
	}
	if err := listScenarios(filepath.Join(dir, "none"), &b); err == nil {
		t.Fatal("want error for missing dir")
	}
}

func TestResolveScenario(t *testing.T) {
	dir := t.TempDir()
	sum := &Summary{RunID: "019f-a", Scenarios: []ScenarioResult{
		{ID: "id-b", Index: 2, Name: "b"},
		{ID: "id-stale", Index: 1, Name: "renamed"},
	}}
	if err := writeSummary(filepath.Join(dir, sum.RunID, "summary.json"), sum); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{Scenarios: []Scenario{{Name: "a"}, {Name: "b"}}}

	cases := []struct {
		target string
		want   int
		err    string
	}{
		{"1", 1, ""},
		{"2", 2, ""},
		{"0", 0, "範囲外"},
		{"3", 0, "範囲外"},
		{"id-b", 2, ""},
		{"id-stale", 0, "一致しない"},
		{"id-none", 0, "見つからない"},
	}
	for _, c := range cases {
		got, err := resolveScenario(cfg, dir, c.target)
		if c.err == "" {
			if err != nil || got != c.want {
				t.Errorf("%s: got %d, %v", c.target, got, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), c.err) || exitCode(err, io.Discard) != exitUsage {
			t.Errorf("%s: want %q (exit 2), got %v", c.target, c.err, err)
		}
	}
}
