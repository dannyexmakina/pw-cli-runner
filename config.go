package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	defaultBrowser     = "chromium"
	defaultTimeout     = 30 * time.Second
	defaultOutputDir   = "./runs"
	defaultConcurrency = 1
)

type Config struct {
	Name        string     `yaml:"name"`
	Browser     string     `yaml:"browser"`
	Headless    *bool      `yaml:"headless"`
	Timeout     string     `yaml:"timeout"`
	OutputDir   string     `yaml:"output_dir"`
	Concurrency int        `yaml:"concurrency"`
	Scenarios   []Scenario `yaml:"scenarios"`

	Path            string        `yaml:"-"`
	SHA256          string        `yaml:"-"`
	TimeoutDuration time.Duration `yaml:"-"`
}

type Scenario struct {
	Name     string      `yaml:"name"`
	Group    string      `yaml:"group"`
	RawSteps []yaml.Node `yaml:"steps"`

	Steps []Step `yaml:"-"`
}

// Step は1アクション。Args は変数展開前の値。
type Step struct {
	Action   string
	Assert   string // Action == "assert" のときの種別
	Args     map[string]string
	FullPage bool
	Raw      any // summary に出す記述そのまま
	Line     int
}

func (c *Config) IsHeadless() bool {
	return c.Headless == nil || *c.Headless
}

func loadConfig(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	var c Config
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	sum := sha256.Sum256(b)
	c.Path = path
	c.SHA256 = hex.EncodeToString(sum[:])

	if c.Browser == "" {
		c.Browser = defaultBrowser
	}
	if c.OutputDir == "" {
		c.OutputDir = defaultOutputDir
	}
	if c.Concurrency == 0 {
		c.Concurrency = defaultConcurrency
	}
	c.TimeoutDuration = defaultTimeout
	if c.Timeout != "" {
		if c.TimeoutDuration, err = time.ParseDuration(c.Timeout); err != nil {
			return nil, fmt.Errorf(errInvalidTimeout, err)
		}
	}

	var errs []error
	for i := range c.Scenarios {
		s := &c.Scenarios[i]
		for j := range s.RawSteps {
			st, err := parseStep(&s.RawSteps[j])
			if err != nil {
				errs = append(errs, fmt.Errorf("scenarios[%d].steps[%d] (line %d): %w", i, j, s.RawSteps[j].Line, err))
				continue
			}
			s.Steps = append(s.Steps, st)
		}
	}
	if len(errs) > 0 {
		return nil, joinErrors(errs)
	}
	return &c, nil
}

// parseStep は `- action: 値` 形式の1ステップを解釈する。
func parseStep(n *yaml.Node) (Step, error) {
	if n.Kind != yaml.MappingNode || len(n.Content) != 2 {
		return Step{}, fmt.Errorf(errStepShape)
	}
	key, val := n.Content[0], n.Content[1]
	st := Step{Action: key.Value, Line: key.Line, Args: map[string]string{}}
	var raw any
	if err := val.Decode(&raw); err != nil {
		return Step{}, err
	}
	st.Raw = map[string]any{key.Value: raw}

	spec, ok := actionSpecs[st.Action]
	if !ok {
		return Step{}, fmt.Errorf(errUnknownAction, st.Action)
	}
	if st.Action == "assert" {
		return parseAssert(st, val)
	}

	switch val.Kind {
	case yaml.ScalarNode:
		if spec.short == "" {
			return Step{}, fmt.Errorf(errNoShorthand, st.Action, spec.required)
		}
		st.Args[spec.short] = val.Value
	case yaml.MappingNode:
		for k := 0; k < len(val.Content); k += 2 {
			name, v := val.Content[k].Value, val.Content[k+1]
			if !spec.allows(name) {
				return Step{}, fmt.Errorf(errUnknownArg, st.Action, name)
			}
			if v.Kind != yaml.ScalarNode {
				return Step{}, fmt.Errorf(errArgNotScalar, st.Action, name)
			}
			if name == "full_page" {
				if err := v.Decode(&st.FullPage); err != nil {
					return Step{}, fmt.Errorf(errArgNotBool, st.Action, name)
				}
				continue
			}
			st.Args[name] = v.Value
		}
	default:
		return Step{}, fmt.Errorf(errStepShape)
	}
	for _, r := range spec.required {
		if _, ok := st.Args[r]; !ok {
			return Step{}, fmt.Errorf(errArgRequired, st.Action, r)
		}
	}
	return st, nil
}

func parseAssert(st Step, val *yaml.Node) (Step, error) {
	if val.Kind != yaml.MappingNode || len(val.Content) != 2 {
		return Step{}, fmt.Errorf(errAssertShape)
	}
	kind, arg := val.Content[0].Value, val.Content[1]
	spec, ok := assertSpecs[kind]
	if !ok {
		return Step{}, fmt.Errorf(errUnknownAssert, kind)
	}
	st.Assert = kind
	if spec.scalar != "" {
		if arg.Kind != yaml.ScalarNode {
			return Step{}, fmt.Errorf(errAssertArg, kind)
		}
		st.Args[spec.scalar] = arg.Value
		return st, nil
	}
	if arg.Kind != yaml.MappingNode {
		return Step{}, fmt.Errorf(errAssertArg, kind)
	}
	for k := 0; k < len(arg.Content); k += 2 {
		name, v := arg.Content[k].Value, arg.Content[k+1]
		if !contains(spec.fields, name) || v.Kind != yaml.ScalarNode {
			return Step{}, fmt.Errorf(errUnknownArg, "assert."+kind, name)
		}
		st.Args[name] = v.Value
	}
	for _, f := range spec.fields {
		if _, ok := st.Args[f]; !ok {
			return Step{}, fmt.Errorf(errArgRequired, "assert."+kind, f)
		}
	}
	return st, nil
}

// Selector はセレクタを取るステップのセレクタ（展開前）を返す。
func (s Step) Selector() (string, bool) {
	sel, ok := s.Args["selector"]
	return sel, ok
}
