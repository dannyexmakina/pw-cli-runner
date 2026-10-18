package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"
)

// listedSummary は summary.json から一覧表示に要る項目だけを読む。
type listedSummary struct {
	RunID     string `json:"run_id"`
	Scenarios []struct {
		ID     string  `json:"id"`
		Index  int     `json:"index"`
		Name   string  `json:"name"`
		Group  *string `json:"group"`
		Status string  `json:"status"`
	} `json:"scenarios"`
}

func newListCmd() *cobra.Command {
	var outputDir string
	cmd := &cobra.Command{
		Use:   "list [file]",
		Short: "output_dir 配下の過去の実行からシナリオIDを一覧表示する",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := outputDir
			if !cmd.Flags().Changed("output-dir") {
				cfg, err := loadConfig(fileArg(args))
				if err != nil {
					return exitErr(exitInvalidYAML, err)
				}
				dir = cfg.OutputDir
			}
			return listScenarios(dir, cmd.OutOrStdout())
		},
	}
	cmd.Flags().StringVar(&outputDir, "output-dir", "", "一覧対象の出力ディレクトリ（指定時はYAMLを読まない）")
	return cmd
}

// readSummaries は dir 配下の */summary.json を実行ID順（UUIDv7のため実行順）に読む。
func readSummaries(dir string) ([]listedSummary, error) {
	if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
		return nil, exitErr(exitEnv, fmt.Errorf(errOutputDirMissing, dir))
	}
	paths, err := filepath.Glob(filepath.Join(dir, "*", "summary.json"))
	if err != nil {
		return nil, exitErr(exitInternal, err)
	}
	sort.Strings(paths)
	var out []listedSummary
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, exitErr(exitEnv, err)
		}
		var s listedSummary
		if err := json.Unmarshal(b, &s); err != nil {
			return nil, exitErr(exitEnv, fmt.Errorf("%s: %w", p, err))
		}
		out = append(out, s)
	}
	return out, nil
}

// listScenarios は1シナリオ1行で `実行ID<TAB>シナリオID<TAB>番号<TAB>status<TAB>group<TAB>name` を出す。
func listScenarios(dir string, w io.Writer) error {
	sums, err := readSummaries(dir)
	if err != nil {
		return err
	}
	for _, s := range sums {
		for _, sc := range s.Scenarios {
			group := "-"
			if sc.Group != nil {
				group = *sc.Group
			}
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%s\n", s.RunID, sc.ID, sc.Index, sc.Status, group, sc.Name)
		}
	}
	return nil
}
