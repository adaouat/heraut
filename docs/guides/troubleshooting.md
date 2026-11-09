# Troubleshooting and FAQ

Find your message or question below. Every error also has an exit code, which
[Exit codes](../reference/exit-codes.md) explains; the guide each answer links to covers the details.

## Messages

| You see | Exit | What it means, and what to do |
|---|---|---|
| `no commits since v1.4.0` | 5 | Nothing was committed since the last release. Nothing to do: see [Nothing to release](#nothing-to-release). |
| `no releasable commits since v1.4.0`, with a list | 5 | The new commits don't release (not conventional commits, or set to `none`). See [How the next version is found](../concepts/next-version.md). |
| `no tags found` (from `heraut version current`) | 5 | No release yet. The first `heraut release` creates one. |
| `! no tag matches versioning.tag_prefix "v", but … tags look like versions` | 0 | Your tags have another prefix: set the one the warning names. See [Adopt heraut on an existing project](adopt-on-an-existing-project.md). |
| `unknown key "…" in …` | 2 | A typo in `.heraut.yml`; the message suggests the key you probably meant. `heraut check config` lists every finding. |
| `… must be true or false, not "yes"` | 2 | Boolean keys take `true` or `false` only. |
| `Environment "prod" must be operated from branch "main"` | 2 | Run it from that branch, or pass `--force` to release from here. See [Several environments](environments.md). |
| `changelog "CHANGELOG.md" has no heraut-release anchors` | 3 | The file was written by another tool. See [Adopt heraut on an existing project](adopt-on-an-existing-project.md#2-your-existing-changelog). |
| `the repository is a shallow clone` | 3 | The CI checkout has one commit and no tags: fetch the full history (`fetch-depth: 0` on GitHub, `GIT_DEPTH: "0"` on GitLab). |
| `the repository has no commits yet` | 3 | There is nothing to version. Make a first commit, then run heraut again. |
| `HEAD is detached and no CI branch variable names the branch` | 3 | The job runs on a tag or a merge request, not a branch. Run it in a branch pipeline. |
| `Preflight check failed: git user.name is not configured` | 3 | Set the identity the CI job commits with. See [GitHub Actions](github-actions.md) or [GitLab CI/CD](gitlab-ci.md). |
| `no GitHub token: none of GH_TOKEN, GITHUB_TOKEN is set` | 3 | Export a token that can create releases. See [Pull request data in the changelog](pr-data-and-tokens.md). |
| `error[E001]`, `E002`, `E003` | 4 | A promotion guard between environments. See [Several environments](environments.md). |
| `publishing is off: the config has no release: block` | 0 | `heraut release` tagged without publishing. Add `release: {}` to publish. |

## Nothing to release

`heraut release` on a branch with no new releasable commit exits **5**, so a job that releases on every
push to the main branch would fail after each release commit. Exit 5 isn't a failure; accept it:

```bash
heraut release || [ $? -eq 5 ]
```

In GitHub Actions or GitLab CI, put that line in the job's script. Any other non-zero code still fails
the job.

## Questions

**Does heraut support monorepos, one version per package?** No. heraut reads every commit of the
repository: it doesn't filter them by path, so all packages share one version and one changelog. The
tags can carry a prefix (`tag_prefix`, or `tag_format` per environment), but the commits that count are
the same for all of them.

**Does it run on Windows?** Yes: download the `.exe` ([Install](../install.md)). Hooks run under `cmd`,
not a POSIX shell, so write them for it or call a script.

**Can I preview a release?** `heraut release --dry-run` runs every step and changes nothing;
`--dry-run --json` gives the same plan as data. `heraut version next` prints only the version.

**Does heraut need network access?** Only to read pull-request data and to publish. `--offline` turns
the first off; without a `release:` block nothing is published.

**A release failed halfway. What now?** [Recover from a failed release](failure-recovery.md) covers each
state a run can stop in.
