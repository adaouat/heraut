package forge

import "testing"

func TestAzureOrgFromCollectionURI(t *testing.T) {
	tests := []struct {
		name string
		uri  string
		want string
	}{
		{"modern with trailing slash", "https://dev.azure.com/myorg/", "myorg"},
		{"modern without trailing slash", "https://dev.azure.com/myorg", "myorg"},
		{"legacy with trailing slash", "https://myorg.visualstudio.com/", "myorg"},
		{"legacy without trailing slash", "https://myorg.visualstudio.com", "myorg"},
		{"self-hosted-style host, first path segment wins", "https://dev.azure.example.com/myorg/myproject/", "myorg"},
		{"empty", "", ""},
		{"malformed with no usable org", "https://", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := azureOrgFromCollectionURI(tc.uri)
			if got != tc.want {
				t.Errorf("azureOrgFromCollectionURI(%q) = %q, want %q", tc.uri, got, tc.want)
			}
		})
	}
}
