# Architecture Decision Records

ADRs document significant architectural choices: what was decided, why, and what
trade-offs were accepted.

| ADR | Title | Status |
|---|---|---|
| [0001](0001-language-go.md) | Implementation Language — Go | Accepted |
| [0002](0002-tool-name-heraut.md) | Tool Name — Héraut / heraut | Accepted |
| [0003](0003-cli-framework-cobra-fang.md) | CLI Framework — cobra + fang | Accepted |
| [0004](0004-config-format-yaml.md) | Config File Format — YAML | Accepted |
| [0005](0005-config-file-discovery.md) | Config File Discovery — `.config/heraut.yml` and `.heraut.yml` | Accepted |
| [0006](0006-config-naming-generator-platform.md) | Config Naming — generator / platform / release | Accepted |
| [0007](0007-version-promotion-error-handling.md) | Version Promotion Error Handling (E001 / E002 / E003) | Accepted |
| [0008](0008-promote-source-env.md) | Explicit `source` Field for `bump: promote` Environments | Accepted |
| [0009](0009-generic-perenv-resolver.md) | Generic Per-Env Resolver Design (`perenv` + `tagfmt`) | Accepted |
| [0010](0010-embedded-cliff-toml-default.md) | Embedded `cliff.toml` Default with Optional User Override | Accepted |
| [0011](0011-single-pipeline-release-via-pre-computation.md) | Single-Pipeline Release via Version Pre-computation | Accepted |
| [0012](0012-changelog-commit-ownership.md) | Changelog Commit Ownership and Release Workflow Order | Accepted |
| [0013](0013-raw-binary-goreleaser-format.md) | Raw Binary GoReleaser Format (No Archives) | Accepted |
| [0014](0014-self-update-architecture.md) | Self-Update Architecture | Superseded (forge ADR-0005) |
| [0015](0015-unified-logging-charm-log.md) | Unified Logging with `charm.land/log` | Rejected |
| [0016](0016-bundled-docker-image.md) | Batteries-Included Docker Image | Accepted |
| [0017](0017-pipeline-progress-reporter.md) | Pipeline Progress Reporter Pattern | Accepted |
| [0018](0018-ci-build-then-release-pipeline.md) | Self-Bootstrapping CI Release Pipeline | Accepted |
| [0019](0019-perenv-content-driver-merge.md) | Per-Environment Content-Driver Overrides Deep-Merge | Accepted |
| [0020](0020-platform-base-url.md) | Per-Platform `base_url` for Self-Hosted Instances | Superseded by ADR-0025 |
| [0021](0021-per-platform-release-notes.md) | Release Notes Regenerated Per Platform | Accepted |
| [0022](0022-fat-injection-thin-templates.md) | Fat Injection / Thin git-cliff Templates | Accepted |
| [0023](0023-remote-metadata-policy.md) | Remote-Metadata Policy for Content Generation | Accepted |
| [0024](0024-ticket-linking.md) | Ticket Linking via git-cliff link_parsers | Accepted |
| [0025](0025-multi-instance-platforms.md) | Multi-Instance Same-Platform Releases | Accepted |
| [0026](0026-azure-devops-metadata-remote.md) | `changelog.remote` — Explicit Metadata Remote for git-cliff | Accepted |
| [0027](0027-builtin-conventional-commit-checker.md) | Built-in Conventional-Commit Checker | Accepted (`commit_lint` superseded by 0033) |
| [0028](0028-drop-cocogitto-generator.md) | Drop the `cocogitto` Generator Entirely | Accepted |
| [0029](0029-pkl-builtin-commit-verify.md) | Publish a Pkl Builtin for `heraut commit verify` | Accepted |
| [0030](0030-commit-check-rev-range-validation.md) | `heraut commit check` — Rev-Range Conventional-Commit Validation | Accepted |
| [0031](0031-interactive-commit-wizard.md) | `heraut commit create` — Interactive Commit Wizard | Accepted |
| [0032](0032-native-content-generator.md) | Built-in (Native) Content Generator | Accepted (config model + opt-in framing superseded by 0033) |
| [0033](0033-native-config-model.md) | Heraut-Native Config Model — `commits` + `rendering` | Accepted |
