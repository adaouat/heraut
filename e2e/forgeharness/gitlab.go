//go:build e2e_forge

package forgeharness

import (
	"encoding/json"
	"fmt"
	"strings"
)

type gitlab struct {
	project, token string
	cfg            Config
}

// NewGitLab returns the sandbox project configured in c.GitLabProject.
func NewGitLab(c Config, token string) Forge {
	return &gitlab{project: c.GitLabProject, token: token, cfg: c}
}

func (g *gitlab) Name() string            { return "gitlab" }
func (g *gitlab) Platform() string        { return "gitlab" }
func (g *gitlab) Coordinates() string     { return g.project }
func (g *gitlab) CoordinatesKey() string  { return "project" }
func (g *gitlab) TokenEnv() string        { return "GITLAB_TOKEN" }
func (g *gitlab) Token() string           { return g.token }
func (g *gitlab) CloneURL() string        { return "https://gitlab.com/" + g.project + ".git" }
func (g *gitlab) GitAuthHeader() string   { return basicAuth("oauth2", g.token) }
func (g *gitlab) HasPreReleaseFlag() bool { return false }

func (g *gitlab) ReleaseURL(tag string) string {
	return "https://gitlab.com/" + g.project + "/-/releases/" + g.URLTag(tag)
}

func (g *gitlab) URLTag(tag string) string {
	return strings.NewReplacer("+", "%2B", "/", "%2F").Replace(tag)
}

func (g *gitlab) base() string { return "projects/" + escape(g.project) }

func (g *gitlab) call(args ...string) ([]byte, error) {
	return api("glab", g.TokenEnv(), g.token, args...)
}

func (g *gitlab) Check() error {
	if err := g.cfg.Guard(g.project); err != nil {
		return err
	}
	out, err := g.call(g.base())
	if err != nil {
		return err
	}
	var p struct {
		Visibility string `json:"visibility"`
	}
	if err := json.Unmarshal(out, &p); err != nil {
		return err
	}
	if p.Visibility != "private" {
		return fmt.Errorf("refusing %q: the project is not private (visibility %q)", g.project, p.Visibility)
	}
	return nil
}

func (g *gitlab) Release(tag string) (Release, bool, error) {
	out, err := g.call(g.base() + "/releases/" + escape(tag))
	if err != nil {
		if isNotFound(err) {
			return Release{}, false, nil
		}
		return Release{}, false, err
	}
	var r struct {
		TagName     string `json:"tag_name"`
		Description string `json:"description"`
		Assets      struct {
			Links []struct{ Name string } `json:"links"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(out, &r); err != nil {
		return Release{}, false, err
	}
	return Release{Tag: r.TagName, Body: r.Description, AssetCount: len(r.Assets.Links)}, true, nil
}

func (g *gitlab) TagCommit(tag string) (string, error) {
	out, err := g.call(g.base() + "/repository/tags/" + escape(tag))
	if err != nil {
		return "", err
	}
	var t struct {
		Commit struct{ ID string } `json:"commit"`
	}
	if err := json.Unmarshal(out, &t); err != nil {
		return "", err
	}
	return t.Commit.ID, nil
}

func (g *gitlab) names(path, prefix string) ([]string, error) {
	out, err := g.call(path + "?search=" + escape(prefix) + "&per_page=100")
	if err != nil {
		return nil, err
	}
	var items []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(out, &items); err != nil {
		return nil, err
	}
	var names []string
	for _, i := range items {
		names = append(names, i.Name)
	}
	return hasPrefix(names, prefix), nil
}

func (g *gitlab) Branches(prefix string) ([]string, error) {
	return g.names(g.base()+"/repository/branches", prefix)
}

func (g *gitlab) Tags(prefix string) ([]string, error) {
	return g.names(g.base()+"/repository/tags", prefix)
}

func (g *gitlab) Releases(prefix string) ([]string, error) {
	out, err := g.call(g.base() + "/releases?per_page=100")
	if err != nil {
		return nil, err
	}
	var rs []struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(out, &rs); err != nil {
		return nil, err
	}
	var tags []string
	for _, r := range rs {
		tags = append(tags, r.TagName)
	}
	return hasPrefix(tags, prefix), nil
}

func (g *gitlab) DeleteRelease(tag string) error {
	_, err := g.call("-X", "DELETE", g.base()+"/releases/"+escape(tag))
	if isNotFound(err) {
		return nil
	}
	return err
}

func (g *gitlab) DeleteTag(tag string) error {
	_, err := g.call("-X", "DELETE", g.base()+"/repository/tags/"+escape(tag))
	return err
}

func (g *gitlab) DeleteBranch(name string) error {
	_, err := g.call("-X", "DELETE", g.base()+"/repository/branches/"+escape(name))
	return err
}
