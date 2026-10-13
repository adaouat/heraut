package cmd

import (
	"errors"
	"fmt"
	"os"

	execadapter "github.com/adaouat/forge/exec"
	"github.com/spf13/cobra"

	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/exitcode"
	"github.com/adaouat/heraut/internal/port"
)

type configFinding struct {
	Path    string `json:"path"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

type configReport struct {
	Path   string          `json:"path"`
	Source string          `json:"source,omitempty"`
	Found  bool            `json:"found"`
	Valid  bool            `json:"valid"`
	Errors []configFinding `json:"errors"`
}

type runtimeFinding struct {
	Section string `json:"section"`
	Name    string `json:"name"`
	Status  string `json:"status"` // ok, warn or fail
	Detail  string `json:"detail,omitempty"`
}

type runtimeReport struct {
	Checks []runtimeFinding `json:"checks"`
}

func newConfigReport(path string, source config.PathSource, cfg *config.Config) (configReport, int) {
	r := configReport{Path: path, Source: string(source), Found: cfg != nil, Errors: []configFinding{}}
	if cfg == nil {
		return r, 0
	}
	errs := config.Validate(cfg)
	for _, e := range errs {
		r.Errors = append(r.Errors, configFinding{Path: e.Path, Message: e.Message, Hint: e.Hint})
	}
	r.Valid = len(errs) == 0
	return r, len(errs)
}

// collectRuntime runs the runtime checks without any spinner and returns one finding per check and
// the number of hard failures (warnings do not count).
func collectRuntime(runner port.Runner, cfg *config.Config, env string) (runtimeReport, int) {
	report := runtimeReport{Checks: []runtimeFinding{}}
	var section string
	var failed int
	app.RuntimeCheck(runner, cfg, env,
		func(title string) { section = title },
		func(name string, run func() app.RuntimeCheckItem) {
			item := run()
			f := runtimeFinding{Section: section, Name: name, Status: "ok", Detail: item.Value}
			switch {
			case item.IsWarn:
				f.Status, f.Detail = "warn", item.Err.Error()
			case item.Err != nil:
				f.Status, f.Detail = "fail", item.Err.Error()
				failed++
			}
			report.Checks = append(report.Checks, f)
		},
	)
	return report, failed
}

// loadOptionalConfig loads the config, treating a missing file as "no config" (nil, nil).
func loadOptionalConfig(path string) (*config.Config, error) {
	cfg, err := config.Load(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, exitcode.Wrap(exitcode.Config, err)
	}
	return cfg, nil
}

func runCheckAllJSON(cmd *cobra.Command) error {
	out := cmd.OutOrStdout()
	cfgPath, _ := cmd.Flags().GetString("config")
	verbose, _ := cmd.Flags().GetBool("verbose")
	env, _ := cmd.Flags().GetString("env")
	runner := execadapter.New(false, verbose)

	path, source := config.ResolvePathWithSource(cfgPath)
	cfg, err := loadOptionalConfig(path)
	if err != nil {
		return writeJSON(out, nil, nil, err)
	}
	if cfg != nil {
		applyOfflineOverride(cmd, cfg)
	}
	env, err = app.ResolveEnv(env, cfg, runner)
	if err != nil {
		return writeJSON(out, nil, nil, exitcode.Wrap(exitcode.Config, err))
	}

	cfgReport, configFailed := newConfigReport(path, source, cfg)
	rtReport, runtimeFailed := collectRuntime(runner, cfg, env)
	data := struct {
		Config  configReport  `json:"config"`
		Runtime runtimeReport `json:"runtime"`
	}{cfgReport, rtReport}

	switch {
	case configFailed > 0:
		return writeJSON(out, data, nil, exitcode.Wrap(exitcode.Config, fmt.Errorf("one or more checks failed")))
	case runtimeFailed > 0:
		return writeJSON(out, data, nil, exitcode.Wrap(exitcode.Runtime, fmt.Errorf("one or more checks failed")))
	}
	return writeJSON(out, data, nil, nil)
}

func runCheckConfigJSON(cmd *cobra.Command) error {
	out := cmd.OutOrStdout()
	cfgPath, _ := cmd.Flags().GetString("config")
	path, source := config.ResolvePathWithSource(cfgPath)
	cfg, err := config.Load(path)
	if err != nil {
		return writeJSON(out, nil, nil, exitcode.Wrap(exitcode.Config, err))
	}
	report, failed := newConfigReport(path, source, cfg)
	if failed > 0 {
		return writeJSON(out, report, nil, exitcode.Wrap(exitcode.Config, fmt.Errorf("%d error(s) in config", failed)))
	}
	return writeJSON(out, report, nil, nil)
}

func runCheckRuntimeJSON(cmd *cobra.Command) error {
	out := cmd.OutOrStdout()
	cfgPath, _ := cmd.Flags().GetString("config")
	verbose, _ := cmd.Flags().GetBool("verbose")
	env, _ := cmd.Flags().GetString("env")
	runner := execadapter.New(false, verbose)

	cfg, err := loadOptionalConfig(config.ResolvePath(cfgPath))
	if err != nil {
		return writeJSON(out, nil, nil, err)
	}
	env, err = app.ResolveEnv(env, cfg, runner)
	if err != nil {
		return writeJSON(out, nil, nil, exitcode.Wrap(exitcode.Config, err))
	}
	report, failed := collectRuntime(runner, cfg, env)
	if failed > 0 {
		return writeJSON(out, report, nil, exitcode.Wrap(exitcode.Runtime, fmt.Errorf("one or more runtime checks failed")))
	}
	return writeJSON(out, report, nil, nil)
}
