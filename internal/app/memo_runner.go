package app

import (
	"strings"
	"sync"

	execadapter "github.com/adaouat/forge/exec"

	"github.com/adaouat/heraut/internal/port"
)

// memoRunner wraps a port.Runner, caching each Run call by its exact (name, args) key so a
// repeated call within one process is served from cache instead of spawning a second subprocess.
// It is safe only for read-only runners: HasResolvablePublishTarget and BuildPipeline both resolve
// the forge from the same caller-supplied readRunner, and without this, a zero-config release
// spawns two `git remote get-url origin` subprocesses for what is meant to be one shared
// resolution (T173). RunEnv/RunDir are not memoized — forge.Resolve's git-origin detection uses
// only Run.
type memoRunner struct {
	port.Runner
	mu    *sync.Mutex // shared with a quiet copy (quietRunner), as is cache
	cache map[string]memoResult
}

type memoResult struct {
	stdout, stderr string
	err            error
}

// NewMemoizingRunner wraps r so repeated identical Run calls are served from cache.
func NewMemoizingRunner(r port.Runner) port.Runner {
	return &memoRunner{Runner: r, mu: &sync.Mutex{}, cache: make(map[string]memoResult)}
}

// quietRunner returns a runner that never echoes a command's output, for a call whose output may hold
// a secret (the origin URL can carry a token, and --verbose echoes every output). A memoizing runner
// keeps its cache, so the call is still made once. Other runners (test doubles) are returned as they
// are.
func quietRunner(r port.Runner) port.Runner {
	switch v := r.(type) {
	case *execadapter.CmdRunner:
		quiet := *v
		quiet.Verbose = false
		return &quiet
	case *memoRunner:
		return &memoRunner{Runner: quietRunner(v.Runner), mu: v.mu, cache: v.cache}
	}
	return r
}

func (m *memoRunner) Run(name string, args ...string) (string, string, error) {
	key := memoKey(name, args)
	m.mu.Lock()
	defer m.mu.Unlock()
	if res, ok := m.cache[key]; ok {
		return res.stdout, res.stderr, res.err
	}
	stdout, stderr, err := m.Runner.Run(name, args...)
	m.cache[key] = memoResult{stdout, stderr, err}
	return stdout, stderr, err
}

func memoKey(name string, args []string) string {
	return name + "\x00" + strings.Join(args, "\x00")
}
