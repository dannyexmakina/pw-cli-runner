package main

import (
	"fmt"
	"os"
	"regexp"
)

var varRe = regexp.MustCompile(`\$\{([A-Za-z_]+)\.([A-Za-z0-9_]+)\}`)

type varRef struct {
	NS, Key string
}

func collectVars(s string) []varRef {
	var refs []varRef
	for _, m := range varRe.FindAllStringSubmatch(s, -1) {
		refs = append(refs, varRef{m[1], m[2]})
	}
	return refs
}

// vars は1シナリオ実行中の展開文脈。
type vars struct {
	runID    string
	scenario Scenario
	id       string
	step     map[string]string
}

func (v *vars) expand(s string) (string, error) {
	var err error
	out := varRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := varRe.FindStringSubmatch(m)
		val, e := v.lookup(sub[1], sub[2])
		if e != nil && err == nil {
			err = e
		}
		return val
	})
	return out, err
}

func (v *vars) lookup(ns, key string) (string, error) {
	switch ns {
	case "env":
		if val, ok := os.LookupEnv(key); ok {
			return val, nil
		}
		return "", fmt.Errorf(errEnvUndefined, key)
	case "scenario":
		switch key {
		case "name":
			return v.scenario.Name, nil
		case "id":
			return v.id, nil
		case "group":
			return v.scenario.Group, nil
		}
	case "step":
		if val, ok := v.step[key]; ok {
			return val, nil
		}
		return "", fmt.Errorf(errStepVarUndefined, key)
	case "run":
		if key == "id" {
			return v.runID, nil
		}
	}
	return "", fmt.Errorf(errUnknownVar, ns, key)
}

func (v *vars) expandArgs(args map[string]string) (map[string]string, error) {
	out := make(map[string]string, len(args))
	for k, a := range args {
		e, err := v.expand(a)
		if err != nil {
			return nil, err
		}
		out[k] = e
	}
	return out, nil
}
