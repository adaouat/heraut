# Mid-flight failure / dirty state

## Problem

`heraut release` is not atomic. If it fails partway through, the repo can be left in a partially-committed state. Failure scenarios by step:

1. **After changelog commit, before tag** — changelog is committed on main, no tag exists. Re-running `heraut release` will try to generate a changelog again (duplicate entry risk) or fail on "nothing to release".
2. **After tag, before push** — local tag exists, remote doesn't. Re-run may try to create a duplicate local tag.
3. **After push, before platform publish** — tag is on remote, GitHub/GitLab release doesn't exist. Re-run may try to re-tag (fails) or skip tag and go straight to publish.
4. **After platform publish, before release notes** — partial GitHub release with no notes.

## Questions to answer

- Should heraut detect and resume a partial release (idempotency / state file)?
- Should heraut be transactional — roll back the changelog commit and tag if publish fails?
- Should heraut be restartable — detect "tag already exists, skip to publish"?
- Is the changelog commit + tag the point of no return? (Can't undo a pushed tag safely)

## Related

- `--force` flag exists for some operations but is not a full recovery mechanism
- `heraut release --dry-run` doesn't help post-failure since state is already dirty

## Priority

Tackle after the CI build-then-release discussion resolves the release pipeline shape.
