package port

// Runner executes external CLI commands.
type Runner interface {
	Run(name string, args ...string) (stdout, stderr string, err error)
	RunEnv(env []string, name string, args ...string) (stdout, stderr string, err error)
}
