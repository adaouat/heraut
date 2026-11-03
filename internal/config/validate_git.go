package config

import (
	"fmt"
	"strings"
)

// validateGit checks every push option, at the root and under each environment (ADR-0079). git
// refuses a push option holding a line break, so it is caught here before anything is
// committed; any other value is the server's business and passes.
func validateGit(cfg *Config) []ValidationError {
	errs := validatePushOptions(cfg.Git, "git")
	for _, name := range sortedEnvKeys(cfg.Environments) {
		errs = append(errs, validatePushOptions(cfg.Environments[name].Git, "environments."+name+".git")...)
	}
	return errs
}

func validatePushOptions(g *Git, base string) []ValidationError {
	if g == nil || g.PushOptions == nil {
		return nil
	}
	var errs []ValidationError
	for _, list := range []struct {
		name   string
		values []string
	}{
		{"commit", g.PushOptions.Commit},
		{"tag", g.PushOptions.Tag},
	} {
		for i, v := range list.values {
			path := fmt.Sprintf("%s.push_options.%s[%d]", base, list.name, i)
			switch {
			case v == "":
				errs = append(errs, ValidationError{
					Path: path, Message: "empty push option",
					Hint: `remove the entry, or set a value such as "ci.skip"`,
				})
			case strings.ContainsAny(v, "\n\r"):
				errs = append(errs, ValidationError{
					Path: path, Message: "push option contains a line break",
					Hint: "git refuses push options with a line break; split it into one entry per option",
				})
			case strings.ContainsRune(v, 0):
				errs = append(errs, ValidationError{
					Path: path, Message: "push option contains a NUL byte",
					Hint: "remove the NUL byte from the value",
				})
			}
		}
	}
	return errs
}
