package port

import "github.com/adaouat/forge/exec"

// Runner executes external CLI commands. The contract now lives in forge; this
// alias keeps heraut's hexagonal port naming while sharing the interface.
type Runner = exec.Runner
