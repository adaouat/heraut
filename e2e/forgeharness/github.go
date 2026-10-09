//go:build e2e_forge

package forgeharness

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type github struct {
	repo, pattern, token string
	cfg                  Config
}

// NewGitHub returns the sandbox repository configured in c.GitHubRepo.
func NewGitHub(c Config, token string) Forge {
	return &github{repo: c.GitHubRepo, pattern: c.Pattern, token: token, cfg: c}
}

func (g *github) Name() string             { return "github" }
func (g *github) Platform() string         { return "github" }
func (g *github) Coordinates() string      { return g.repo }
func (g *github) CoordinatesKey() string   { return "repository" }
func (g *github) TokenEnv() string         { return "GH_TOKEN" }
func (g *github) Token() string            { return g.token }
func (g *github) CloneURL() string         { return "https://github.com/" + g.repo + ".git" }
func (g *github) GitAuthHeader() string    { return basicAuth("x-access-token", g.token) }
func (g *github) HasPreReleaseFlag() bool  { return true }
func (g *github) URLTag(tag string) string { return strings.ReplaceAll(tag, "+", "%2B") }

func (g *github) ReleaseURL(tag string) string {
	return "https://github.com/" + g.repo + "/releases/tag/" + g.URLTag(tag)
}
func (g *github) call(args ...string) ([]byte, error) {
	return api("gh", g.TokenEnv(), g.token, args...)
}

func (g *github) Check() error {
	if err := g.cfg.Guard(g.repo); err != nil {
		return err
	}
	out, err := g.call("repos/" + g.repo)
	if err != nil {
		return err
	}
	var r struct {
		Private bool `json:"private"`
	}
	if err := json.Unmarshal(out, &r); err != nil {
		return err
	}
	if !r.Private {
		return fmt.Errorf("refusing %q: the repository is not private", g.repo)
	}
	return nil
}

type ghRelease struct {
	ID         int    `json:"id"`
	TagName    string `json:"tag_name"`
	Body       string `json:"body"`
	Prerelease bool   `json:"prerelease"`
	Draft      bool   `json:"draft"`
	Assets     []struct {
		Name string `json:"name"`
	} `json:"assets"`
}

func (g *github) releases() ([]ghRelease, error) {
	out, err := g.call("repos/" + g.repo + "/releases?per_page=100")
	if err != nil {
		return nil, err
	}
	var rs []ghRelease
	if err := json.Unmarshal(out, &rs); err != nil {
		return nil, err
	}
	return rs, nil
}

func (g *github) Release(tag string) (Release, bool, error) {
	r, ok, err := g.releaseByTag(tag)
	if err != nil || !ok {
		return Release{}, ok, err
	}
	return Release{Tag: r.TagName, Body: r.Body, Prerelease: r.Prerelease, Draft: r.Draft, AssetCount: len(r.Assets)}, true, nil
}

// releaseByTag uses the by-tag endpoint: unlike the release list, it is consistent right after a
// release was created. That endpoint never returns drafts, so a 404 falls back to the list.
func (g *github) releaseByTag(tag string) (ghRelease, bool, error) {
	out, err := g.call("repos/" + g.repo + "/releases/tags/" + tag)
	if err != nil {
		if !isNotFound(err) {
			return ghRelease{}, false, err
		}
		return g.draftByTag(tag)
	}
	var r ghRelease
	if err := json.Unmarshal(out, &r); err != nil {
		return ghRelease{}, false, err
	}
	return r, true, nil
}

func (g *github) draftByTag(tag string) (ghRelease, bool, error) {
	rs, err := g.releases()
	if err != nil {
		return ghRelease{}, false, err
	}
	for _, r := range rs {
		if r.TagName == tag {
			return r, true, nil
		}
	}
	return ghRelease{}, false, nil
}

func (g *github) TagCommit(tag string) (string, error) {
	out, err := g.call("repos/" + g.repo + "/git/ref/tags/" + tag)
	if err != nil {
		return "", err
	}
	var ref struct {
		Object struct{ Type, SHA string } `json:"object"`
	}
	if err := json.Unmarshal(out, &ref); err != nil {
		return "", err
	}
	if ref.Object.Type != "tag" {
		return ref.Object.SHA, nil
	}
	out, err = g.call("repos/" + g.repo + "/git/tags/" + ref.Object.SHA)
	if err != nil {
		return "", err
	}
	var t struct {
		Object struct{ SHA string } `json:"object"`
	}
	if err := json.Unmarshal(out, &t); err != nil {
		return "", err
	}
	return t.Object.SHA, nil
}

func (g *github) refs(kind, prefix string) ([]string, error) {
	out, err := g.call("repos/" + g.repo + "/git/matching-refs/" + kind + "/" + prefix)
	if err != nil {
		return nil, err
	}
	var refs []struct {
		Ref string `json:"ref"`
	}
	if err := json.Unmarshal(out, &refs); err != nil {
		return nil, err
	}
	var names []string
	for _, r := range refs {
		names = append(names, strings.TrimPrefix(r.Ref, "refs/"+kind+"/"))
	}
	return names, nil
}

func (g *github) Branches(prefix string) ([]string, error) { return g.refs("heads", prefix) }
func (g *github) Tags(prefix string) ([]string, error)     { return g.refs("tags", prefix) }

func (g *github) Releases(prefix string) ([]string, error) {
	rs, err := g.releases()
	if err != nil {
		return nil, err
	}
	var tags []string
	for _, r := range rs {
		tags = append(tags, r.TagName)
	}
	return hasPrefix(tags, prefix), nil
}

func (g *github) DeleteRelease(tag string) error {
	r, ok, err := g.releaseByTag(tag)
	if err != nil || !ok {
		return err
	}
	_, err = g.call("-X", "DELETE", "repos/"+g.repo+"/releases/"+strconv.Itoa(r.ID))
	return err
}

func (g *github) DeleteTag(tag string) error {
	_, err := g.call("-X", "DELETE", "repos/"+g.repo+"/git/refs/tags/"+tag)
	return err
}

func (g *github) DeleteBranch(name string) error {
	_, err := g.call("-X", "DELETE", "repos/"+g.repo+"/git/refs/heads/"+name)
	return err
}

func (g *github) OpenAndMerge(base, head, title string) (int, error) {
	out, err := g.call("-X", "POST", "repos/"+g.repo+"/pulls", "-f", "title="+title, "-f", "head="+head, "-f", "base="+base)
	if err != nil {
		return 0, err
	}
	var pr struct {
		Number int `json:"number"`
	}
	if err := json.Unmarshal(out, &pr); err != nil {
		return 0, err
	}
	// GitHub answers 405/409 for a moment while it computes mergeability: retry a few times
	var mergeErr error
	for attempt := 0; attempt < 6; attempt++ {
		_, mergeErr = g.call("-X", "PUT", fmt.Sprintf("repos/%s/pulls/%d/merge", g.repo, pr.Number), "-f", "merge_method=merge")
		if mergeErr == nil {
			return pr.Number, nil
		}
		time.Sleep(pollEvery)
	}
	return 0, mergeErr
}
