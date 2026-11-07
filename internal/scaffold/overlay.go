package scaffold

import "github.com/adaouat/heraut/internal/config"

// overlayAnswers lays what the wizard asks — gen, built from a by answersToConfig — over orig. The
// wizard owns the strategy, prefix, CalVer format, sprint, tag format, environments' own fields,
// whether there is a changelog and where, whether there is a release and its forges, and the
// enrichment choices; everything else is orig's. Pointed-to structs are copied before they change,
// so orig is never mutated.
func overlayAnswers(orig config.Config, a Answers, gen config.Config) config.Config {
	out := orig

	v := orig.Versioning
	v.Strategy = gen.Versioning.Strategy
	v.TagPrefix = gen.Versioning.TagPrefix
	v.Format = gen.Versioning.Format
	v.Sprint = gen.Versioning.Sprint
	v.TagFormat = a.TagFormat
	out.Versioning = v

	out.Environments = nil
	if len(gen.Environments) > 0 {
		out.Environments = make(map[string]config.Environment, len(gen.Environments))
		for name, ge := range gen.Environments {
			e := orig.Environments[name]
			e.Bump, e.TagFormat, e.Source, e.Branch = ge.Bump, ge.TagFormat, ge.Source, ge.Branch
			e.DisableChangelog, e.DisableRelease = ge.DisableChangelog, ge.DisableRelease
			e.Changelog, e.Release = ge.Changelog, ge.Release
			out.Environments[name] = e
		}
	}

	out.Changelog = gen.Changelog
	if gen.Changelog != nil && orig.Changelog != nil {
		c := *orig.Changelog
		if c.Output != "" || gen.Changelog.Output != "CHANGELOG.md" {
			c.Output = gen.Changelog.Output
		}
		out.Changelog = &c
	}

	out.Forges = append(overlayForges(orig.Forges, gen.Forges), untargetedForges(orig, gen.Forges)...)
	out.Release = overlayRelease(orig, gen, len(out.Forges))
	out.Commits = overlayCommits(orig.Commits, a, gen.Commits, out.Forges)
	return out
}

// overlayForges keeps each forge's keys the wizard doesn't ask about (api_url), matched by name.
func overlayForges(orig, gen []config.Forge) []config.Forge {
	if len(gen) == 0 {
		return nil
	}
	out := make([]config.Forge, len(gen))
	for i, gf := range gen {
		f := gf
		for _, of := range orig {
			if of.Name == gf.Name {
				f = of
				f.Type, f.Repository, f.Project = gf.Type, gf.Repository, gf.Project
				f.TokenEnv, f.APIMode, f.BaseURL = gf.TokenEnv, gf.APIMode, gf.BaseURL
				break
			}
		}
		out[i] = f
	}
	return out
}

// untargetedForges returns orig's forges no release target uses: forges for PR data only, which the
// wizard never shows (it asks about release targets), so it has nothing to say about them.
func untargetedForges(orig config.Config, gen []config.Forge) []config.Forge {
	var out []config.Forge
	for _, f := range orig.Forges {
		if forgeNameExists(gen, f.Name) {
			continue
		}
		if _, targeted := origTarget(orig, f.Name); targeted {
			continue
		}
		out = append(out, f)
	}
	return out
}

// overlayRelease keeps the release keys the wizard doesn't ask about (notes, allow_missing_assets) and
// each target's own assets, matched by the forge it names. A target that named no forge keeps naming
// none while there is a single forge.
func overlayRelease(orig, gen config.Config, forges int) *config.Release {
	if gen.Release == nil {
		return nil
	}
	r := *gen.Release
	if orig.Release != nil {
		r = *orig.Release
		r.Assets = gen.Release.Assets
	}
	r.Targets = nil
	for _, gt := range gen.Release.Targets {
		t := gt
		if existing, ok := origTarget(orig, gt.Forge); ok {
			t = existing
			t.Draft = gt.Draft
			if t.Forge == "" && forges > 1 {
				t.Forge = gt.Forge
			}
		}
		r.Targets = append(r.Targets, t)
	}
	return &r
}

// origTarget finds orig's release target for the named forge; a target without a forge names the
// single configured one.
func origTarget(orig config.Config, forge string) (config.Target, bool) {
	if orig.Release == nil {
		return config.Target{}, false
	}
	for _, t := range orig.Release.Targets {
		name := t.Forge
		if name == "" && len(orig.Forges) == 1 {
			name = orig.Forges[0].Name
		}
		if name == forge {
			return t, true
		}
	}
	return config.Target{}, false
}

// overlayCommits keeps the commit keys the wizard doesn't ask about (types, scopes, rules, …) and sets
// the enrichment choices; an enrichment forge that no longer names a forge is dropped.
func overlayCommits(orig *config.Commits, a Answers, gen *config.Commits, forges []config.Forge) *config.Commits {
	if orig == nil {
		return gen
	}
	c := *orig
	c.Tickets = a.Tickets
	c.EnrichmentPolicy = a.EnrichmentPolicy
	c.EnrichmentForge = ""
	if gen != nil && gen.EnrichmentForge != "" {
		c.EnrichmentForge = gen.EnrichmentForge
	} else if forgeNameExists(forges, a.EnrichmentForge) {
		c.EnrichmentForge = a.EnrichmentForge
	}
	return &c
}
