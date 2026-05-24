package versioning

// Resolver resolves the next version for a release.
type Resolver interface {
	Resolve() (Result, error)
}
