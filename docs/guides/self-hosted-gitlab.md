# Self-hosted GitLab

heraut works with GitLab on your own server the same way as with gitlab.com. What differs is how it
finds the server: inside your GitLab's CI it finds everything on its own, but on your machine you
tell it where the server is.

The examples use `https://gitlab.example.com` and the project `acme/app`.

## In your GitLab's CI: nothing to add

In a pipeline on your instance, GitLab gives the job the server's address, the project and a token.
heraut reads them, so the minimal configuration is enough:

```yaml
version: "1"

versioning:
  strategy: semver
  tag_prefix: "v"

changelog:
  output: CHANGELOG.md

release: {}
```

The release job itself is the one in [Release from GitLab CI/CD](gitlab-ci.md), including the
setting that lets the job token push.

## On your machine: name the server

Outside CI, heraut only recognises github.com and gitlab.com from your git remote. For your own
server, add a `forges:` entry:

```yaml
forges:
  - name: gitlab
    platform: gitlab
    base_url: https://gitlab.example.com
    project: acme/app
```

Then set a token: a personal access token with the `api` scope, in `GITLAB_TOKEN`.

```bash
export GITLAB_TOKEN=…
heraut check runtime
heraut release --dry-run
```

The same entry also works in CI: on its own instance the job still uses its job token, and
`project` is used as written.

| Key | What to put there |
|---|---|
| `base_url` | Your GitLab's web address. It must be `https`: heraut sends the token there. |
| `project` | The project path, with its groups: `acme/app`, `acme/tools/app`. |
| `api_url` | Rarely needed: the merge request lookups use it when the API isn't at `<base_url>/api/v4`. |
| `token_env` | Only to read the token from another variable than `GITLAB_TOKEN`. |

## Without a `forges:` entry, outside CI

If `GITLAB_TOKEN` is set and nothing else names a forge, heraut assumes gitlab.com. With a
self-hosted origin, a release then pushes the tag to your server and fails when it tries to create
the release on gitlab.com. `heraut release --dry-run` already shows `Publish to gitlab` without your
host, so add the `forges:` entry above whenever you release from your own machine.

## A private certificate authority

If your server's certificate is signed by your company's own authority, heraut needs to trust it
like any other program on the machine: install the authority's certificate in the system store. On
Linux, `SSL_CERT_FILE=/path/to/ca.pem` also works for a single command or CI job.

## When it fails

| Message | Cause | Fix |
|---|---|---|
| `forges[0].base_url: "http://…" must use https` | a plain `http` address | use `https` (plain `http` is accepted only for `localhost`) |
| `heraut release publishes (the config has a release: block) but found no destination` | no `forges:` entry, outside CI, and no token | add the `forges:` entry |
| `environment variable GITLAB_TOKEN is not set` | no token outside CI | set `GITLAB_TOKEN`, or name your variable in `token_env` |
| `x509: certificate signed by unknown authority` | a private certificate authority | trust it, see above |
| `401 Unauthorized` | the token is wrong, expired, or lacks the `api` scope | create a new token with `api` |
