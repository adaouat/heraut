# Environment variables

Every variable heraut reads. You set a few of them yourself: the tokens, and the three `HERAUT_`
settings. The rest come from your CI system, and heraut reads them to find the project, the branch
and a token without any configuration.

## heraut's own settings

| Variable | What it does |
|---|---|
| `HERAUT_FILE` | Path to the configuration file. `--config` wins over it; without either, heraut looks for `.config/heraut.yml`, then `.heraut.yml`. `heraut init` writes to this path too. Handy in CI: `HERAUT_FILE=ci/heraut.yml heraut release`. |
| `HERAUT_SKIP_HOOKS` | Hook points to skip for this run, comma-separated: `HERAUT_SKIP_HOOKS=pre_release,post_release`. The same as `--skip-hook`; when both are given the flag wins, and `--no-hooks` wins over both. |
| `HERAUT_CHECK_UPDATE` | Set to `false` to turn off the one-line "a newer heraut is available" hint printed after a command. |

## Tokens

heraut needs a token to publish a release and to read pull request or merge request data for the
changelog. A `token_env` on a `forges:` entry names your own variable, read instead of the defaults
below. One exception: inside GitLab CI, publishing to the job's own GitLab instance always uses
`CI_JOB_TOKEN`.

| Variable | Forge | Used for |
|---|---|---|
| `GH_TOKEN` | GitHub | Publishing: read first. |
| `GITHUB_TOKEN` | GitHub | Publishing when `GH_TOKEN` is unset, and pull request data. GitHub Actions provides it to the job. |
| `GH_ENTERPRISE_TOKEN` | GitHub Enterprise Server | Publishing: read first on a GitHub Enterprise Server host. |
| `GITHUB_ENTERPRISE_TOKEN` | GitHub Enterprise Server | Publishing when `GH_ENTERPRISE_TOKEN` is unset; `GH_TOKEN` comes after both. |
| `GITLAB_TOKEN` | GitLab | Publishing and merge request data, outside GitLab CI or for another GitLab instance than the job's. |
| `CI_JOB_TOKEN` | GitLab | Publishing and merge request data inside GitLab CI, on the job's own instance. GitLab provides it. |
| `AZURE_DEVOPS_TOKEN` | Azure DevOps | Pull request data, outside Azure Pipelines. |
| `SYSTEM_ACCESSTOKEN` | Azure DevOps | Pull request data inside Azure Pipelines, when the pipeline maps it into the job. |

`heraut check runtime` checks that each release target has a token and that the token can reach
the forge's API.

## Read from your CI system

You don't set these: the CI system does. heraut reads them to find the forge, the project and the
branch when `.heraut.yml` doesn't name them, and to build links in the changelog.

### GitHub Actions

| Variable | Read for |
|---|---|
| `GITHUB_ACTIONS` | Knowing the job runs in GitHub Actions. |
| `GITHUB_SERVER_URL` | The GitHub host, for links and for GitHub Enterprise Server. |
| `GITHUB_API_URL` | The API address. |
| `GITHUB_REPOSITORY` | The repository, `owner/repo`. |
| `GITHUB_REF_NAME` | The branch, when the checkout has no branch of its own (a detached `HEAD`). |
| `GITHUB_REF_TYPE` | Whether `GITHUB_REF_NAME` is a branch; a tag build is not used as a branch. |

### GitLab CI/CD

| Variable | Read for |
|---|---|
| `GITLAB_CI` | Knowing the job runs in GitLab CI. |
| `CI_SERVER_URL` | The GitLab host, for links and to tell whether a target is the job's own instance. |
| `CI_API_V4_URL` | The API address. |
| `CI_PROJECT_PATH` | The project, `group/project`. |
| `CI_PROJECT_URL` | The project's web address, for links. |
| `CI_PROJECT_ID` | The project, when publishing to the job's own project. |
| `CI_COMMIT_BRANCH` | The branch: GitLab checks out the commit, not the branch, and heraut pushes the changelog commit to this one. Tag and merge request pipelines don't set it. |

### Azure Pipelines

| Variable | Read for |
|---|---|
| `TF_BUILD` | Knowing the job runs in Azure Pipelines. |
| `SYSTEM_COLLECTIONURI` | The organization. |
| `SYSTEM_TEAMPROJECT` | The project. |
| `BUILD_REPOSITORY_NAME` | The repository. |
| `BUILD_SOURCEBRANCH` | The branch, when it is a `refs/heads/` ref; a tag build is not used as a branch. |

## Other variables

| Variable | What it does |
|---|---|
| `NO_COLOR` | Any value turns off colors and styling. |
| `PAGER`, `NO_PAGER` | `heraut whatsnew` pages long output through `PAGER` (or `less`) in a terminal; `NO_PAGER` turns paging off. |
