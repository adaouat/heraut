package port

// Platform creates and manages releases on a hosting service.
type Platform interface {
	Name() string
	ReleaseURL(tag string) string
	// LinkContext returns this platform's link-resolution coordinates (host, owner,
	// repo, type) for rendering per-platform release-notes links. The pipeline passes
	// it to the notes generator only in the multi-platform case (ADR-0020 / ADR-0021).
	LinkContext() LinkContext
	Check() error
	CreateRelease(tag, notes string) error
	HasAssets() bool
	UploadAssets(tag string) error
}
