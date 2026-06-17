# Changelog

## [0.35.2](https://github.com/adaouat/heraut/compare/v0.35.1..v0.35.2) - 2026-06-17

### 🐛 Bug Fixes

- *(platforms/gitlab)* Skip token injection in CI autologin mode - ([26d30c9](https://github.com/adaouat/heraut/commit/26d30c92937e7dd2317cb9c4d675bb5f5e8280a8)) by @bchatard

## [0.35.1](https://github.com/adaouat/heraut/compare/v0.35.0..v0.35.1) - 2026-06-17

### 🐛 Bug Fixes

- *(platforms)* Write release notes to temp file to avoid ARG_MAX overflow - ([fc45bcb](https://github.com/adaouat/heraut/commit/fc45bcbf2dc5960d29a89763b500f1ae08654123)) by @bchatard

## [0.35.0](https://github.com/adaouat/heraut/compare/v0.34.0..v0.35.0) - 2026-06-16

### 🚀 Features

- *(scaffold)* Carry through per-platform overrides on heraut init update - ([b5cd66e](https://github.com/adaouat/heraut/commit/b5cd66e83b7a5c31f205e340af86d4bc377c0e39)) by @bchatard

- *(scaffold)* Carry through per-env changelog/release on heraut init update (T109) - ([0298873](https://github.com/adaouat/heraut/commit/0298873208a10b414e59276bd907acb7d01c26b8)) by @bchatard

## [0.34.0](https://github.com/adaouat/heraut/compare/v0.33.0..v0.34.0) - 2026-06-16

### 🚀 Features

- *(cmd)* Add --env auto to resolve environment from current branch - ([801d3fc](https://github.com/adaouat/heraut/commit/801d3fccfbbfec477522a303e34f24e32956560b)) by @bchatard


### 📚 Documentation

- *(roadmap)* T108 design spike — choose positional-match carry-through - ([3d99ea5](https://github.com/adaouat/heraut/commit/3d99ea5fa2a7eda5c97ffd3cc94f5860e1f8a46a)) by @bchatard

- *(roadmap)* Add T110 — `--env auto` branch-based env detection - ([16caa8a](https://github.com/adaouat/heraut/commit/16caa8a0950814327360505624d25ebca35b26fe)) by @bchatard

## [0.33.0](https://github.com/adaouat/heraut/compare/v0.32.0..v0.33.0) - 2026-06-15

### 🚀 Features

- *(deps)* Bump forge to v0.17.0 - ([8feead4](https://github.com/adaouat/heraut/commit/8feead4698d652b88d20287dcfc5b80a2a76ee91)) by @bchatard

- *(scaffold)* Preserve release.assets, tickets, remote_metadata on init update - ([c58327e](https://github.com/adaouat/heraut/commit/c58327efa35ab4305385923ffb2c0e4d00d0df74)) by @bchatard


### 🐛 Bug Fixes

- *(versioning/semver)* Anchor BREAKING CHANGE footer to line start (T106) - ([babd265](https://github.com/adaouat/heraut/commit/babd26541d22ed792e7de2e7df70a5a9d9129356)) by @bchatard

- *(versioning/semver)* Require BREAKING CHANGE footer to start its paragraph - ([b370bfa](https://github.com/adaouat/heraut/commit/b370bfa14e26bcb2685fa871a6f92cd0826014fd)) by @bchatard


### 📚 Documentation

- *(roadmap)* Add T106 — fix isBreaking footer-anchoring false positive - ([7181aba](https://github.com/adaouat/heraut/commit/7181aba8b78ededa05b61c1a16559f3c90de63b1)) by @bchatard

- *(roadmap)* Add T107/T108 — heraut init config round-trip - ([99e818e](https://github.com/adaouat/heraut/commit/99e818ec6d2bee0bebbc3d0397130aeaec39ab91)) by @bchatard

## [0.32.0](https://github.com/adaouat/heraut/compare/v0.31.0..v0.32.0) - 2026-06-14

### 🚀 Features

- *(app)* Check runtime --env checks the env's effective platforms - ([74ae0e1](https://github.com/adaouat/heraut/commit/74ae0e1f1f4d222ff4f1f8ef6c0e49f2f9063a64)) by @bchatard

- *(config)* Validate tag_pattern requires git-cliff generator - ([bcd9467](https://github.com/adaouat/heraut/commit/bcd9467dd5671b73d1660a09aae7d32f6bfb26b6)) by @bchatard

- *(scaffold)* Warn before dropping unsupported settings on init update - ([1a0f3f9](https://github.com/adaouat/heraut/commit/1a0f3f9d94778290479efaacc4b3b38cf26427b0)) by @bchatard


### 🐛 Bug Fixes

- *(cmd)* Unify --version override validation across release/changelog - ([e269720](https://github.com/adaouat/heraut/commit/e2697206840e0567de991e48a3494497c5cbd9a3)) by @bchatard

- *(cmd)* Align check/version validation, no-config, and exit codes - ([d2406b6](https://github.com/adaouat/heraut/commit/d2406b61f23695cbf33846a195559e38fda19968)) by @bchatard

- *(cmd/release)* Inherit root platforms when env release omits platforms - ([1b5283e](https://github.com/adaouat/heraut/commit/1b5283eb7d850281d349dcea22175e9243a11ca3)) by @bchatard

- *(pipeline)* Push only the resolved tag, not all local tags - ([8e08976](https://github.com/adaouat/heraut/commit/8e08976003c721da462b556e52de6f36bbfa32fd)) by @bchatard

- *(platforms/gitlab)* Inject token_env into release-time glab calls - ([bd0bcb9](https://github.com/adaouat/heraut/commit/bd0bcb975bd84c68c9563bbbe325c324731a0a9a)) by @bchatard

- *(versioning/perenv)* Skip pre-release tags in semver-per-env BumpAuto - ([938b911](https://github.com/adaouat/heraut/commit/938b91181f07b8fc418a5d017268f61834df719a)) by @bchatard

- *(versioning/semver)* Anchor breaking-change bang to commit type prefix - ([f4a8e9e](https://github.com/adaouat/heraut/commit/f4a8e9e53c579f10f520f57d80f8dd1a9fd960fe)) by @bchatard

- *(versioning/semver)* Skip pre-release tags when resolving current version - ([3e4b71a](https://github.com/adaouat/heraut/commit/3e4b71a75718abe1995e2a20788effbe4a270200)) by @bchatard


### 🚜 Refactor

- *(config)* Extract shared EffectivePlatforms helper (T105) - ([f9ec386](https://github.com/adaouat/heraut/commit/f9ec386ac7e296c0a2803a0d87323f097aedb51c)) by @bchatard

- Hygiene pass — drop T-id comments, dedupe iota, unify case - ([8413f47](https://github.com/adaouat/heraut/commit/8413f4768a851b6c7942c421899f62bf30539582)) by @bchatard


### 📚 Documentation

- *(claude)* Rewrite CLAUDE.md to post-forge reality - ([6aa08e4](https://github.com/adaouat/heraut/commit/6aa08e459db06656f3dc1cd649f2f8356fdfabb3)) by @bchatard

- *(roadmap)* Add phase 17 — full-project review remediation - ([0cdca6a](https://github.com/adaouat/heraut/commit/0cdca6ae4133cdc9a0fda057da6f92318e2bd3ce)) by @bchatard

- *(roadmap)* Reconcile v1.0.0 checkpoints and refresh overview - ([98702f2](https://github.com/adaouat/heraut/commit/98702f244584a7891bcf958594c78fdc932d6c2b)) by @bchatard

- *(roadmap)* Add T105 — extract shared effective-platforms helper - ([319d73b](https://github.com/adaouat/heraut/commit/319d73b98c2406e0cb40aaebaba5aa8d1d7b48c8)) by @bchatard

- *(specs)* Sync platform tables and document whatsnew (T94) - ([b1ac326](https://github.com/adaouat/heraut/commit/b1ac326b2b5c809852c2d0364e80b81a2e060932)) by @bchatard

- *(specs)* Reflect T91's anchored breaking-change detection in spec 04 - ([25be6cd](https://github.com/adaouat/heraut/commit/25be6cd18cc144f13817842cb7e1bb1a0349dac1)) by @bchatard

- *(specs)* Fix remaining name/catalog/upload-asset drift - ([ba47155](https://github.com/adaouat/heraut/commit/ba47155b91038dd4d861732b0d203904cc8504d4)) by @bchatard

## [0.31.0](https://github.com/adaouat/heraut/compare/v0.30.2..v0.31.0) - 2026-06-12

### 🚀 Features

- *(app)* One heraut check runtime Platforms row per configured entry - ([6bdc4b4](https://github.com/adaouat/heraut/commit/6bdc4b44187766af1d426db10a848848602f075e)) by @bchatard

- *(config)* Require unique platform name, allow self-hosted base_url - ([2d6f2f6](https://github.com/adaouat/heraut/commit/2d6f2f6cb079595a26e2bd9d0da081a6e7a3fd4d)) by @bchatard

- *(platforms/github)* Support self-hosted instances via GH_HOST - ([c113508](https://github.com/adaouat/heraut/commit/c113508953c9a43d714ac57ce7fe24eb5d3d3d19)) by @bchatard

- *(platforms/gitlab)* Support self-hosted instances via GITLAB_HOST - ([3bfdb06](https://github.com/adaouat/heraut/commit/3bfdb0676abda29058987054f361ba0752e2a4e3)) by @bchatard


### 📚 Documentation

- *(adr)* Add ADR-0025, supersede ADR-0020 (multi-instance platforms) - ([b3ae621](https://github.com/adaouat/heraut/commit/b3ae6216d5b734a92c4fc4cc8bc69ae70d126717)) by @bchatard

- *(app)* Clarify Platforms section dispatch order in RuntimeCheck - ([e47b250](https://github.com/adaouat/heraut/commit/e47b250d7a14ae90b40bb83ee65c586c3201ebf2)) by @bchatard

- *(plans)* Add multi-instance platforms implementation plan - ([4f9eb70](https://github.com/adaouat/heraut/commit/4f9eb7055cec2156663327004a17ac630fd1c3f1)) by @bchatard

- Design spec for multi-instance same-platform releases - ([2b3e99b](https://github.com/adaouat/heraut/commit/2b3e99be5926448915e5f7d90f4131ee7841a861)) by @bchatard


### 🎨 Styling

- *(app)* Drop unneeded loop variable copy in check.go - ([60b3e9d](https://github.com/adaouat/heraut/commit/60b3e9dc9c919bb7f0b2fe008753bbe0b0c2fb3e)) by @bchatard

## [0.30.2](https://github.com/adaouat/heraut/compare/v0.30.1..v0.30.2) - 2026-06-12

### 🐛 Bug Fixes

- *(deps)* Bump forge to v0.16.0 - ([56d38f7](https://github.com/adaouat/heraut/commit/56d38f7d51bce67cf201de5d585feff5370303ba)) by @bchatard

## [0.30.1](https://github.com/adaouat/heraut/compare/v0.30.0..v0.30.1) - 2026-06-11

### 🎨 Styling

- Gofmt -s on calver parser_test - ([e5ec535](https://github.com/adaouat/heraut/commit/e5ec535375aa14ca9c872bf563cece3ce3ffe5e5)) by @bchatard


### ⚙️ Miscellaneous Tasks

- Run the full hk linter set in CI (hadolint, actionlint, typos, …) - ([0906de5](https://github.com/adaouat/heraut/commit/0906de56c76fdaf2dc5e1700e2d4ccfd1b372e69)) by @bchatard

- Fix shellcheck findings in release.yml run blocks - ([c8cb125](https://github.com/adaouat/heraut/commit/c8cb1253ee4d1e715a99727187b4cd6e289058ea)) by @bchatard

## [0.30.0](https://github.com/adaouat/heraut/compare/v0.29.0..v0.30.0) - 2026-06-11

### 🚀 Features

- *(app)* Propagate tickets config onto content drivers - ([10410fa](https://github.com/adaouat/heraut/commit/10410fa7a6d9b2ab7709376d95b9b2370697abdf)) by @bchatard

- *(config)* Add tickets field for issue-tracker links - ([770c318](https://github.com/adaouat/heraut/commit/770c31816a7b13aafe690fff35b81d64d9331f05)) by @bchatard

- *(config)* Validate tickets (regex, {ticket} url, git-cliff only) - ([a159801](https://github.com/adaouat/heraut/commit/a15980141a30bd5e5e20cd0b4918093211621400)) by @bchatard

- *(config)* Add tickets to schema + sample + fixtures - ([0de6706](https://github.com/adaouat/heraut/commit/0de6706295a1d00cb1b9b2c1376640825cd4cbb4)) by @bchatard

- *(generators/gitcliff)* Inject ticket link_parsers into effective config - ([cc1925d](https://github.com/adaouat/heraut/commit/cc1925dfdf822934e8aabf0381326a20f6afaad7)) by @bchatard

- *(generators/gitcliff)* Render ticket links in changelog and release notes - ([98f83e1](https://github.com/adaouat/heraut/commit/98f83e14817ef671d209d9d0aeeab29ac1f489fe)) by @bchatard


### 📚 Documentation

- *(plans)* Add ticket-linking design spec - ([6dee113](https://github.com/adaouat/heraut/commit/6dee113ec25380379575f08f006a51707d73d31c)) by @bchatard

- *(plans)* Add ticket-linking implementation plan - ([d33f307](https://github.com/adaouat/heraut/commit/d33f307cb016e3e6f72b89e3882d996a523ed9d3)) by @bchatard

- *(roadmap)* Add Phase 15 — ticket linking (T79–T82) - ([dfc44c5](https://github.com/adaouat/heraut/commit/dfc44c54cea71fa718d170607ea66f86f3fe048a)) by @bchatard

- ADR-0024 ticket linking; document tickets; complete T79–T82 - ([485c07a](https://github.com/adaouat/heraut/commit/485c07af93d398cc78db68b238a320834e9d4391)) by @bchatard

## [0.29.0](https://github.com/adaouat/heraut/compare/v0.28.0..v0.29.0) - 2026-06-11

### 🚀 Features

- *(cmd)* Add --build to heraut release for build-id release flows - ([280b29d](https://github.com/adaouat/heraut/commit/280b29df0e307f41ff402180daa79376996a2e01)) by @bchatard


### 🐛 Bug Fixes

- *(generators/gitcliff)* Render New Contributors from any platform's remote - ([316ce84](https://github.com/adaouat/heraut/commit/316ce846e33400cabea6cbc6200a6d6d310f823d)) by @bchatard


### 📚 Documentation

- *(roadmap)* Mark T68 done (checkbox lag; work landed in ADR-0021) - ([7061e89](https://github.com/adaouat/heraut/commit/7061e89fa3c6ce8b419d6aae8b2bc1c4f6315f04)) by @bchatard

- *(roadmap)* Complete T57 (heraut release --build) - ([d86c09b](https://github.com/adaouat/heraut/commit/d86c09ba961e31c742bbfd8749b23e026487ae0a)) by @bchatard

- *(specs)* Document heraut release --build (T57) - ([3e8e37a](https://github.com/adaouat/heraut/commit/3e8e37a8ecd53a158f1ce8738ae26f6c8272c90d)) by @bchatard


### 🎨 Styling

- *(generators/gitcliff)* Drop redundant version heading from release notes - ([c977f61](https://github.com/adaouat/heraut/commit/c977f61c2386f848bedeb19a58b389aaf00c4297)) by @bchatard


### ⚙️ Miscellaneous Tasks

- Keep CHANGELOG.md in Docker build context for the whatsnew embed - ([acac7b6](https://github.com/adaouat/heraut/commit/acac7b60e7022ccc844b8792012b2a11a6820f38)) by @bchatard

## [0.28.0](https://github.com/adaouat/heraut/compare/v0.27.0..v0.28.0) - 2026-06-10

### 🚀 Features

- *(whatsnew)* Embed CHANGELOG.md as the offline fallback - ([09fe57c](https://github.com/adaouat/heraut/commit/09fe57cd825096f7d42c738e2203a8bd51e6778e)) by @bchatard


### 💼 Other

- *(deps)* Bump forge to v0.15.0 - ([93cee93](https://github.com/adaouat/heraut/commit/93cee93d646ecb2e7e501c5f675055a5ca3e4a60)) by @bchatard

## [0.27.0](https://github.com/adaouat/heraut/compare/v0.26.0..v0.27.0) - 2026-06-10

### 🚀 Features

- *(cmd)* Add whatsnew command, adopt forge's shared hint wiring - ([9457530](https://github.com/adaouat/heraut/commit/94575309ed2de5f2f78a56a803feec796cfa69b9)) by @bchatard


### 💼 Other

- *(deps)* Bump forge to v0.13.0 - ([6a4f278](https://github.com/adaouat/heraut/commit/6a4f2782d1dd3410372d625a44e5ea6252e2aded)) by @bchatard

- *(deps)* Bump forge to v0.14.0 - ([b36f93a](https://github.com/adaouat/heraut/commit/b36f93add337fa6c7d99110dd257d4c66c79cea5)) by @bchatard

## [0.26.0](https://github.com/adaouat/heraut/compare/v0.25.1..v0.26.0) - 2026-06-10

### 🚀 Features

- *(app)* Propagate remote_metadata policy + report cliff degrade - ([a465871](https://github.com/adaouat/heraut/commit/a465871cbb4e7626297cdb82141f38467ea62305)) by @bchatard

- *(cmd)* Add --offline flag forcing remote_metadata: disabled - ([2dffbe0](https://github.com/adaouat/heraut/commit/2dffbe086b63c65faee0a337b12f20c0d8f580d6)) by @bchatard

- *(config)* Add remote_metadata policy field + validation - ([b9a467c](https://github.com/adaouat/heraut/commit/b9a467c4ccfa3437955a3d0d080ef34e3d30a747)) by @bchatard

- *(config)* Add remote_metadata to schema.json + sample + fixtures - ([7aa259f](https://github.com/adaouat/heraut/commit/7aa259f04d1b711bc6f802f110bcf0c5fda3b412)) by @bchatard

- *(generators/gitcliff)* Honor remote_metadata policy via --offline - ([e14c020](https://github.com/adaouat/heraut/commit/e14c020028999da0abf7a1b07104ad817c1c353f)) by @bchatard

- *(pipeline)* Surface remote-metadata degrade as a step sub-result - ([7c682e1](https://github.com/adaouat/heraut/commit/7c682e15015da32aeea5a11c16e3f03f942e1528)) by @bchatard


### 📚 Documentation

- *(adr)* Add ADR-0023 remote-metadata policy; complete roadmap T78 - ([fb22f0a](https://github.com/adaouat/heraut/commit/fb22f0a61983c46cd8d889538291e1f71e678089)) by @bchatard

- *(adr)* Record why the CLI override is boolean --offline (ADR-0023) - ([e93cb06](https://github.com/adaouat/heraut/commit/e93cb06291a66e7aa844da7cd406d382f5225b2c)) by @bchatard

- *(roadmap)* Add T78 — remote_metadata policy for git-cliff enrichment - ([9680617](https://github.com/adaouat/heraut/commit/9680617638b460bcadf76f93bb7812460f7f18f8)) by @bchatard


### ⚙️ Miscellaneous Tasks

- Authenticate git-cliff GitHub API calls in release workflow - ([cc8f128](https://github.com/adaouat/heraut/commit/cc8f1282c3f848ad16916a4818d7488aef9cb96e)) by @bchatard

## [0.25.1](https://github.com/adaouat/heraut/compare/v0.25.0..v0.25.1) - 2026-06-10

### 🐛 Bug Fixes

- *(deps)* Run go mod tidy - ([bc8a7b9](https://github.com/adaouat/heraut/commit/bc8a7b9bf88bd836c545353503f21cb3792e0398)) by @bchatard


### 💼 Other

- *(deps)* Bump forge to v0.11.1 - ([82dd26c](https://github.com/adaouat/heraut/commit/82dd26c2ce1700f243a3a5afd185ce5da61a29e7)) by @bchatard


### 🚜 Refactor

- Align forge/log import alias to forgelog - ([14750f4](https://github.com/adaouat/heraut/commit/14750f4ee20b1708eee0320f67fd454446f9a87e)) by @bchatard

## [0.25.0](https://github.com/adaouat/heraut/compare/v0.24.0..v0.25.0) - 2026-06-09

### 🚀 Features

- *(generators/gitcliff)* Fat injection / thin branch-free templates - ([1c5301d](https://github.com/adaouat/heraut/commit/1c5301dcb8fa23d4f8cc890cff26aa96b8ff581a)) by @bchatard


### 🐛 Bug Fixes

- *(generators/cocogitto)* Repair embedded cog.toml for cog 7.0.0 + enrich - ([b8b659e](https://github.com/adaouat/heraut/commit/b8b659ee55e6637b41ae468657674bb9c22f39b2)) by @bchatard


### 📚 Documentation

- *(roadmap)* Add T76 — richer cocogitto default templates - ([f0396f8](https://github.com/adaouat/heraut/commit/f0396f81165f7c910e013710d0a0e700542365d1)) by @bchatard


### 🧪 Testing

- *(generators)* Real-CLI smoke tests for embedded default configs - ([960d24b](https://github.com/adaouat/heraut/commit/960d24b8d99d0abdcb5fbabebef8c32a3c4517d7)) by @bchatard

## [0.24.0](https://github.com/adaouat/heraut/compare/v0.23.1..v0.24.0) - 2026-06-09

### 🚀 Features

- *(config)* Add per-platform base_url field with self-hosted gate - ([35da050](https://github.com/adaouat/heraut/commit/35da0504a9b50c4b82d300f1c9c73b83d1dcc8a1)) by @bchatard

- *(generators/cocogitto)* Render per-platform commit links in templates - ([52290c4](https://github.com/adaouat/heraut/commit/52290c4da924bfc54bb592d0173648c94669c411)) by @bchatard

- *(generators/gitcliff)* Prefer heraut-injected link context in templates - ([928e917](https://github.com/adaouat/heraut/commit/928e91780c7ede55c09808984adaa9fcec4d2a41)) by @bchatard

- *(pipeline)* Regenerate release notes per platform when multi-target - ([3062929](https://github.com/adaouat/heraut/commit/3062929c8fa94402418298883282c861a8403d69)) by @bchatard

- *(port)* Carry per-platform LinkContext through Generator.Generate - ([78d25c0](https://github.com/adaouat/heraut/commit/78d25c079d356ff1a74af3ccccbed8bf0a446982)) by @bchatard

- *(port)* Expose per-platform LinkContext() on port.Platform - ([34aff60](https://github.com/adaouat/heraut/commit/34aff60cb1c2297e962364ad8f0ee842d7302421)) by @bchatard

- *(release)* Add operator-debugging diagnostics to the release pipeline - ([46ebbc7](https://github.com/adaouat/heraut/commit/46ebbc7ad8924fae1c2960e990cd19d0aea3df46)) by @bchatard

- *(ui)* Brand the heraut banner in heraldic gold - ([cc01486](https://github.com/adaouat/heraut/commit/cc01486781282e44442f612110ad70e8513af4d2)) by @bchatard


### 🐛 Bug Fixes

- *(generators/gitcliff)* GitHub PR links use /pull/ not /pulls/ - ([d5a6353](https://github.com/adaouat/heraut/commit/d5a6353bb2c297db9c8c0b6f14aec92046545afb)) by @bchatard


### 💼 Other

- *(deps)* Bump forge to v0.11.0 - ([aa7ddb6](https://github.com/adaouat/heraut/commit/aa7ddb6432b798f647c7c766ac469e587b323c8e)) by @bchatard


### 📚 Documentation

- *(adr)* Add 0020 per-platform base_url; complete T65 - ([c60a5fc](https://github.com/adaouat/heraut/commit/c60a5fc0ffe72b9bf982f3cba5ababd7f5c2ea51)) by @bchatard

- *(adr)* Record single-platform CI non-regression invariant for base_url - ([7a8f93e](https://github.com/adaouat/heraut/commit/7a8f93e699ee909013f52009753a4b9d8d93fc07)) by @bchatard

- *(adr)* Defer ReleaseURL base_url wiring to the host-targeting thread - ([6790cd0](https://github.com/adaouat/heraut/commit/6790cd09092d20bc949570c5c67be31e94b81a73)) by @bchatard

- *(adr)* Add 0021 per-platform release notes; complete T67 - ([9b50820](https://github.com/adaouat/heraut/commit/9b508208b034440f8d27946c85e209552c1caf05)) by @bchatard

- *(adr)* Resolve T68 context-injection shape with PoC findings - ([259cebf](https://github.com/adaouat/heraut/commit/259cebff49a865ab28c6676bb3942c05e5b4493f)) by @bchatard

- *(cmd)* Clarify --verbose help — echoes commands and emits diagnostics - ([1f32a53](https://github.com/adaouat/heraut/commit/1f32a5341149777bdaf065080d62d311a7f27260)) by @bchatard

- *(roadmap)* Add Phase 14 — multi-platform release correctness (T65-T73) - ([c30c695](https://github.com/adaouat/heraut/commit/c30c695ecb966523e5fe3a9759c32c478d9d1abc)) by @bchatard

- *(roadmap)* Add T75 — fat-injection / thin templates (deferred) - ([e378691](https://github.com/adaouat/heraut/commit/e3786911383df1a21ff3ab909e96b4a4c3fe4953)) by @bchatard

- *(roadmap)* Refine T75 — concrete prefix vars, fully-thin contract - ([28b4aa3](https://github.com/adaouat/heraut/commit/28b4aa32c545d5d5a066ff0c0ea60bd2db8a1405)) by @bchatard

- *(spec)* Document communique's multi-platform link-resolution exclusion - ([546836d](https://github.com/adaouat/heraut/commit/546836dd4e1ff81e715a4e2211a8cbc2c8db618e)) by @bchatard


### 🧪 Testing

- *(pipeline)* End-to-end multi-platform distinct-notes integration test - ([432a78f](https://github.com/adaouat/heraut/commit/432a78fa0971aba58b55b3cebd591be92939cd59)) by @bchatard

## [0.23.1](https://github.com/adaouat/heraut/compare/v0.23.0..v0.23.1) - 2026-06-07

### 🐛 Bug Fixes

- Readable --help usage block (bump forge to v0.9.0) - ([618e95d](https://github.com/adaouat/heraut/commit/618e95d2c640aa7dda213e409362c1b05a0458a0)) by @bchatard

## [0.23.0](https://github.com/adaouat/heraut/compare/v0.22.0..v0.23.0) - 2026-06-05

### 🚀 Features

- *(cmd)* Adopt forge cli.Run + theme (drop direct fang) - ([ea3eed8](https://github.com/adaouat/heraut/commit/ea3eed8d3ea36ab89f6b2be00df37836f561040c)) by @bchatard

- Brand huh prompts with the Heraldic theme - ([6801a3d](https://github.com/adaouat/heraut/commit/6801a3d5540d559001a0327b0888083fe1edf7c8)) by @bchatard

## [0.22.0](https://github.com/adaouat/heraut/compare/v0.21.1..v0.22.0) - 2026-06-05

### 🚀 Features

- *(cmd)* Apply the Heraldic fang theme (forge v0.7.0) - ([e2add7d](https://github.com/adaouat/heraut/commit/e2add7ddf62bc54d818eebc7e325dd3c350091aa)) by @bchatard


### ⚙️ Miscellaneous Tasks

- *(release)* Use global git identity, simplify cask commit - ([1bf1f44](https://github.com/adaouat/heraut/commit/1bf1f44dfd36f063d7170dc66841359bf42c99f7)) by @bchatard

- *(release)* Use forge's release-setup composite action - ([e9b6d74](https://github.com/adaouat/heraut/commit/e9b6d747f35e7c0e8ab29b8c989b868646f44814)) by @bchatard

## [0.21.1](https://github.com/adaouat/heraut/compare/v0.21.0..v0.21.1) - 2026-06-04

### 💼 Other

- Bump Go toolchain lock to 1.26.4 - ([45233c8](https://github.com/adaouat/heraut/commit/45233c8f891ac9128125e3c8122620204b876bbe)) by @bchatard

- Bump go.mod directive to 1.26.4 - ([2dd5018](https://github.com/adaouat/heraut/commit/2dd5018a15cdc064a150487e9c5751107cd52a69)) by @bchatard

- Depend on published forge v0.6.2 (drop replace) - ([e5d3d09](https://github.com/adaouat/heraut/commit/e5d3d09394721f0e3feb279cc87b85f4913a833e)) by @bchatard


### 🚜 Refactor

- *(cmd)* Drop self-update, use forge updatecheck hint - ([b92ff7a](https://github.com/adaouat/heraut/commit/b92ff7ae4e817e642bf4b17b89c3e28ff98ba6d2)) by @bchatard

- *(config)* Use forge config primitives - ([53e89f5](https://github.com/adaouat/heraut/commit/53e89f52b4f6578daf2f1891e6452b407e6372f1)) by @bchatard

- *(config)* Emit forge's config error wording - ([17d46eb](https://github.com/adaouat/heraut/commit/17d46eb5526957fe86f1e7ee0f1bdb612efc8ea6)) by @bchatard

- *(exec)* Consume forge exec.Runner and exectest - ([e7fc732](https://github.com/adaouat/heraut/commit/e7fc7322e75e9cb8046e62ede621ac5c169a8dc6)) by @bchatard

- *(exitcode)* Delegate Wrap/Resolve to forge - ([9f835fa](https://github.com/adaouat/heraut/commit/9f835fab94e127c239712a9b11a987fbda75a3e3)) by @bchatard

- *(exitcode)* Re-export forge's shared exit codes - ([a5e9d5f](https://github.com/adaouat/heraut/commit/a5e9d5fa1fb60f184e6978ec955391585e282cc5)) by @bchatard

- *(ui)* Route status/header/detection through forge - ([2af13c8](https://github.com/adaouat/heraut/commit/2af13c88865789792306f6f102bffc92eaaeabd7)) by @bchatard

- *(ui)* Run checks and pipeline steps via forge.Spinner - ([42828e6](https://github.com/adaouat/heraut/commit/42828e6a94ff728b10347876bd1f70d09ad5ca1b)) by @bchatard


### 📚 Documentation

- *(readme)* Drop removed self-update references - ([75ae055](https://github.com/adaouat/heraut/commit/75ae055c07d79cdf90cad27279c33f96a043192f)) by @bchatard

- Mark self-update superseded across specs and ADR-0014 - ([eb811b3](https://github.com/adaouat/heraut/commit/eb811b38c107b6a4dcfa5b081607769088126a58)) by @bchatard

- Document heraut Homebrew cask install and ADR notes - ([5d5b008](https://github.com/adaouat/heraut/commit/5d5b008b1b82408647402f6d8280294d13c2b03b)) by @bchatard


### ⚙️ Miscellaneous Tasks

- Call forge's shared lint/test reusable workflow - ([42397fe](https://github.com/adaouat/heraut/commit/42397fe70b3a8878318ea754f30d3c7a9be31e61)) by @bchatard

- Re-pin forge go-ci to v0.6.1 and set coverage-threshold - ([661119e](https://github.com/adaouat/heraut/commit/661119e6ed147a339fc966d934467c78e62d47c7)) by @bchatard

- Publish Homebrew cask to adaouat/homebrew-tap - ([697056e](https://github.com/adaouat/heraut/commit/697056e92238a6585d384fe237e28ce1f7f16e1c)) by @bchatard

## [0.21.0](https://github.com/adaouat/heraut/compare/v0.20.1..v0.21.0) - 2026-06-02

### 🚀 Features

- *(cmd)* Add --no-push flag to heraut changelog - ([c807c1d](https://github.com/adaouat/heraut/commit/c807c1d5960bc289499f6f1784c901bb6920857b)) by @bchatard

## [0.20.1](https://github.com/adaouat/heraut/compare/v0.20.0..v0.20.1) - 2026-06-02

### 🐛 Bug Fixes

- *(selfupdate)* Only hint when latest is strictly newer than current - ([49557ce](https://github.com/adaouat/heraut/commit/49557ce6c81350aa39124c9a79f7211bcb4454ea)) by @bchatard

## [0.20.0](https://github.com/adaouat/heraut/compare/v0.19.0..v0.20.0) - 2026-06-02

### 🚀 Features

- *(app)* Enforce per-env branch guard (T60) - ([56c0445](https://github.com/adaouat/heraut/commit/56c0445fdf78522bac643aae128081e39247f634)) by @bchatard

- *(changelog)* Auto-derive tag_pattern to scope per-env changelogs (T61) - ([c8970a3](https://github.com/adaouat/heraut/commit/c8970a3e7f6fb6413c591560d3591e583587e10e)) by @bchatard

- *(changelog)* Strip env/build tokens from changelog headings (T62) - ([359d9fd](https://github.com/adaouat/heraut/commit/359d9fdd7482b521662d7066e617edab567e1d46)) by @bchatard

- *(config)* Per-env content-driver overrides deep-merge (T63, ADR-0019) - ([e0446f2](https://github.com/adaouat/heraut/commit/e0446f21b125ae560aaa4b5967a3a6d04151d91d)) by @bchatard


### 📚 Documentation

- *(roadmap)* Add T63 — per-env content-driver overrides deep-merge - ([3887510](https://github.com/adaouat/heraut/commit/38875102af09ce22ac09415ca5fe59f78fc69c7f)) by @bchatard

- Clarify that an omitted env branch imposes no restriction (T60) - ([e4d4bba](https://github.com/adaouat/heraut/commit/e4d4bba9d84e6eced5ecc5ab67276ab2657dd93d)) by @bchatard

## [0.19.0](https://github.com/adaouat/heraut/compare/v0.18.0..v0.19.0) - 2026-06-01

### 🚀 Features

- *(changelog)* {build} token and --build flag for CI build IDs - ([7f66a21](https://github.com/adaouat/heraut/commit/7f66a21f4576aed684579bd474b72d1d679b6491)) by @bchatard

- *(changelog)* Validate --build value up front (T55) - ([533fe56](https://github.com/adaouat/heraut/commit/533fe56babcd51fb01a0e00de7e9f4629ecc0cc9)) by @bchatard

- *(gitcliff)* Derive build-id postprocessor pattern from tag_format - ([1a81167](https://github.com/adaouat/heraut/commit/1a811677d0426095be1bd462f2c9923ab1df0c60)) by @bchatard

- *(tagfmt)* Actionable error when {build} format lacks a build ID (T59) - ([7283fe8](https://github.com/adaouat/heraut/commit/7283fe8f6e6a0d148413347d7cc9f0be4349bbad)) by @bchatard

- *(version)* Add --bare to version current (T58) - ([9926afb](https://github.com/adaouat/heraut/commit/9926afbcd2ab560acfe5d3461bbf1feb4c746dd4)) by @bchatard


### 🐛 Bug Fixes

- *(app)* Version current honours top-level tag_format (T54) - ([4354bdc](https://github.com/adaouat/heraut/commit/4354bdcdda8ea1f02905a8c8a2f35a30da156c9d)) by @bchatard

- *(cliff)* Reflect injected build-id postprocessor in effective config (T56) - ([bcb9f01](https://github.com/adaouat/heraut/commit/bcb9f018f0fe8fbadb41cfecc9e83a550911f2e2)) by @bchatard


### 📚 Documentation

- *(guides)* Mobile/CI multi-build tagging how-to - ([40dbec9](https://github.com/adaouat/heraut/commit/40dbec90db926fcd92385b2650aae052089a57a6)) by @bchatard

- *(roadmap)* Note T57 held pending production use of the changelog flow - ([e94d34a](https://github.com/adaouat/heraut/commit/e94d34a1650c60642a100737ef721ad0cad7e820)) by @bchatard

- Record build-id review findings as roadmap T53–T59 + spec caveats - ([4c6b9f7](https://github.com/adaouat/heraut/commit/4c6b9f78fa160f09cae803c69a8e5502236701f8)) by @bchatard


### 🧪 Testing

- *(app)* Cover build-id resolver and CurrentVersion paths - ([ab78fa4](https://github.com/adaouat/heraut/commit/ab78fa434e77897e3ffd2b9572db6a51de620492)) by @bchatard


### ⚙️ Miscellaneous Tasks

- *(osv-scan)* Point at the subdirectory action, not the repo root - ([6025dd1](https://github.com/adaouat/heraut/commit/6025dd1958092c5db27ae21376e1790e8ff3c8bf)) by @bchatard

## [0.18.0](https://github.com/adaouat/heraut/compare/v0.17.0..v0.18.0) - 2026-06-01

### 🚀 Features

- *(scaffold)* Token env var is now a select with known + custom option - ([938b607](https://github.com/adaouat/heraut/commit/938b6071d6f7d615f6fd756c9391265c7d6198ea)) by @bchatard


### 🐛 Bug Fixes

- *(scaffold)* Wrap token names in backticks to prevent italic rendering - ([6ed4d45](https://github.com/adaouat/heraut/commit/6ed4d458ab16a8d6a8e4ead329d382dc2b494a41)) by @bchatard

## [0.17.0](https://github.com/adaouat/heraut/compare/v0.16.0..v0.17.0) - 2026-06-01

### 🚀 Features

- *(scaffold)* Show CI/CD tip after GitLab platform selection in init wizard - ([58fdb4e](https://github.com/adaouat/heraut/commit/58fdb4e56809bd8b9514c052d4d3e103f2e7abee)) by @bchatard

- *(scaffold)* Pre-fill project/repo from git remote in init wizard - ([5a666e5](https://github.com/adaouat/heraut/commit/5a666e55d96e4ad676518aa758ccbba799cfaa83)) by @bchatard


### 📚 Documentation

- Update specs and roadmap for today's check and init enhancements - ([29e58e8](https://github.com/adaouat/heraut/commit/29e58e8cf0a841be410135dd9ce9d54c1c3a5128)) by @bchatard

## [0.16.0](https://github.com/adaouat/heraut/compare/v0.15.6..v0.16.0) - 2026-06-01

### 🚀 Features

- *(cmd/check)* Display config file path and resolution source - ([db4c5ff](https://github.com/adaouat/heraut/commit/db4c5ff79c632e66a86058c104efba4f00e095dc)) by @bchatard

- *(cmd/check)* Runtime check works without a config file - ([1796095](https://github.com/adaouat/heraut/commit/179609527441ed78305cf1bf72559f359b7109b4)) by @bchatard


### ⚙️ Miscellaneous Tasks

- *(release)* Fix Docker version tags for workflow_dispatch trigger - ([4e65a80](https://github.com/adaouat/heraut/commit/4e65a80cceda5514f431c3f0a7c11d514124640d)) by @bchatard

## [0.15.6](https://github.com/adaouat/heraut/compare/v0.15.5..v0.15.6) - 2026-05-31

### ⚙️ Miscellaneous Tasks

- *(ci)* Remove heraut from mise, install directly in CI - ([5fb1a5f](https://github.com/adaouat/heraut/commit/5fb1a5f52056f910ebf17b1d1629691999569a99)) by @bchatard

## [0.15.5](https://github.com/adaouat/heraut/compare/v0.15.4..v0.15.5) - 2026-05-31

### ⚙️ Miscellaneous Tasks

- *(release)* Set bot identity and disable update hint - ([b6ae4f5](https://github.com/adaouat/heraut/commit/b6ae4f5acf5935030e3bf5b4c84bed89a33f917d)) by @bchatard

## [0.15.4](https://github.com/adaouat/heraut/compare/v0.15.3..v0.15.4) - 2026-05-31

### 🐛 Bug Fixes

- *(ci)* Set GPG ownertrust and no-tty for non-interactive signing - ([317c462](https://github.com/adaouat/heraut/commit/317c462a7d135ad0adb0b5cc8f056dad23c1844b)) by @bchatard


### ⚙️ Miscellaneous Tasks

- *(release)* Use crazy-max/ghaction-import-gpg for key setup - ([45f7837](https://github.com/adaouat/heraut/commit/45f78371759a129a3287881a0cddc3de02174cfd)) by @bchatard

## [0.15.3](https://github.com/adaouat/heraut/compare/v0.15.2..v0.15.3) - 2026-05-30

### 🐛 Bug Fixes

- *(ci)* Import GPG key without base64 decode - ([919337b](https://github.com/adaouat/heraut/commit/919337b1fb5f3564f67c259c376d04e7658d53f0)) by @bchatard


### ⚙️ Miscellaneous Tasks

- *(release)* Configure GPG commit and tag signing - ([02e045d](https://github.com/adaouat/heraut/commit/02e045d424e37ad2066603f61d5cf6b4cc9ae634)) by @bchatard

## [0.15.2](https://github.com/adaouat/heraut/compare/v0.15.1..v0.15.2) - 2026-05-30

### ⚙️ Miscellaneous Tasks

- *(config)* Add release notes generator to heraut's own config - ([ad74c6c](https://github.com/adaouat/heraut/commit/ad74c6c7edf34fc29dffe10e4fe0caa0782c9905)) by @bchatard

## [0.15.1](https://github.com/adaouat/heraut/compare/v0.15.0..v0.15.1) - 2026-05-30

### 🐛 Bug Fixes

- *(platforms)* Atomic create+upload to avoid GitHub HTTP 422 - ([3058a83](https://github.com/adaouat/heraut/commit/3058a8362a57378480b00dd7e4b50df4d1f659a0)) by @bchatard

## [0.15.0](https://github.com/adaouat/heraut/compare/v0.14.0..v0.15.0) - 2026-05-30

### 🚀 Features

- *(release)* T51 — CI build-then-release pipeline - ([57a2af6](https://github.com/adaouat/heraut/commit/57a2af6bf6201040945b949f83e3c4dcbeed69f7)) by @bchatard


### 🐛 Bug Fixes

- *(ci)* Strip release block before version next in bootstrap step - ([f039ef1](https://github.com/adaouat/heraut/commit/f039ef1fd391385dfc2f4fb6d0ace72e1decfb8e)) by @bchatard


### 📚 Documentation

- *(adr)* ADR-0018 — self-bootstrapping CI release pipeline - ([5ec7576](https://github.com/adaouat/heraut/commit/5ec7576cc0d0dc9c7d519a07c4d346b3e7f24c60)) by @bchatard

- *(roadmap)* Add T51 — CI build-then-release pipeline - ([0adad5e](https://github.com/adaouat/heraut/commit/0adad5e3562fb96bdbfc90c89abb01e56fa5140f)) by @bchatard


### ⚙️ Miscellaneous Tasks

- *(mise)* Add heraut as managed tool, auto-pin after release - ([551d5ac](https://github.com/adaouat/heraut/commit/551d5acdecd0e1371b566ff943b436cc22ba29a9)) by @bchatard

## [0.14.0](https://github.com/adaouat/heraut/compare/v0.13.4..v0.14.0) - 2026-05-30

### 🚀 Features

- *(cmd)* Honour HERAUT_FILE in heraut init write destination - ([bd7f023](https://github.com/adaouat/heraut/commit/bd7f0230c479c682dad93aea7695fb4a71e1c001)) by @bchatard

- *(config)* Add HERAUT_FILE env var for config file path - ([026a3fa](https://github.com/adaouat/heraut/commit/026a3fa4b77c967e29e5fcef4957925aa5887ec2)) by @bchatard


### 🐛 Bug Fixes

- *(config)* Trim whitespace from HERAUT_FILE and isolate env var in tests - ([27fd50c](https://github.com/adaouat/heraut/commit/27fd50c20bd3e2e76a65896794c1a786c1ac3efc)) by @bchatard


### 📚 Documentation

- *(adr)* Update ADR-0005 to document HERAUT_FILE discovery step - ([16011a7](https://github.com/adaouat/heraut/commit/16011a77e107b770a0bd31a6786f5fc853313076)) by @bchatard

- *(claude)* Add config discovery and non-obvious constraints sections - ([3817029](https://github.com/adaouat/heraut/commit/3817029f970884713919ee478a27a46235e2d6e1)) by @bchatard

- *(specs)* Document pre-commit hook behaviour for changelog commit - ([52a82ea](https://github.com/adaouat/heraut/commit/52a82eac9da39d08c46a18ec913edc84aa729a27)) by @bchatard


### 🧪 Testing

- *(config)* Cover HERAUT_FILE empty string fallthrough - ([bd3cbad](https://github.com/adaouat/heraut/commit/bd3cbadbab0d35e8efc7837aaf9a0ead7d1f3a14)) by @bchatard


### ⚙️ Miscellaneous Tasks

- *(config)* Add gopls + Claude Code plugins, exclude CHANGELOG from typos - ([3f261bd](https://github.com/adaouat/heraut/commit/3f261bd052a3cf9d3970cc9ff9e94d8b9c545a7b)) by @bchatard

## [0.13.4](https://github.com/adaouat/heraut/compare/v0.13.3..v0.13.4) - 2026-05-29

### 🐛 Bug Fixes

- *(selfupdate)* Correct checksum filename to match goreleaser config - ([8645ed2](https://github.com/adaouat/heraut/commit/8645ed21eae821b6b6059e5cbef804a1d2c0b44f)) by @bchatard

## [0.13.3](https://github.com/adaouat/heraut/compare/v0.13.2..v0.13.3) - 2026-05-29

### 🐛 Bug Fixes

- *(pipeline)* Disable_changelog no longer suppresses --tag - ([2fa8b0e](https://github.com/adaouat/heraut/commit/2fa8b0ebd8f76d8db0fa0dcf8d148114af942d08)) by @bchatard


### 📚 Documentation

- *(roadmap)* Add T50 — disable_changelog should not suppress --tag - ([20c00fa](https://github.com/adaouat/heraut/commit/20c00fac71ef77ffa6c96b48686bbd6d756bf171)) by @bchatard

- *(specs)* Document release without notes as an explicit valid pattern - ([e409edb](https://github.com/adaouat/heraut/commit/e409edb640a5217eb74496f51f1b97fdb0893f35)) by @bchatard

## [0.13.2](https://github.com/adaouat/heraut/compare/v0.13.1..v0.13.2) - 2026-05-29

### 🐛 Bug Fixes

- *(cmd)* Error when heraut release has no platforms configured - ([7f38edf](https://github.com/adaouat/heraut/commit/7f38edf06100b0a590aea03cd46d6554f8c4ab0b)) by @bchatard


### 📚 Documentation

- *(specs)* Document heraut changelog tag-only workflow - ([2acd646](https://github.com/adaouat/heraut/commit/2acd64630b81fc08cf4181379ac630790522fc2b)) by @bchatard

## [0.13.1](https://github.com/adaouat/heraut/compare/v0.13.0..v0.13.1) - 2026-05-29

### 🐛 Bug Fixes

- *(pipeline)* Use explicit remote in git push calls - ([ad7dfff](https://github.com/adaouat/heraut/commit/ad7dfff5b9a4d67285332d0bf5db509ed44b7526)) by @bchatard


### 🧪 Testing

- *(platforms/github)* Cover draft+prerelease combination - ([911d362](https://github.com/adaouat/heraut/commit/911d362dfbac2d2f88bfa46239db2c7a44b6f6f4)) by @bchatard

## [0.13.0](https://github.com/adaouat/heraut/compare/v0.12.10..v0.13.0) - 2026-05-29

### 🚀 Features

- *(check)* Config-aware required vs optional tool checks in runtime - ([92b5fd1](https://github.com/adaouat/heraut/commit/92b5fd1d85e77885127592166a64ca534a424f64)) by @bchatard

- *(check)* Grouped runtime TUI — Git / Platforms / Generators - ([bc01b6e](https://github.com/adaouat/heraut/commit/bc01b6e6cecf9d9e4087b18ce527a48ae0ff1f2f)) by @bchatard

- *(config)* Rename versioning.prefix → versioning.tag_prefix - ([0e40585](https://github.com/adaouat/heraut/commit/0e40585a8d1d78a9db5bd084b2f384ade59b18e6)) by @bchatard

- *(platforms)* Add API auth verification to platform Check() - ([607cfa7](https://github.com/adaouat/heraut/commit/607cfa7e446e222fb0d80e1558cd3c227e8898b5)) by @bchatard


### 🐛 Bug Fixes

- *(check)* Always show optional tools in runtime check output - ([2842632](https://github.com/adaouat/heraut/commit/2842632776bec09356fca5c968ff5e46cb8af525)) by @bchatard

- *(config,scaffold)* Default changelog.output to CHANGELOG.md when empty - ([60ea9c7](https://github.com/adaouat/heraut/commit/60ea9c7c0e1b91dbc2bd6e6b1d69d5bbc00ba83f)) by @bchatard

- *(selfupdate)* Suppress hint and clear cache after successful update - ([e47be4d](https://github.com/adaouat/heraut/commit/e47be4db03e7fef026cd8dca45b55cf521675f81)) by @bchatard


### 📚 Documentation

- *(roadmap)* Complete Checkpoint K — Phase 10 beta polish done - ([cbafaba](https://github.com/adaouat/heraut/commit/cbafabab4e12799f0deb13906469b384e7d40e08)) by @bchatard

## [0.12.10](https://github.com/adaouat/heraut/compare/v0.12.9..v0.12.10) - 2026-05-28

### ⚙️ Miscellaneous Tasks

- Implement draft→publish flow for immutable release attestation - ([4d52b5a](https://github.com/adaouat/heraut/commit/4d52b5a91e26a89a4bab4cd18594e1aa83bde960)) by @bchatard

## [0.12.9](https://github.com/adaouat/heraut/compare/v0.12.8..v0.12.9) - 2026-05-28

### ⚙️ Miscellaneous Tasks

- Fix binary attestation to use checksums.txt - ([439cae5](https://github.com/adaouat/heraut/commit/439cae5cf69446a2bf1f0bdb8680dddd310c719b)) by @bchatard

## [0.12.8](https://github.com/adaouat/heraut/compare/v0.12.7..v0.12.8) - 2026-05-28

### ⚙️ Miscellaneous Tasks

- Use subject-path glob for binary attestation - ([6e6e386](https://github.com/adaouat/heraut/commit/6e6e386d2f80b0131f5abc76916730bde2606e11)) by @bchatard

## [0.12.7](https://github.com/adaouat/heraut/compare/v0.12.6..v0.12.7) - 2026-05-28

### ⚙️ Miscellaneous Tasks

- Use subject-checksums only for binary attestation - ([2ab644a](https://github.com/adaouat/heraut/commit/2ab644a23d56539ec8dab97a0e5bc494f502d4ef)) by @bchatard

## [0.12.6](https://github.com/adaouat/heraut/compare/v0.12.5..v0.12.6) - 2026-05-28

### ⚙️ Miscellaneous Tasks

- Switch to actions/attest for binary and image attestations - ([16f63f8](https://github.com/adaouat/heraut/commit/16f63f8549ea3a6562364560a3c2f0efec007812)) by @bchatard

## [0.12.5](https://github.com/adaouat/heraut/compare/v0.12.4..v0.12.5) - 2026-05-28

### ⚙️ Miscellaneous Tasks

- Upload attestation bundle as release asset - ([be9f817](https://github.com/adaouat/heraut/commit/be9f817ee1899c110c77149bc4f83fb1b3ca1df0)) by @bchatard

## [0.12.4](https://github.com/adaouat/heraut/compare/v0.12.3..v0.12.4) - 2026-05-28

### ⚙️ Miscellaneous Tasks

- Add artifact-metadata: write permission to docker jobs - ([decfcbc](https://github.com/adaouat/heraut/commit/decfcbccc2625b5533ee4bef199ae2dc933dd01d)) by @bchatard

## [0.12.3](https://github.com/adaouat/heraut/compare/v0.12.2..v0.12.3) - 2026-05-28

### 📚 Documentation

- *(roadmap)* Add Phase 10 — Beta Polish (T43–T49) - ([6de2674](https://github.com/adaouat/heraut/commit/6de267472726373d8a6bff0a01e22a6ad25dc562)) by @bchatard


### ⚙️ Miscellaneous Tasks

- Add attestations: write permission to release and docker jobs - ([a265f96](https://github.com/adaouat/heraut/commit/a265f96278f9c34b3b7627d5a7f9d24cada0e2e0)) by @bchatard

- Build linux/amd64 and linux/arm64 Docker images in parallel - ([f51daeb](https://github.com/adaouat/heraut/commit/f51daeba215080bf2f1314b9f193cc2687a2c958)) by @bchatard

## [0.12.2](https://github.com/adaouat/heraut/compare/v0.12.1..v0.12.2) - 2026-05-28

### ⚙️ Miscellaneous Tasks

- *(docker)* Disable update check in bundled image - ([422b199](https://github.com/adaouat/heraut/commit/422b1995df8a023e368bba2da80fc7e5613515d9)) by @bchatard

- *(mise)* Upgrade goreleaser 2.15 → 2.16 - ([220f4ab](https://github.com/adaouat/heraut/commit/220f4abda2bfbb3b79ea9e57a7247ab96f8b38d6)) by @bchatard

- Add build provenance attestations to release and docker jobs - ([6896852](https://github.com/adaouat/heraut/commit/6896852d0c06f34119893c257276f52ce2a766d2)) by @bchatard

## [0.12.1](https://github.com/adaouat/heraut/compare/v0.12.0..v0.12.1) - 2026-05-28

### 🐛 Bug Fixes

- *(scaffold)* Write prefix: "" explicitly when wizard input is empty - ([fef31ed](https://github.com/adaouat/heraut/commit/fef31ed9ac02ae498c8f7d6244f60dac07e4717e)) by @bchatard

- *(versioning/semver)* Strip tag prefix from --version override if present - ([75d04a6](https://github.com/adaouat/heraut/commit/75d04a6d0671df9deeceb371d12a17bb192c5039)) by @bchatard


### ⚙️ Miscellaneous Tasks

- Parallelise goreleaser and docker jobs in release workflow - ([15ac51f](https://github.com/adaouat/heraut/commit/15ac51ff14aba0ab44208fdeefa7b51d34019a62)) by @bchatard

- Fix docker build cache miss on tag-triggered releases - ([193b685](https://github.com/adaouat/heraut/commit/193b685621d9f17b92f855ab7a0440abdf21cb6c)) by @bchatard

- Bump mise tools and add actionlint + hadolint linters - ([29024ef](https://github.com/adaouat/heraut/commit/29024ef23be22816d9c229e027bf3cc29f2fe383)) by @bchatard

## [0.12.0](https://github.com/adaouat/heraut/compare/v0.11.2..v0.12.0) - 2026-05-28

### 🚀 Features

- *(pipeline)* Honour git config tag.gpgSign for signed tags (Option B) - ([a7dbbd7](https://github.com/adaouat/heraut/commit/a7dbbd79417507c23ee61f6fdd5d27c05955c7c8)) by @bchatard

## [0.11.2](https://github.com/adaouat/heraut/compare/v0.11.1..v0.11.2) - 2026-05-27

### ⚙️ Miscellaneous Tasks

- Fix git-cliff-action --unreleased → --latest for release notes - ([72e08e1](https://github.com/adaouat/heraut/commit/72e08e15586e498f6a1bf09414a63ad0d7850ca2)) by @bchatard

## [0.11.1](https://github.com/adaouat/heraut/compare/v0.11.0..v0.11.1) - 2026-05-27

### 🐛 Bug Fixes

- *(generators/gitcliff)* Correct git-cliff range flags per mode - ([061149c](https://github.com/adaouat/heraut/commit/061149ce599e186c6980a0e1796891e00f101759)) by @bchatard

## [0.11.0](https://github.com/adaouat/heraut/compare/v0.10.0..v0.11.0) - 2026-05-27

### 🚀 Features

- *(pipeline)* Wire StepFn progress reporter into release pipeline (T41) - ([7020d82](https://github.com/adaouat/heraut/commit/7020d82f5eac8078362b4c2b2346ae543b59ab41)) by @bchatard

- *(pipeline)* Wire StepFn progress reporter into changelog pipeline (T42) - ([0c607ac](https://github.com/adaouat/heraut/commit/0c607aceaba51c2da5299fc06d0a4c03f30374b9)) by @bchatard

- *(ui)* Add Progress step runner and StepFn type (T40) - ([b55e0b3](https://github.com/adaouat/heraut/commit/b55e0b316ca8b72343f28d2c559a792a2e0fcf22)) by @bchatard


### 🐛 Bug Fixes

- *(versioning/semver)* --version override works with bump: auto - ([1efacb2](https://github.com/adaouat/heraut/commit/1efacb2f9e25d6c3bab6fec1bbd5af6737041dbf)) by @bchatard


### 📚 Documentation

- *(cmd)* Clarify --version flag excludes tag prefix - ([4b64fef](https://github.com/adaouat/heraut/commit/4b64fef4d8e97abf1cc6e9687090c45efea61dc9)) by @bchatard

- *(roadmap)* Add Phase 9 — TUI Polish (T40/T41/T42) and ADR-0017 - ([b593ea5](https://github.com/adaouat/heraut/commit/b593ea5120cb6c248d97c8fa39013433772ba315)) by @bchatard

- *(roadmap)* CHECKPOINT J — TUI Polish complete - ([564893f](https://github.com/adaouat/heraut/commit/564893f8d75cf1dba77a2b35ec346dcd8e7c75aa)) by @bchatard

## [0.10.0](https://github.com/adaouat/heraut/compare/v0.9.1..v0.10.0) - 2026-05-27

### 🚀 Features

- *(cmd)* Preview config and confirm before writing on heraut init - ([5e680d3](https://github.com/adaouat/heraut/commit/5e680d36e7bfd6ac7c53bad29f5d80dd3ebfded9)) by @bchatard

- *(config)* Unified environments block (T37) - ([c8f175b](https://github.com/adaouat/heraut/commit/c8f175b8295fc0f58266b34900e6e5c1be8a0ecd)) by @bchatard


### 🐛 Bug Fixes

- *(pipeline)* Wrap gitCommitChangelog error with context - ([a6c3ec7](https://github.com/adaouat/heraut/commit/a6c3ec7c170f670c76c8f56de2882d345b50410c)) by @bchatard


### 🚜 Refactor

- *(cmd)* Rename HERAUT_NO_UPDATE_CHECK to HERAUT_CHECK_UPDATE=false - ([703bbf5](https://github.com/adaouat/heraut/commit/703bbf52844ce60b2dcf11616fa3a7d796c5f20d)) by @bchatard

- *(pipeline)* Extract shared git helpers to gitHelper struct - ([a62e346](https://github.com/adaouat/heraut/commit/a62e34696f302b19fd9f6a5e629d58ac27eca218)) by @bchatard

- *(platforms)* Extract resolveGlobs to shared platforms package - ([1444b8d](https://github.com/adaouat/heraut/commit/1444b8da4d60b16ae01f1f7beb7151e12fb73c53)) by @bchatard

- *(selfupdate)* Drop Gatekeeper quarantine removal; document at install - ([f1ccb34](https://github.com/adaouat/heraut/commit/f1ccb34343c77e1ce4a95e6ffbd289016339293c)) by @bchatard


### 📚 Documentation

- *(plans)* Add Azure DevOps platform effort evaluation - ([66f5651](https://github.com/adaouat/heraut/commit/66f5651b1cadd6d29b94fda8bb96cd5dade5d06d)) by @bchatard

- *(roadmap)* Add T39 — coverage sweep for cmd/release and cmd/changelog - ([e687b4d](https://github.com/adaouat/heraut/commit/e687b4d9ba05c08fc57b7fb3f99d9674ef09f40e)) by @bchatard

- Spec and task for unified environments block (T37) - ([ead2d17](https://github.com/adaouat/heraut/commit/ead2d17589fb6225008d72b207b72c8cb70cd0c1)) by @bchatard

- Update environments table description to reflect T37 unified block - ([8687fa4](https://github.com/adaouat/heraut/commit/8687fa4935abd0b8b0adab9dd68f81587c147098)) by @bchatard


### 🧪 Testing

- *(cmd)* Assert version next returns Runtime exit code when no new commits - ([86c624a](https://github.com/adaouat/heraut/commit/86c624aae3e5e88be79309ee17f452663c6cf617)) by @bchatard

- *(cmd)* T39 — coverage sweep for release, changelog, and check cliff - ([fe78dc4](https://github.com/adaouat/heraut/commit/fe78dc451f8572c07ddb4bb88a59c175cf2c8043)) by @bchatard


### ⚙️ Miscellaneous Tasks

- *(ci)* Bump mise 2026.5.6→2026.5.15 and glab 1.97.0→1.99.0 in Dockerfile - ([e552631](https://github.com/adaouat/heraut/commit/e552631a2c830f4f78635cfe0dcc67e6d46e115a)) by @bchatard

- Add Renovate config for automated dependency updates - ([f7cf294](https://github.com/adaouat/heraut/commit/f7cf294e99910698bafd734e168d4f303e8c04bf)) by @bchatard

- Pin all GitHub Actions to commit SHAs - ([0380ed4](https://github.com/adaouat/heraut/commit/0380ed4e1b4a2878a30b5b44baa70efc5c2b0983)) by @bchatard

- Add govulncheck to lint job and weekly OSV scan workflow - ([1b6ec0e](https://github.com/adaouat/heraut/commit/1b6ec0eb92e83454a68369f28e25c292aa9b94ad)) by @bchatard

## [0.9.1](https://github.com/adaouat/heraut/compare/v0.9.0..v0.9.1) - 2026-05-26

### 🐛 Bug Fixes

- *(platforms)* Token forwarding, auth check, glob dirs, gitlab upload command - ([18f16c8](https://github.com/adaouat/heraut/commit/18f16c82512c93e3110697c20b9af53ce3eb7c87)) by @bchatard

- *(platforms/gitlab)* Remove catalog field — GitLab publishes automatically - ([9bbfe00](https://github.com/adaouat/heraut/commit/9bbfe0049763faa44e44c60c4e8117096abac6d2)) by @bchatard

- *(scaffold)* Skip platform wizard when no release notes, default token env vars - ([fe4e643](https://github.com/adaouat/heraut/commit/fe4e643e0e9d9cc98db2d5c8e353dc92240fe811)) by @bchatard


### 📚 Documentation

- *(specs)* Spec changelog.env / release.notes.env, plan EnvOverride fate - ([1dfecaa](https://github.com/adaouat/heraut/commit/1dfecaaa9cb1aefd427e28fd61fa8569344fe9a7)) by @bchatard

- *(specs)* Drop changelog.env/notes.env in favour of disable_changelog/disable_notes - ([b1935c9](https://github.com/adaouat/heraut/commit/b1935c9bd7478a0a42ff48612e17e6adda5e9872)) by @bchatard

- Add annotated sample config and maintenance rule for schema/sample sync - ([6f1a159](https://github.com/adaouat/heraut/commit/6f1a1594b6a20acee6eb4bfc3796c71e72734eb2)) by @bchatard

- Document Docker image tag formats in README - ([e092a0a](https://github.com/adaouat/heraut/commit/e092a0aa320cebf0d9ffc87d414eba4a01aa3e2b)) by @bchatard

## [0.9.0](https://github.com/adaouat/heraut/compare/v0.8.0..v0.9.0) - 2026-05-26

### 🚀 Features

- *(pipeline)* Configurable git tag type — annotated (default) or lightweight - ([b3098d2](https://github.com/adaouat/heraut/commit/b3098d2ab58b608546cc9e5b96d8af55c01c7847)) by @bchatard

- *(ui/check)* Streaming spinners, section headers, resolved values, working tree - ([13f27b3](https://github.com/adaouat/heraut/commit/13f27b348ade47b021969a7eaf76f17886416224)) by @bchatard


### 🐛 Bug Fixes

- *(cmd)* Use real runner for resolver in dry-run mode - ([d3bc60f](https://github.com/adaouat/heraut/commit/d3bc60f8ecf740634d3d2bea7bf766b5b71ff91a)) by @bchatard


### 📚 Documentation

- *(roadmap)* Add Phase 8 — Stable Release Preparation (T28/T33/T34/T35) - ([349b43b](https://github.com/adaouat/heraut/commit/349b43b686dee2e4729a60d725abe13a23599595)) by @bchatard

- Add mise install, fix dry-run spec for resolver reads - ([cc654af](https://github.com/adaouat/heraut/commit/cc654afeb19239ea41bc6b8b4061f4e81758b1f9)) by @bchatard


### 🧪 Testing

- *(coverage)* Coverage sweep T34 — raise CI gate to 80% - ([54e3428](https://github.com/adaouat/heraut/commit/54e3428096d3da9ad8e75e10893c642012802a83)) by @bchatard


### ⚙️ Miscellaneous Tasks

- *(config)* Bootstrap heraut's own .config/heraut.yml and initial CHANGELOG.md - ([c09669f](https://github.com/adaouat/heraut/commit/c09669f842432d1e3afe64b254ee794a98d06396)) by @bchatard

- Split CI into parallel lint/test/build jobs with coverage gate - ([6a0c831](https://github.com/adaouat/heraut/commit/6a0c831e7edf250dbf69a529ccf584ccb3cb0497)) by @bchatard

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

