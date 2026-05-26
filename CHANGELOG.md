# Changelog

## [unreleased]

### 🚀 Features

- *(pipeline)* Configurable git tag type — annotated (default) or lightweight - ([b3098d2](https://github.com/adaouat/heraut/commit/b3098d2ab58b608546cc9e5b96d8af55c01c7847))


### 📚 Documentation

- *(roadmap)* Add Phase 8 — Stable Release Preparation (T28/T33/T34/T35) - ([349b43b](https://github.com/adaouat/heraut/commit/349b43b686dee2e4729a60d725abe13a23599595))


### 🧪 Testing

- *(coverage)* Coverage sweep T34 — raise CI gate to 80% - ([54e3428](https://github.com/adaouat/heraut/commit/54e3428096d3da9ad8e75e10893c642012802a83))


### ⚙️ Miscellaneous Tasks

- Split CI into parallel lint/test/build jobs with coverage gate - ([6a0c831](https://github.com/adaouat/heraut/commit/6a0c831e7edf250dbf69a529ccf584ccb3cb0497))

## [0.8.0](https://github.com/adaouat/heraut/compare/v0.7.1..v0.8.0) - 2026-05-25

### 🚀 Features

- *(adapter/exec)* Verbose output echo + stderr on failure (T29) - ([717a81e](https://github.com/adaouat/heraut/commit/717a81e27a2173fa8f2d320ef9d7f3dfabb430a1)) by @bchatard

- *(ci)* Bundled Docker image with all external CLIs (T31) - ([3b72987](https://github.com/adaouat/heraut/commit/3b729876afaffc30d7e737c2f2887eb8a2b32a21)) by @bchatard

- *(cmd)* Map errors to structured exit codes (T27) - ([4d7ff0e](https://github.com/adaouat/heraut/commit/4d7ff0e0487940cb2147a66d13f50dbcfab78b3c)) by @bchatard

- *(ui)* Success/Err/Warn/Info status-line helpers + check.go wiring (T32) - ([28e3f1a](https://github.com/adaouat/heraut/commit/28e3f1aa08578e34695911bf25807aa4596130c6)) by @bchatard

- *(versioning/perenv)* Rich Biome-style promotion error messages (T30) - ([0a0fd7f](https://github.com/adaouat/heraut/commit/0a0fd7f9ce1711b5525044a5051ac842ad4e578d)) by @bchatard


### 🐛 Bug Fixes

- *(versioning/perenv)* Fall back to top-level tag_format when env has none - ([f32be48](https://github.com/adaouat/heraut/commit/f32be48df5314b413b17967848bea2a4070c8b70)) by @bchatard

- *(versioning/semver)* Add actionable hint to no-commits error - ([842f3e9](https://github.com/adaouat/heraut/commit/842f3e9b12a0b2c5de9121f458e39e484f6a4f0a)) by @bchatard


### 💼 Other

- Promote indirect deps to direct after go mod tidy - ([ab42ef9](https://github.com/adaouat/heraut/commit/ab42ef9c9f17653b40941ab92bf6d045d854d152)) by @bchatard


### 🚜 Refactor

- *(cmd)* Migrate remaining status lines to ui helpers - ([16a3bbd](https://github.com/adaouat/heraut/commit/16a3bbd2c59bc341c383939ebe8e45a17d397100)) by @bchatard


### 📚 Documentation

- *(adr)* Add ADR-0015 unified logging (charm.land/log), Proposed - ([4eea422](https://github.com/adaouat/heraut/commit/4eea4227d50d26c598826b6858870192368b70f5)) by @bchatard

- *(roadmap)* Restore T30 heading dropped in T29 commit - ([917aa34](https://github.com/adaouat/heraut/commit/917aa343fd7b67223f753328ba16ab91bbcf595c)) by @bchatard

- *(roadmap)* Complete checkpoint H; fix ADR count references - ([a164542](https://github.com/adaouat/heraut/commit/a16454253a9bc162bfe5e590c18c544c0bd9b3fa)) by @bchatard

## [0.7.1](https://github.com/adaouat/heraut/compare/v0.7.0..v0.7.1) - 2026-05-25

### 📚 Documentation

- *(adr)* Reconcile ADRs with implementation (T23) - ([ef176f7](https://github.com/adaouat/heraut/commit/ef176f798fee26f1a240508a9013702a384979b0)) by @bchatard

- *(roadmap)* Add T31 bundled Docker image task - ([eaa9335](https://github.com/adaouat/heraut/commit/eaa9335ed625d017d12988b291e0d013dc274348)) by @bchatard

- *(specs)* Reconcile specs with implementation (T22) - ([83400ee](https://github.com/adaouat/heraut/commit/83400ee6192141bf485a0928d3e0883b5e831a55)) by @bchatard

- Add public README (T24) - ([0d6553a](https://github.com/adaouat/heraut/commit/0d6553ae8319858cfcb8e0d4a59b058edce38b0f)) by @bchatard

## [0.7.0](https://github.com/adaouat/heraut/compare/v0.6.0..v0.7.0) - 2026-05-25

### 🚀 Features

- *(scaffold)* Add heraut init wizard (T20) - ([6ca3ae0](https://github.com/adaouat/heraut/commit/6ca3ae07bd194e03f0979e121419631565b16763)) by @bchatard

- *(scaffold)* Prompt for sprint number when CalVer format uses SPRINT - ([11fb6c2](https://github.com/adaouat/heraut/commit/11fb6c2199989c7fb1dc37502d3108ca4b6ac4eb)) by @bchatard

- *(scaffold)* Versioned schema.json URL per heraut release (T26) - ([566381a](https://github.com/adaouat/heraut/commit/566381afd20c8d6fa64be1c86624330c14f2a269)) by @bchatard

- *(schema)* Add JSON Schema draft-07 for .heraut.yml (T25) - ([cad3f03](https://github.com/adaouat/heraut/commit/cad3f03a8466aa701835024ecdc7e9e141c733ab)) by @bchatard

- *(selfupdate)* Add heraut self-update command (T21) - ([01d1417](https://github.com/adaouat/heraut/commit/01d1417e0309a4efbc98fdc3bc9d09046ca23ca7)) by @bchatard


### 🐛 Bug Fixes

- *(cmd)* Init writes to InitDest(), not ResolvePath() - ([25d11ed](https://github.com/adaouat/heraut/commit/25d11ed126b8f5e1850122030f26249018987c9e)) by @bchatard

- *(selfupdate)* Friendly error when no releases published yet - ([24acc89](https://github.com/adaouat/heraut/commit/24acc8985cd93f3ad0ada620510caf7e6ed6ae34)) by @bchatard


### 📚 Documentation

- *(roadmap)* Mark T20 complete - ([d388b65](https://github.com/adaouat/heraut/commit/d388b6527e7659924c2e9869c29dc5bd8006e220)) by @bchatard

- *(roadmap)* Mark CHECKPOINT G complete - ([8722908](https://github.com/adaouat/heraut/commit/8722908caf8048329b8a0ad7fb11d229af51cce2)) by @bchatard

## [0.6.0](https://github.com/adaouat/heraut/compare/v0.5.4..v0.6.0) - 2026-05-24

### 🚀 Features

- *(app)* Add BuildChangelogPipeline and wire all generators and platforms (T17) - ([28b8449](https://github.com/adaouat/heraut/commit/28b84498b0a41826256a7019b77c218c11e53735)) by @bchatard

- *(app)* Add PreflightCheck, RuntimeCheck, CheckCliff; add CheckCliff to gitcliff (T18) - ([0141811](https://github.com/adaouat/heraut/commit/014181184b2c0b1200176bfab9f0955166d0b872)) by @bchatard

- *(cmd)* Add heraut changelog subcommand with --commit, --tag, --version flags (T17) - ([c60d01a](https://github.com/adaouat/heraut/commit/c60d01a5945b73b07134f2601a1605127cd68b9a)) by @bchatard

- *(cmd)* Add heraut check subcommands (config, runtime, cliff) (T18) - ([be77515](https://github.com/adaouat/heraut/commit/be7751535e4bb15a628bf1d7a8975cba75067793)) by @bchatard

- *(cmd)* Add automatic preflight check to heraut release and changelog (T18) - ([88ec108](https://github.com/adaouat/heraut/commit/88ec108a82cae08d2c4a87c34c6483c3d7009719)) by @bchatard

- *(cmd)* Add heraut cliff changelog/release-notes; wire DisableNotes per-env (T19) - ([59067e4](https://github.com/adaouat/heraut/commit/59067e4dd21eeec72772fcbf3621bab594a32b56)) by @bchatard

- *(pipeline)* Add ChangelogPipeline for changelog-only flow (T17) - ([7166158](https://github.com/adaouat/heraut/commit/716615880dbcaa5952b6dad2c9f7a38a99bf7311)) by @bchatard

- *(pipeline)* Add DisableNotes to Config and honour it in release Run (T19) - ([77b1a32](https://github.com/adaouat/heraut/commit/77b1a32da8a1fd04ad9a302fd818174a2ea60651)) by @bchatard


### 🐛 Bug Fixes

- *(cmd)* Skip preflight and pipe.Check in dry-run; mark CHECKPOINT F complete - ([90e4f75](https://github.com/adaouat/heraut/commit/90e4f75acf4aba68b109fe30bfa379ae39a0455e)) by @bchatard


### 💼 Other

- *(deps)* Promote yaml.v3 and go-toml/v2 to direct dependencies - ([90e32b8](https://github.com/adaouat/heraut/commit/90e32b8a0d2232541ecdaa6dea10cbec5dfdda81)) by @bchatard


### 📚 Documentation

- *(roadmap)* Mark T17 complete with implementation notes - ([77517f1](https://github.com/adaouat/heraut/commit/77517f19cc3455349ae6de574d6470421617a9e1)) by @bchatard

## [0.5.4](https://github.com/adaouat/heraut/compare/v0.5.3..v0.5.4) - 2026-05-24

### ⚙️ Miscellaneous Tasks

- *(goreleaser)* Add Dockerfile.goreleaser for release image builds - ([5918db3](https://github.com/adaouat/heraut/commit/5918db3c40ba8e48174953943b30894480f8bf3a)) by @bchatard

## [0.5.3](https://github.com/adaouat/heraut/compare/v0.5.2..v0.5.3) - 2026-05-24

### ⚙️ Miscellaneous Tasks

- *(goreleaser)* Remove stray #magic___^_^___line from ldflags - ([659ff9a](https://github.com/adaouat/heraut/commit/659ff9a0bae962558e7ceaa213805ae78ccbedef)) by @bchatard

## [0.5.2](https://github.com/adaouat/heraut/compare/v0.5.1..v0.5.2) - 2026-05-24

### ⚙️ Miscellaneous Tasks

- *(release)* Add pull-requests: read permission for git-cliff PR enrichment - ([2556b0b](https://github.com/adaouat/heraut/commit/2556b0b8197ab96b23e60690028b4fb9371f971b)) by @bchatard

## [0.5.1](https://github.com/adaouat/heraut/compare/v0.5.0..v0.5.1) - 2026-05-24

### 🧪 Testing

- *(platforms/github)* Clear GITHUB_REPOSITORY in TestCheck_RepositoryMissing - ([0fa6a58](https://github.com/adaouat/heraut/commit/0fa6a589a784500513188ef24305060b4e200590)) by @bchatard


### ⚙️ Miscellaneous Tasks

- *(release)* Use orhun/git-cliff-action for release notes generation - ([2033c3f](https://github.com/adaouat/heraut/commit/2033c3f211934791120e6ac35381df12c680aefa)) by @bchatard

## [0.5.0](https://github.com/adaouat/heraut/compare/v0.4.0..v0.5.0) - 2026-05-24

### 🚀 Features

- *(generators/cocogitto)* Add cocogitto generator with contract tests (T15) - ([767f631](https://github.com/adaouat/heraut/commit/767f631635e4d5704cab9ef9d651050a15944e5e)) by @bchatard

- *(generators/communique)* Add communique generator with contract tests (T14) - ([e5642f8](https://github.com/adaouat/heraut/commit/e5642f8215e86f646710d2e596334aade5e04af3)) by @bchatard

- *(platforms/gitlab)* Add GitLab platform with contract tests (T16) - ([a46faff](https://github.com/adaouat/heraut/commit/a46faff21c7b928e2c0c4b4eb01029883abef48e)) by @bchatard

## [0.4.0](https://github.com/adaouat/heraut/compare/v0.3.0..v0.4.0) - 2026-05-24

### 🚀 Features

- *(cmd)* Add heraut version next/current/sprint-bump subcommands (T13) - ([000be52](https://github.com/adaouat/heraut/commit/000be5288d05d6707ca6df3699a941767bebda3d)) by @bchatard

- *(versioning/calver)* Add CalVer resolver with injectable clock (T11) - ([ab2c62f](https://github.com/adaouat/heraut/commit/ab2c62f32cf9fddd9b3a70cd5c2b58d11cf5ae89)) by @bchatard

- *(versioning/perenv)* Add generic per-env resolver with E001/E002/E003 guards (T12) - ([fe9fee5](https://github.com/adaouat/heraut/commit/fe9fee587f5bc27c70d5dfabae2cdc9fdad48084)) by @bchatard

## [0.3.0](https://github.com/adaouat/heraut/compare/v0.2.0..v0.3.0) - 2026-05-24

### 🚀 Features

- *(cmd)* Wire release pipeline end-to-end for semver + GitHub (T10) - ([392fd50](https://github.com/adaouat/heraut/commit/392fd50fc22fa0471dcef2656e6e2bb6d0434dac)) by @bchatard

- *(generators/gitcliff)* Add gitcliff generator with embedded TOML defaults (T08) - ([5a72bfb](https://github.com/adaouat/heraut/commit/5a72bfbf59e2a398ad04addfcfe68d798fa1d93a)) by @bchatard

- *(platforms/github)* Add GitHub platform with contract tests (T09) - ([419f039](https://github.com/adaouat/heraut/commit/419f03947d516e24359ba5fc1f79bf3a441bf00a)) by @bchatard

- *(versioning/semver)* Add SemVer resolver, bump logic, and Result type (T07) - ([0ff2c84](https://github.com/adaouat/heraut/commit/0ff2c84a6b39610242675e1ac09926f970a6e2b4)) by @bchatard

- *(versioning/tagfmt)* Add shared tag format package (T06) - ([5791836](https://github.com/adaouat/heraut/commit/5791836d2d393e41d2895a126142ddb653cea721)) by @bchatard

## [0.2.0](https://github.com/adaouat/heraut/compare/v0.1.0..v0.2.0) - 2026-05-24

### 🚀 Features

- *(config)* Add config structs, strict loader, and path resolution (T04) - ([c9ad804](https://github.com/adaouat/heraut/commit/c9ad804b83e315d42ff0eb58469b927620a8e251)) by @bchatard

- *(config)* Add semantic validator with cycle detection (T05) - ([e30036d](https://github.com/adaouat/heraut/commit/e30036d3cc114ef32c614998fdf4a32a381908d7)) by @bchatard

- *(port)* Add port interfaces, exec adapter, and testutil (T03) - ([b7d4714](https://github.com/adaouat/heraut/commit/b7d4714d45d6acea851d247cf36868d969aa6624)) by @bchatard

## [0.1.0](https://github.com/adaouat/heraut/compare/v0.0.0..v0.1.0) - 2026-05-24

### 🚀 Features

- *(cmd)* Bootstrap Go module and cobra+fang root command (T00) - ([e597bb6](https://github.com/adaouat/heraut/commit/e597bb63a0dfcba4702e3006c0437f3cd6d4c9c7)) by @bchatard

- *(ui)* Add ASCII art banner and catchphrase to --help and --version - ([52da342](https://github.com/adaouat/heraut/commit/52da342cbb88ee7027f0a7c0e6c511e2ff49264e)) by @bchatard


### 📚 Documentation

- Bootstrap CLAUDE.md, project rules, specs, ADRs, and roadmap - ([349a7c5](https://github.com/adaouat/heraut/commit/349a7c5647b62d19166370caa5f3429efc023e8f)) by @bchatard

- Align project structure with bifrost reference - ([8527f62](https://github.com/adaouat/heraut/commit/8527f62da1e4d6dc544ce7eed1d0674d9a5ddd16)) by @bchatard


### ⚙️ Miscellaneous Tasks

- Install Go toolchain and wire DX tasks - ([a905881](https://github.com/adaouat/heraut/commit/a905881e698ce15bd0eeb76f40899396d0dce8ef)) by @bchatard

- Add GitHub Actions CI pipeline (T01) - ([9706761](https://github.com/adaouat/heraut/commit/97067618a87c524d1b83c08e0faae18941db4a97)) by @bchatard

- Add GoReleaser build and Docker release pipeline (T02) - ([6ffeace](https://github.com/adaouat/heraut/commit/6ffeacefbded4a066fd38fe355a16749e441f537)) by @bchatard

## [0.0.0](https://github.com/adaouat/heraut/compare/0.0.0..v0.0.0) - 2026-05-23

### ⚙️ Miscellaneous Tasks

- Init project - ([6b27f19](https://github.com/adaouat/heraut/commit/6b27f19045550b628cf3b07f4c1be33d64788589)) by @bchatard

