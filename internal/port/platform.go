package port

// Platform creates and manages releases on a hosting service.
type Platform interface {
	Name() string
	ReleaseURL(tag string) string
	// ReleaseURLFromContext builds the release URL from a pre-resolved link context so
	// the pipeline can keep the displayed URL consistent with the one used to generate
	// release notes (ADR-0022). Falls back to ReleaseURL when lc is nil.
	ReleaseURLFromContext(tag string, lc *LinkContext) string
	// LinkContext returns this platform's link-resolution coordinates (host, owner,
	// repo, type) for rendering per-platform release-notes links. The pipeline passes
	// it to the notes generator only in the multi-platform case (ADR-0020 / ADR-0021).
	LinkContext() LinkContext
	Check() error
	CreateRelease(tag, notes string) error
	HasAssets() bool
	UploadAssets(tag string) error
}
