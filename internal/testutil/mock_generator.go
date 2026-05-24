package testutil

// MockGenerator is a port.Generator that records calls for contract testing.
type MockGenerator struct {
	CheckErr    error
	ValidateErr error
	GenerateOut string
	GenerateErr error
	GenerateCalls []string // tags passed to Generate
}

func (m *MockGenerator) Check() error { return m.CheckErr }

func (m *MockGenerator) Validate() error { return m.ValidateErr }

func (m *MockGenerator) Generate(tag string) (string, error) {
	m.GenerateCalls = append(m.GenerateCalls, tag)
	return m.GenerateOut, m.GenerateErr
}
