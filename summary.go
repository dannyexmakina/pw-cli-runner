package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

const timeFormat = "2006-01-02T15:04:05.000Z07:00"

func newV7() string {
	return uuid.Must(uuid.NewV7()).String()
}

type jsonTime time.Time

func (t jsonTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(t).Format(timeFormat))
}

type Summary struct {
	RunID      string           `json:"run_id"`
	Mode       string           `json:"mode"`
	YAML       string           `json:"yaml"`
	YAMLSHA256 string           `json:"yaml_sha256"`
	Env        SummaryEnv       `json:"env"`
	Browser    string           `json:"browser"`
	StartedAt  jsonTime         `json:"started_at"`
	FinishedAt jsonTime         `json:"finished_at"`
	DurationMS int64            `json:"duration_ms"`
	Status     string           `json:"status"`
	Totals     Totals           `json:"totals"`
	Scenarios  []ScenarioResult `json:"scenarios"`
}

type SummaryEnv struct {
	Pwyr         string   `json:"pwyr"`
	PlaywrightGo string   `json:"playwright_go"`
	OS           string   `json:"os"`
	Arch         string   `json:"arch"`
	Flags        []string `json:"flags"`
}

type Totals struct {
	Scenarios int `json:"scenarios"`
	Passed    int `json:"passed"`
	Failed    int `json:"failed"`
}

type ScenarioResult struct {
	ID         string            `json:"id"`
	Index      int               `json:"index"`
	Name       string            `json:"name"`
	Group      *string           `json:"group"`
	Status     string            `json:"status"`
	DurationMS int64             `json:"duration_ms"`
	Error      *ScenarioError    `json:"error"`
	Extracted  map[string]string `json:"extracted"`
	Steps      []StepResult      `json:"steps"`
}

type ScenarioError struct {
	Step    int    `json:"step"`
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

type StepResult struct {
	Index      int        `json:"index"`
	Action     string     `json:"action"`
	Args       any        `json:"args"`
	Status     string     `json:"status"`
	DurationMS int64      `json:"duration_ms"`
	Error      *string    `json:"error"`
	Output     any        `json:"output"`
	Diagnosis  *Diagnosis `json:"diagnosis"`
}

const (
	statusPass    = "pass"
	statusFail    = "fail"
	statusSkipped = "skipped"

	kindTimeout    = "timeout"
	kindSelector   = "selector"
	kindAssert     = "assert"
	kindNavigation = "navigation"
	kindInternal   = "internal"
)

func writeSummary(path string, s *Summary) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := marshalSummary(s)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// marshalSummary はセレクタの `>>` 等がエスケープされないよう HTML エスケープを切る。
func marshalSummary(s *Summary) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
