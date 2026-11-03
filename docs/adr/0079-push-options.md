# ADR-0079: Push options for the release pushes — `git.push_options`

- **Status**: Accepted
- **Date**: 2026-11-03
- **Deciders**: bchatard

---

## Context

heraut pushes twice when it commits and tags: `git push origin HEAD` for the changelog commit, then
`git push origin <tag>` (`internal/pipeline/git.go`). Both pushes run in `heraut release` and in
`heraut changelog --commit/--tag`, unless `--no-push` is passed. In CI, the commit push starts a
new branch pipeline for the `chore(release)` commit, which has nothing to do: the next heraut run
stops on "no commits since". Projects want to skip that pipeline and keep the tag's.

The tool heraut offers today is `versioning.commit_message`. With it, `[skip ci]` can go in the
commit message. On GitLab that skips too much: the tag points at that same commit, so its pipeline
is skipped as well, and the tag pipeline is usually the one that builds or deploys.

git has a per-push channel for this: `git push --push-option=<value>` (`-o`). The server reads the
value, and git does nothing else with it. GitLab reads `ci.skip`, `ci.variable="KEY=VALUE"` and
`integrations.skip_ci`. Since heraut makes two separate pushes, it can skip the commit's pipeline
and keep the tag's.

### Options

1. **Free-form git arguments** (`git.push_args`, `git.commit_args`). Rejected. Config could then
   inject `--force`, `--no-verify`, `--mirror` or `--delete` into a release. heraut's own
   `--no-verify` ban would not hold, and what a run does could no longer be read from its config.
2. **One list for both pushes.** Rejected. `ci.skip` on both pushes is the GitLab trap above, so
   the two pushes need separate lists.
3. **Push options only, one list per push, with a per-environment override.** Chosen.

## Decision

**A new top-level `git` block, with `push_options.commit` and `push_options.tag`, each a list of
strings passed to its push as `--push-option=<value>`, in order.**

```yaml
git:
  push_options:
    commit: ["ci.skip"]   # git push --push-option=ci.skip origin HEAD
    tag: []               # git push origin <tag>
```

- **Scope.** The commit list applies to `git push origin HEAD` and the tag list to
  `git push origin <tag>`. This holds wherever heraut pushes: `heraut release`, and
  `heraut changelog --commit/--tag`. With `--no-push`, nothing is pushed and neither list is used.
  The fetches, the tag lookups and the forge API calls are unchanged.
- **The argument form.** Each value goes as one `--push-option=<value>` argument, so a value
  starting with `-` is never read as a flag.
- **Validation**, run by `heraut check config` and before any run:
  - **Empty values and line breaks are config errors (exit 2).** git rejects push options that
    contain a newline, and an empty one is surely a mistake. The check catches them before
    anything is committed.
  - **A NUL byte is also a config error.**
  - **Any other value is passed through unchecked.** Push options are defined by the server,
    and heraut has no list of valid values.
- **Per environment.** `environments.<name>.git.push_options.commit` and `.tag` override the root,
  each independently, under the rule the lists already follow (Spec 02 § Content override
  semantics):
  - **Absent:** the environment inherits the root list.
  - **Present:** the environment's list replaces the root's.
  - **Present and empty (`[]`):** the environment pushes with no options, which is how a
    root-wide `ci.skip` is turned off for one environment.

  Like every per-environment field, this is only valid with `semver-per-env` and
  `calver-per-env`.
- **Dry run and plan.** The `[dry-run]` lines for a push name its options. In the `--json` plan,
  the `commit_changelog` and `push_tag` steps gain a `push_options` array, omitted when empty.
  The field is additive, so the envelope's existing fields are unchanged.
- **No CLI flag.** A value that has to change per run can be set through a per-environment
  override, or in the config the CI job selects (`HERAUT_FILE`). A repeatable `--push-option`
  flag can be added later if a real case needs one.

### Servers that do not support push options

git refuses a push that carries push options when the remote does not advertise the capability
("the receiving end does not support push options"), and nothing is pushed. heraut wraps git's
error as it does for any failed push, so the run stops at that push and leaves the repository in
a state the failure-recovery guide covers. GitLab supports push options. Whether github.com
accepts them, and ignores them, has not been verified. A Lane B run against both sandboxes
settles it before the docs say anything about GitHub. On GitHub the usual way to skip CI is
`[skip ci]` in `versioning.commit_message`. Whether that also skips a tag-push workflow there is
checked in the same run.

## Consequences

- GitLab projects can skip the `chore(release)` commit's pipeline and keep the tag's, which
  `commit_message` cannot do.
- heraut gains a top-level config block (`git`). `config.go`, `schema.json` and
  `docs/heraut.sample.yml` gain it together, with valid and invalid fixtures.
- An unsupported push option fails the push, after the commit exists locally (commit push) or
  after the tag exists locally (tag push). The failure-recovery guide's states A and B already
  cover both. The guide gains a line saying that removing the option and retrying is the fix.
- `commit_message` keeps working as before, including `[skip ci]`. The two can be combined.
- Not covered: options for `git commit` or `git tag` (none has a use yet), and a preflight check
  of the remote's capability. `heraut check runtime` could probe it later with
  `git ls-remote`, but an advertised capability says nothing about which values the server
  honours.
