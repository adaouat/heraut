package testutil

import "github.com/adaouat/heraut/internal/port"

// MockPlatform is a port.Platform that records calls for contract testing.
type MockPlatform struct {
	PlatformName     string
	CheckErr         error
	CreateReleaseErr error
	HasAssetsVal     bool
	UploadAssetsErr  error
	LinkContextVal   port.LinkContext

	CreateReleaseCalls []struct{ Tag, Notes string }
	UploadAssetsCalls  []string
}

func (m *MockPlatform) Name() string { return m.PlatformName }

func (m *MockPlatform) ReleaseURL(tag string) string {
	return "https://example.com/releases/" + tag
}

func (m *MockPlatform) LinkContext() port.LinkContext { return m.LinkContextVal }

func (m *MockPlatform) Check() error { return m.CheckErr }

func (m *MockPlatform) CreateRelease(tag, notes string) error {
	m.CreateReleaseCalls = append(m.CreateReleaseCalls, struct{ Tag, Notes string }{tag, notes})
	return m.CreateReleaseErr
}

func (m *MockPlatform) HasAssets() bool { return m.HasAssetsVal }

func (m *MockPlatform) UploadAssets(tag string) error {
	m.UploadAssetsCalls = append(m.UploadAssetsCalls, tag)
	return m.UploadAssetsErr
}
