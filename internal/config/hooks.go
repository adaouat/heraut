package config

// Hooks configures shell commands run at points in the release lifecycle (ADR-0053).
// post_bump/pre_changelog/pre_tag/post_tag fire in both `heraut release` and
// `heraut changelog --tag`; pre_release/post_release fire only in `heraut release`, once per
// publish target.
type Hooks struct {
	// PostBump runs immediately after the next version is resolved.
	PostBump []string `yaml:"post_bump,omitempty"`
	// PreChangelog runs before changelog generation.
	PreChangelog []string `yaml:"pre_changelog,omitempty"`
	// PreTag runs before the local git tag is created.
	PreTag []string `yaml:"pre_tag,omitempty"`
	// PostTag runs after the tag is pushed to origin.
	PostTag []string `yaml:"post_tag,omitempty"`
	// PreRelease runs before publishing to a platform, once per platform.
	PreRelease []string `yaml:"pre_release,omitempty"`
	// PostRelease runs after publishing to a platform, once per platform.
	PostRelease []string `yaml:"post_release,omitempty"`
}

// PostBumpHooks returns the configured post_bump hook commands, or nil. Nil-safe.
func (c *Config) PostBumpHooks() []string {
	if c.Hooks == nil {
		return nil
	}
	return c.Hooks.PostBump
}

// PreChangelogHooks returns the configured pre_changelog hook commands, or nil. Nil-safe.
func (c *Config) PreChangelogHooks() []string {
	if c.Hooks == nil {
		return nil
	}
	return c.Hooks.PreChangelog
}

// PreTagHooks returns the configured pre_tag hook commands, or nil. Nil-safe.
func (c *Config) PreTagHooks() []string {
	if c.Hooks == nil {
		return nil
	}
	return c.Hooks.PreTag
}

// PostTagHooks returns the configured post_tag hook commands, or nil. Nil-safe.
func (c *Config) PostTagHooks() []string {
	if c.Hooks == nil {
		return nil
	}
	return c.Hooks.PostTag
}

// PreReleaseHooks returns the configured pre_release hook commands, or nil. Nil-safe.
func (c *Config) PreReleaseHooks() []string {
	if c.Hooks == nil {
		return nil
	}
	return c.Hooks.PreRelease
}

// PostReleaseHooks returns the configured post_release hook commands, or nil. Nil-safe.
func (c *Config) PostReleaseHooks() []string {
	if c.Hooks == nil {
		return nil
	}
	return c.Hooks.PostRelease
}
