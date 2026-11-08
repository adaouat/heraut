# Check commit messages

heraut picks the next version and writes the changelog from your commit messages, so they need to
follow [Conventional Commits](https://www.conventionalcommits.org/): `type(scope): description`,
such as `feat(api): add search` or `fix: crash on empty query`. heraut can check them at three
moments: when you commit, on every pull request, and while you write one.

## When you commit: a `commit-msg` hook

`heraut commit verify --file <path>` checks one message, prints what it understood, and exits 1 when
the message is invalid. Run it from your repository's `commit-msg` hook.

**Plain git**: `.git/hooks/commit-msg`, made executable (`chmod +x`):

```sh
#!/bin/sh
exec heraut commit verify --file "$1"
```

**lefthook**: in `lefthook.yml`:

```yaml
commit-msg:
  commands:
    heraut:
      run: heraut commit verify --file {1}
```

**hk**: in `hk.pkl`:

```pkl
hooks {
  ["commit-msg"] {
    steps {
      ["heraut"] {
        check = "heraut commit verify --file {{ commit_msg_file }}"
      }
    }
  }
}
```

A refused commit says why:

```text
invalid conventional commit header "added search": expected "type(scope)!: description"
```

Merge commits and `fixup!` / `squash!` commits are always accepted.

## On every pull request: `heraut commit check`

`heraut commit check` checks every commit in a range and lists the invalid ones, so a pull request
with a bad message fails before it's merged:

```console
$ heraut commit check origin/main..HEAD
✗ 0ac7be1  Update stuff — validating commit message: invalid conventional commit header "Update stuff": expected "type(scope)!: description"
1 of 3 commits invalid
```

**GitHub Actions**:

```yaml
on: pull_request

jobs:
  commits:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
        with:
          fetch-depth: 0
      # install heraut as in the release workflow
      - run: heraut commit check "origin/${{ github.base_ref }}..HEAD"
```

**GitLab CI/CD**:

```yaml
commits:
  image:
    name: ghcr.io/adaouat/heraut:1.2.3
    entrypoint: [""]
  rules:
    - if: $CI_PIPELINE_SOURCE == "merge_request_event"
  variables:
    GIT_DEPTH: "0"
  script:
    - heraut commit check "$CI_MERGE_REQUEST_DIFF_BASE_SHA..HEAD"
```

Without a range it checks the whole history; `--from-latest-tag` checks what the next release
will contain.

## While you write: `heraut commit create`

`heraut commit create` asks for the type, scope, description, body, breaking change and tickets,
then runs `git commit`. `--all` stages tracked changes first, like `git commit -a`. It needs an
interactive terminal.

## Your types, scopes and rules

The allowed types are the usual ones: `feat`, `fix`, `docs`, `chore`, `refactor`, `test`, `style`,
`perf`, `ci`, `build`, `security` and `revert`. In `.heraut.yml`, `commits` adjusts them, and adds checks of
your own:

```yaml
commits:
  types:
    - name: deps                  # a new type, with its own changelog section
      render: "📦 Dependencies"
    - name: style
      remove: true                # no longer allowed
  scopes:
    - name: api
    - name: ui
  scopes_restricted: true         # refuse any other scope
  rules:
    - name: no-wip
      deny: "(?i)\\bwip\\b"
      message: commit message must not contain WIP
```

| Setting | Effect |
|---|---|
| `types` | Added to the built-in list, by name. To narrow the list, `remove:` types; listing some doesn't drop the others. |
| `scopes` + `scopes_restricted: true` | Refuses other scopes: `Commit scope "db" is not allowed (allowed: deps, deps-dev, release, api, ui).` Without `scopes_restricted`, scopes are suggestions in `commit create`. |
| `rules` | A regular expression a message must not match (`deny`) or must match (`require`), or a required ticket (see [Link tickets](tickets.md)). Each can be limited to some types or scopes. |

`deps`, `deps-dev` and `release` are always among the scopes, for dependency bots and heraut's own
release commit.
