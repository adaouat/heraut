package gitcliff

import (
	"fmt"

	"github.com/pelletier/go-toml/v2"
)

// MergeTOML deep-merges override into base.
// Maps are merged recursively; all other types (scalars, arrays) replace the base value.
// If override is empty, base is returned verbatim.
func MergeTOML(base, override string) (string, error) {
	var baseMap map[string]any
	if err := toml.Unmarshal([]byte(base), &baseMap); err != nil {
		return "", fmt.Errorf("parsing base TOML: %w", err)
	}
	if baseMap == nil {
		baseMap = make(map[string]any)
	}

	if override != "" {
		var overrideMap map[string]any
		if err := toml.Unmarshal([]byte(override), &overrideMap); err != nil {
			return "", fmt.Errorf("parsing override TOML: %w", err)
		}
		mergeMaps(baseMap, overrideMap)
	}

	out, err := toml.Marshal(baseMap)
	if err != nil {
		return "", fmt.Errorf("marshalling merged TOML: %w", err)
	}
	return string(out), nil
}

// mergeMaps deep-merges src into dst in place.
func mergeMaps(dst, src map[string]any) {
	for k, sv := range src {
		dv, exists := dst[k]
		if exists {
			dm, dIsMap := dv.(map[string]any)
			sm, sIsMap := sv.(map[string]any)
			if dIsMap && sIsMap {
				mergeMaps(dm, sm)
				continue
			}
		}
		dst[k] = sv
	}
}
