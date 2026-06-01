package config

// EffectiveTagFormat returns the tag format for env: the environment-level
// override when set, otherwise the top-level versioning.tag_format. This lets a
// single top-level format (e.g. "{env}/{version}") cover every environment while
// allowing per-environment overrides. env may be empty for non-per-env strategies.
func (c *Config) EffectiveTagFormat(env string) string {
	if env != "" {
		if envCfg, ok := c.Environments[env]; ok && envCfg.TagFormat != "" {
			return envCfg.TagFormat
		}
	}
	return c.Versioning.TagFormat
}
