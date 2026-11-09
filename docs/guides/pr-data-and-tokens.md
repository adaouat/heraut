# Pull request data in the changelog

From git alone, heraut writes one line per commit: its message and its hash. With access to your
forge's API it adds the pull request (or merge request) each commit came from, and who opened it:

```markdown
- Add search - ([4e1c2a7](https://github.com/acme/app/commit/4e1c2a7…)) by @ada in [#12](https://github.com/acme/app/pull/12)
```

The release notes also get a **New Contributors ❤️** block naming everyone whose first contribution
is in the release, with their pull requests.

### Someone listed as new twice?

heraut recognises people by the email of their commits. Someone who committed under two emails (a
work one and a personal one) looks new again under the second. List their emails in a `.mailmap` at
the root of the repository, committed like any other file, and heraut counts them as one person:

```text
# .mailmap: the name and email to show, then each other email
Ada Lovelace <ada@example.com> <ada.lovelace@corp.example.com>
Ada Lovelace <ada@example.com> <ada@home.example.com>
```

`git log --format='%aN <%aE>' | sort -u` shows each person once when the file is right. The same
name and email then appear in the changelog everywhere.

Within one release heraut already merges two emails that lead to the same forge account, without a
`.mailmap`. Across releases, only the `.mailmap` can tell they're the same person.

## The token it needs

| Forge | Where | Token |
|---|---|---|
| GitHub | GitHub Actions | the job's `GITHUB_TOKEN`, with `pull-requests: read` in the workflow's `permissions` |
| GitHub | elsewhere | `GH_TOKEN` or `GITHUB_TOKEN`: a token that can read the repository's pull requests |
| GitLab | GitLab CI | nothing to set: the job token can read merge requests |
| GitLab | elsewhere | `GITLAB_TOKEN`, a token with `read_api` (or `api`) |
| Azure DevOps | Azure Pipelines | `SYSTEM_ACCESSTOKEN`, mapped into the job's environment |
| Azure DevOps | elsewhere | `AZURE_DEVOPS_TOKEN`, a token that can read code |

The same token usually also publishes the release, so a release job rarely needs a second one. All
the variables are listed in [Environment variables](../reference/environment-variables.md).

On GitLab, the handles come from the commit author's email (`ada@example.com` gives `@ada`). For the
real GitLab usernames, set `api_mode: graphql` on the forge and give it a `read_api` token; the job
token can't call GitLab's GraphQL API.

## When the data can't be fetched

By default heraut writes the changelog anyway, without the links, and says so:

```text
✓ [2/2] Generate changelog — without enrichment
     ! remote enrichment unavailable; rendering without PR attribution: … 401 Unauthorized
     ! remote metadata unavailable — PR authors/numbers omitted
```

`commits.enrichment_policy` changes that:

```yaml
commits:
  enrichment_policy: required   # optional (the default) | required | disabled
```

| Policy | When the data can't be fetched |
|---|---|
| `optional` | Writes the changelog without it, with a warning. |
| `required` | Stops the run before anything is written. Use it when a release without its pull request links is not acceptable. |
| `disabled` | Never asks: commit lines only, no API call. |

One flag changes the policy for one run: `--offline` acts as `disabled` (handy on a plane, to look
at a changelog quickly, or to release without PR data while the forge is down).

### When heraut calls the network

`heraut changelog`, `heraut release` and `heraut notes` ask the forge for pull request data whenever
they find one: from `forges:` in the config, the CI job, or an `origin` on github.com, gitlab.com or
Azure DevOps. They ask even when no token is set; the request then fails, with a warning, and the
changelog is written without PR data. Only `--dry-run` and `--offline` (or `enrichment_policy:
disabled`) keep them off the network.

`heraut check runtime` names the forge they will ask, and warns when it has no token:

```text
PR data

! pr data — github acme/app on https://github.com, no token: changelog, release and notes still call its API and go without PR data; set its token, or enrichment_policy: disabled
```

Two other commands reach GitHub for heraut itself: `heraut whatsnew` reads heraut's own release notes
(and falls back to the ones built into the binary when it can't), and a daily check after a command
looks for a newer heraut (see [Upgrade](../install.md#upgrade)).

## Several forges: one source

When you publish to several forges (say GitLab and a GitHub mirror), the pull request data still
comes from one of them, named by `commits.enrichment_forge`. Choose the forge where your pull or merge
requests actually live:

```yaml
forges:
  - name: gitlab
    platform: gitlab
  - name: github
    platform: github
    repository: acme/app

commits:
  enrichment_forge: gitlab
```

Every target's notes then carry the same `!12` links, authors and New Contributors, pointing to
GitLab. Only the commit and compare links follow each target. See
[Publish to GitHub from GitLab CI/CD](github-from-gitlab-ci.md).

## When it fails

| Message | Cause | Fix |
|---|---|---|
| `remote enrichment unavailable; …: 401 Unauthorized` | no token, or it can't read the pull requests | set the token from the table above |
| `remote enrichment (required): …` and the run stops | `enrichment_policy: required` and the fetch failed | fix the token, or `--offline` to release without PR data |
| `You cannot use job tokens to authenticate GraphQL requests` | `api_mode: graphql` with GitLab's job token | `api_mode: rest`, or a `read_api` token in `token_env` |
| no warning, and no links | heraut found no forge at all: not in CI, an origin it doesn't recognize, and no forge token set | add a `forges:` entry |
