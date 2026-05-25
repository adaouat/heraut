package selfupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// fetchRelease fetches the latest release manifest from latestURL.
func fetchRelease(ctx context.Context, client *http.Client, latestURL string) (*Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching release: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("no published releases found")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("decoding release: %w", err)
	}
	return &rel, nil
}

// assetName returns the goreleaser archive name for the given OS and arch.
// version must be the bare version WITHOUT the "v" prefix (e.g. "1.2.3").
func assetName(version, goos, goarch string) string {
	name := fmt.Sprintf("heraut_%s_%s_%s", version, goos, goarch)
	if goos == "windows" {
		name += ".exe"
	}
	return name
}

// checksumAssetName returns the goreleaser checksum filename for the given version.
func checksumAssetName(version string) string {
	return fmt.Sprintf("heraut_%s_checksums.txt", version)
}

// findAsset returns a pointer to the first asset with the given name, or nil.
func findAsset(assets []Asset, name string) *Asset {
	for i := range assets {
		if assets[i].Name == name {
			return &assets[i]
		}
	}
	return nil
}

// bareVersion strips the leading "v" from a version tag (e.g. "v1.2.3" → "1.2.3").
func bareVersion(v string) string {
	return strings.TrimPrefix(v, "v")
}
