# Architecture Decision Records

ADRs document significant architectural choices: what was decided, why, and what
trade-offs were accepted.

| ADR | Title | Status |
|---|---|---|
| [0001](0001-language-go.md) | Implementation Language — Go | Accepted |
| [0002](0002-tool-name-heraut.md) | Tool Name — Héraut / heraut | Accepted |
| [0003](0003-cli-framework-cobra-fang.md) | CLI Framework — cobra + fang | Accepted (framework choice holds; entry point now routes through `github.com/adaouat/forge/cli`, which wraps cobra+fang internally — see body note) |
| [0004](0004-config-format-yaml.md) | Config File Format — YAML | Accepted |
| [0005](0005-config-file-discovery.md) | Config File Discovery — `.config/heraut.yml` and `.heraut.yml` | Accepted |
| [0006](0006-config-naming-generator-platform.md) | Config Naming — generator / platform / release | Accepted (naming superseded by 0043 — `release.platforms`→`release.targets`+`forges:`, `generator: git-cliff`→native-only) |
| [0007](0007-version-promotion-error-handling.md) | Version Promotion Error Handling (E001 / E002 / E003) | Accepted |
| [0008](0008-promote-source-env.md) | Explicit `source` Field for `bump: promote` Environments | Accepted |
| [0009](0009-generic-perenv-resolver.md) | Generic Per-Env Resolver Design (`perenv` + `tagfmt`) | Accepted |
| [0010](0010-embedded-cliff-toml-default.md) | Embedded `cliff.toml` Default with Optional User Override | Superseded by ADR-0045 |
| [0011](0011-single-pipeline-release-via-pre-computation.md) | Single-Pipeline Release via Version Pre-computation | Accepted (pre-computation decision holds; the sketched step order is superseded by 0021's actual tag→push→notes→publish sequence — see body note) |
| [0012](0012-changelog-commit-ownership.md) | Changelog Commit Ownership and Release Workflow Order | Accepted (commit-ownership decision holds; see 0011's note on step order) |
| [0013](0013-raw-binary-goreleaser-format.md) | Raw Binary GoReleaser Format (No Archives) | Accepted |
| [0014](0014-self-update-architecture.md) | Self-Update Architecture | Superseded by forge ADR-0005 |
| [0015](0015-unified-logging-charm-log.md) | Unified Logging with `charm.land/log` | Rejected |
| [0016](0016-bundled-docker-image.md) | Bundled Docker Image (Full Release Runner) | Accepted |
| [0017](0017-pipeline-progress-reporter.md) | Pipeline Progress Reporter Pattern | Accepted |
| [0018](0018-ci-build-then-release-pipeline.md) | Self-Bootstrapping CI Release Pipeline | Accepted |
| [0019](0019-perenv-content-driver-merge.md) | Per-Environment Content-Driver Overrides Deep-Merge | Accepted (merge mechanism holds; the generator-switch-triggers-full-replacement branch never existed, and the `release.platforms`/git-cliff example is superseded by 0044/0045) |
| [0020](0020-platform-base-url.md) | Per-Platform `base_url` for Self-Hosted Instances | Superseded by ADR-0025 |
| [0021](0021-per-platform-release-notes.md) | Release Notes Regenerated Per Platform | Accepted (per-platform regeneration still holds; §Context-injection shape's cocogitto/communique/git-cliff adapter examples are gone — native is the sole generator, 0045) |
| [0022](0022-fat-injection-thin-templates.md) | Fat Injection / Thin git-cliff Templates | Superseded by ADR-0045 (git-cliff templates removed; `internal/generators/gitcliff/` deleted) |
| [0023](0023-remote-metadata-policy.md) | Remote-Metadata Policy for Content Generation | Accepted (`remote_metadata` key renamed to `commits.enrichment_policy` by 0043) |
| [0024](0024-ticket-linking.md) | Ticket Linking via git-cliff link_parsers | Superseded by ADR-0033 (`tickets` moved to `commits.tickets`; git-cliff-only gate unreachable since 0045 made native the sole generator) |
| [0025](0025-multi-instance-platforms.md) | Multi-Instance Same-Platform Releases | Accepted (config surface — `release.platforms` list with unique `name`s — superseded by multiple `forges:` entries + `release.targets` by 0044) |
| [0026](0026-azure-devops-metadata-remote.md) | `changelog.remote` — Explicit Metadata Remote for git-cliff | Accepted (git-cliff-only gate + `api_url` superseded by 0040; `changelog.remote` block removed and replaced by `forges:` + `commits.enrichment_forge` by 0043) |
| [0027](0027-builtin-conventional-commit-checker.md) | Built-in Conventional-Commit Checker | Accepted (`commit_lint` superseded by 0033) |
| [0028](0028-drop-cocogitto-generator.md) | Drop the `cocogitto` Generator Entirely | Accepted |
| [0029](0029-pkl-builtin-commit-verify.md) | Publish a Pkl Builtin for `heraut commit verify` | Accepted |
| [0030](0030-commit-check-rev-range-validation.md) | `heraut commit check` — Rev-Range Conventional-Commit Validation | Accepted |
| [0031](0031-interactive-commit-wizard.md) | `heraut commit create` — Interactive Commit Wizard | Accepted (`commit_lint.*` fields renamed to `commits.*` by 0033) |
| [0032](0032-native-content-generator.md) | Built-in (Native) Content Generator | Accepted (config model + opt-in framing superseded by 0033; "additive, not a replacement" framing superseded by 0045 — native is now the sole generator) |
| [0033](0033-native-config-model.md) | Heraut-Native Config Model — Unified `commits` + `rendering` | Accepted (the `commits.remote_metadata` field it introduced was itself renamed to `commits.enrichment_policy` by 0043; "git-cliff stays functional but unreconciled" ended with 0045) |
| [0034](0034-native-remote-enrichment.md) | Native Remote Enrichment (Phase 2) | Accepted (§3 Azure transport superseded by 0035; first-timer mechanism superseded by 0036; §1 `gh api`/`glab api` transport superseded by 0043's P2 — all enrichment is native `net/http` now) |
| [0035](0035-azure-enrichment-native-http.md) | Azure DevOps Enrichment via a Native `net/http` Client (not the `az` CLI) | Accepted (`changelog.remote.api_url` reference superseded by 0040; implementation now lives in `internal/forge/azure/`, not `internal/generators/native/enrich_azure.go`) |
| [0036](0036-unified-enrichment-model.md) | Unified Cross-Platform Enrichment Model | Accepted |
| [0037](0037-native-template-api.md) | Native Generator Public Template API | Accepted |
| [0038](0038-incremental-changelog.md) | Incremental Changelog Generation (native) | Accepted (GitLab full-regeneration rate-limit warning superseded by 0042) |
| [0039](0039-commit-author-attribution.md) | Commit-Author Attribution (native) | Accepted (GitHub-only scope superseded — GitLab by 0042, Azure by T151; `prFragment` moved from `internal/generators/native/enrich_github.go` to `internal/forge/github/graphql.go` by 0043) |
| [0040](0040-changelog-remote-native-base-url.md) | `changelog.remote` for native + unified `base_url` host override | Superseded by ADR-0043 (`changelog.remote` removed entirely; replaced by `forges:` + `commits.enrichment_forge`) |
| [0041](0041-remote-metadata-required-enforcement-and-force.md) | `remote_metadata: required` enforcement and `--force` override | Accepted (`commits.remote_metadata` renamed to `commits.enrichment_policy` by 0043; enforcement/`--force` semantics unchanged) |
| [0042](0042-gitlab-graphql-enrichment.md) | GitLab enrichment via batched GraphQL | Accepted (transport ported from `glab api graphql` to native `net/http` by 0043 P1; the GraphQL query shapes and behavior described here are otherwise still current) |
| [0043](0043-forge-abstraction.md) | Forge abstraction + unified forges: config | Accepted (P3 framing — publishing folds into `port.Forge` with a transport change — superseded by 0044, which keeps `gh`/`glab` and unifies config only; "GitHub and Azure keep their current transports" now stale — GitHub's P2 migration to native `net/http` shipped) |
| [0044](0044-publishing-config-unification.md) | Publishing config unification — `release.targets` replaces `release.platforms` | Accepted (decision holds; the migration example's `release.notes.generator` key is itself removed by 0045 — see body note) |
| [0045](0045-native-sole-generator.md) | Native as Heraut's Sole Content Generator | Accepted |
| [0046](0046-release-block-atomicity.md) | Release block is one intent, not two | Accepted |
