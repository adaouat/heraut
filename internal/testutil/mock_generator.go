package testutil

import "github.com/adaouat/heraut/internal/port"

// MockGenerator is a port.Generator that records calls for contract testing.
type MockGenerator struct {
	CheckErr         error
	ValidateErr      error
	GenerateOut      string
	GenerateErr      error
	GenerateCalls    []string            // tags passed to Generate
	GenerateContexts []*port.LinkContext // link context passed alongside each tag (nil = single-platform)
	DegradedVal      bool                // value returned by Degraded()
}

func (m *MockGenerator) Check() error { return m.CheckErr }

func (m *MockGenerator) Validate() error { return m.ValidateErr }

func (m *MockGenerator) Generate(tag string, lc *port.LinkContext) (string, error) {
	m.GenerateCalls = append(m.GenerateCalls, tag)
	m.GenerateContexts = append(m.GenerateContexts, lc)
	return m.GenerateOut, m.GenerateErr
}

// Degraded reports whether the generator fell back to offline mode (T78). It mirrors the
// optional Degraded() method the pipeline type-asserts on real generators.
func (m *MockGenerator) Degraded() bool { return m.DegradedVal }
