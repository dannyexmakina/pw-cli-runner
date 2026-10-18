package main

import (
	"fmt"
	"strconv"
)

// resolveScenario は single の対象（番号またはシナリオID）を YAML 上の番号（1始まり）にする。
// シナリオIDは実行ごとに採番されるため、outputDir 配下の過去の summary.json から引き、
// その時点の番号と名前が今の YAML と一致する場合のみ受け付ける。
func resolveScenario(cfg *Config, outputDir, target string) (int, error) {
	if n, err := strconv.Atoi(target); err == nil {
		if n < 1 || n > len(cfg.Scenarios) {
			return 0, exitErr(exitUsage, fmt.Errorf(errIndexOutOfRange, n, len(cfg.Scenarios)))
		}
		return n, nil
	}
	sums, err := readSummaries(outputDir)
	if err != nil {
		return 0, err
	}
	for _, s := range sums {
		for _, sc := range s.Scenarios {
			if sc.ID != target {
				continue
			}
			if sc.Index < 1 || sc.Index > len(cfg.Scenarios) || cfg.Scenarios[sc.Index-1].Name != sc.Name {
				return 0, exitErr(exitUsage, fmt.Errorf(errScenarioIDStale, target, sc.Index, sc.Name))
			}
			return sc.Index, nil
		}
	}
	return 0, exitErr(exitUsage, fmt.Errorf(errScenarioIDNotFound, target, outputDir))
}
