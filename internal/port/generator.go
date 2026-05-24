package port

// Generator produces changelog or release-notes text.
type Generator interface {
	Check() error
	Validate() error
	Generate(tag string) (string, error)
}
