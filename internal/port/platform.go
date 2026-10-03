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
	// CreateRelease publishes a release for tag with notes. prerelease is derived by the caller
	// from the resolved version, but only under a SemVer strategy (ADR-0064) — a SemVer
	// pre-release like 2.0.0-rc.1 passes true, a final version passes false, and so does every
	// CalVer version, regardless of whether it happens to parse as a SemVer pre-release (CalVer
	// has no pre-release concept). A driver with no pre-release concept of its own (GitLab)
	// accepts and ignores it.
	CreateRelease(tag, notes string, prerelease bool) error
	HasAssets() bool
	UploadAssets(tag string) error
}
