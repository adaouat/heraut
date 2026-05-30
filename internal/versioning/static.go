package versioning

// StaticResolver returns a fixed pre-resolved version without any git calls.
// Used when --version is passed to heraut release, bypassing automatic resolution
// for all strategies (ADR-0018).
type StaticResolver struct {
	result Result
}

var _ Resolver = (*StaticResolver)(nil)

// NewStaticResolver creates a resolver that always returns the given tag and version.
// tag is the full tag string (e.g. "v1.2.3"); version is the tag without any prefix.
func NewStaticResolver(tag, version string) *StaticResolver {
	return &StaticResolver{result: Result{Tag: tag, Version: version}}
}

func (r *StaticResolver) Resolve() (Result, error) {
	return r.result, nil
}
