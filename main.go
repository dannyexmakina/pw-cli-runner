package main

import (
	_ "embed"
	"fmt"
	"os"
	"os/signal"
	"regexp"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const defaultFile = "pwyr.yml"

//go:embed templates/01.yml
var defaultTemplate []byte

const defaultGitignore = "runs/\n"

func main() {
	root := newRootCmd()
	err := func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = exitErr(exitInternal, fmt.Errorf("%v", r))
			}
		}()
		return root.Execute()
	}()
	os.Exit(exitCode(err, os.Stderr))
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "pwyr",
		Short:         "YAMLで宣言したブラウザ操作シナリオをPlaywrightで実行する",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(
		newInitCmd(),
		newCheckCmd(),
		newExecCmd(modeDryrun, "全操作を実行するがファイルを書かない（セレクタ診断付き）", false),
		newExecCmd(modeRun, "本番実行する", false),
		newExecCmd(modeRun, "シナリオID または番号（1始まり）を指定して1シナリオだけ実行する", true),
		newListCmd(),
	)
	return root
}

func fileArg(args []string) string {
	if len(args) == 1 {
		return args[0]
	}
	return defaultFile
}

func loadAndCheck(args []string) (*Config, error) {
	cfg, err := loadConfig(fileArg(args))
	if err != nil {
		return nil, exitErr(exitInvalidYAML, err)
	}
	if err := checkConfig(cfg); err != nil {
		return nil, exitErr(exitInvalidYAML, err)
	}
	return cfg, nil
}

func newCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check [file]",
		Short: "YAMLを静的検証する",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadAndCheck(args)
			if err != nil {
				return err
			}
			if err := checkPDF(cfg, cfg.Browser, cfg.IsHeadless()); err != nil {
				return exitErr(exitInvalidYAML, err)
			}
			fmt.Fprintln(cmd.ErrOrStderr(), "ok")
			return nil
		},
	}
}

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "最小YAMLと.gitignoreをカレントディレクトリに書き出す",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, f := range []string{defaultFile, ".gitignore"} {
				if _, err := os.Stat(f); err == nil {
					return exitErr(exitEnv, fmt.Errorf(errFileExists, f))
				}
			}
			if err := os.WriteFile(defaultFile, defaultTemplate, 0o644); err != nil {
				return exitErr(exitEnv, err)
			}
			if err := os.WriteFile(".gitignore", []byte(defaultGitignore), 0o644); err != nil {
				return exitErr(exitEnv, err)
			}
			return nil
		},
	}
}

// newExecCmd は run / dryrun / single を作る。single は先頭引数で対象シナリオを受け、
// --dryrun で dryrun になる。
func newExecCmd(mode, short string, single bool) *cobra.Command {
	var (
		dryrun      bool
		concurrency int
		headed      bool
		browser     string
		outputDir   string
		filter      string
	)
	cmd := &cobra.Command{
		Use:   mode + " [file]",
		Short: short,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var target string
			if single {
				target, args = args[0], args[1:]
				if dryrun {
					mode = modeDryrun
				}
			}
			cfg, err := loadConfig(fileArg(args))
			if err != nil {
				return exitErr(exitInvalidYAML, err)
			}
			opts := runOptions{
				mode:        mode,
				browser:     cfg.Browser,
				headless:    cfg.IsHeadless(),
				outputDir:   cfg.OutputDir,
				concurrency: cfg.Concurrency,
			}
			f := cmd.Flags()
			if f.Changed("concurrency") {
				if concurrency < 1 {
					return exitErr(exitUsage, fmt.Errorf(errConcurrency))
				}
				opts.concurrency = concurrency
			}
			if f.Changed("headed") {
				opts.headless = !headed
			}
			if f.Changed("browser") {
				opts.browser = browser
			}
			if f.Changed("output-dir") {
				opts.outputDir = outputDir
			}
			if f.Changed("filter") {
				if opts.filter, err = regexp.Compile(filter); err != nil {
					return exitErr(exitUsage, fmt.Errorf("--filter: %w", err))
				}
			}
			if single {
				if opts.only, err = resolveScenario(cfg, opts.outputDir, target); err != nil {
					return err
				}
			}
			f.Visit(func(fl *pflag.Flag) {
				opts.flags = append(opts.flags, "--"+fl.Name, fl.Value.String())
			})

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
			defer stop()
			return execute(ctx, cfg, opts, cmd.OutOrStdout(), newProgress(cmd.ErrOrStderr()))
		},
	}
	f := cmd.Flags()
	if single {
		cmd.Use = "single <scenario-id|index> [file]"
		cmd.Args = cobra.RangeArgs(1, 2)
		f.BoolVar(&dryrun, "dryrun", false, "dryrun として実行する")
	} else {
		f.IntVar(&concurrency, "concurrency", 0, "シナリオ並行数")
		f.StringVar(&filter, "filter", "", "シナリオ名で絞り込む正規表現")
	}
	f.BoolVar(&headed, "headed", false, "ブラウザを表示して実行する")
	f.StringVar(&browser, "browser", "", "chromium | firefox | webkit")
	f.StringVar(&outputDir, "output-dir", "", "出力先ディレクトリ")
	return cmd
}
