package port

// Platform creates and manages releases on a hosting service.
type Platform interface {
	Name() string
	ReleaseURL(tag string) string
	Check() error
	CreateRelease(tag, notes string) error
	HasAssets() bool
	UploadAssets(tag string) error
}
