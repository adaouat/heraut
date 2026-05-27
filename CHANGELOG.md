# Changelog

## [0.10.0](//compare/v0.9.1..v0.10.0) - 2026-05-27

### 🚀 Features

- *(cmd)* Preview config and confirm before writing on heraut init - ([5e680d3](//commit/5e680d36e7bfd6ac7c53bad29f5d80dd3ebfded9))

- *(config)* Unified environments block (T37) - ([c8f175b](//commit/c8f175b8295fc0f58266b34900e6e5c1be8a0ecd))


### 🐛 Bug Fixes

- *(pipeline)* Wrap gitCommitChangelog error with context - ([a6c3ec7](//commit/a6c3ec7c170f670c76c8f56de2882d345b50410c))


### 🚜 Refactor

- *(cmd)* Rename HERAUT_NO_UPDATE_CHECK to HERAUT_CHECK_UPDATE=false - ([703bbf5](//commit/703bbf52844ce60b2dcf11616fa3a7d796c5f20d))

- *(pipeline)* Extract shared git helpers to gitHelper struct - ([a62e346](//commit/a62e34696f302b19fd9f6a5e629d58ac27eca218))

- *(platforms)* Extract resolveGlobs to shared platforms package - ([1444b8d](//commit/1444b8da4d60b16ae01f1f7beb7151e12fb73c53))

- *(selfupdate)* Drop Gatekeeper quarantine removal; document at install - ([f1ccb34](//commit/f1ccb34343c77e1ce4a95e6ffbd289016339293c))


### 📚 Documentation

- *(plans)* Add Azure DevOps platform effort evaluation - ([66f5651](//commit/66f5651b1cadd6d29b94fda8bb96cd5dade5d06d))

- *(roadmap)* Add T39 — coverage sweep for cmd/release and cmd/changelog - ([e687b4d](//commit/e687b4d9ba05c08fc57b7fb3f99d9674ef09f40e))

- Spec and task for unified environments block (T37) - ([ead2d17](//commit/ead2d17589fb6225008d72b207b72c8cb70cd0c1))

- Update environments table description to reflect T37 unified block - ([8687fa4](//commit/8687fa4935abd0b8b0adab9dd68f81587c147098))


### 🧪 Testing

- *(cmd)* Assert version next returns Runtime exit code when no new commits - ([86c624a](//commit/86c624aae3e5e88be79309ee17f452663c6cf617))

- *(cmd)* T39 — coverage sweep for release, changelog, and check cliff - ([fe78dc4](//commit/fe78dc451f8572c07ddb4bb88a59c175cf2c8043))


### ⚙️ Miscellaneous Tasks

- *(ci)* Bump mise 2026.5.6→2026.5.15 and glab 1.97.0→1.99.0 in Dockerfile - ([e552631](//commit/e552631a2c830f4f78635cfe0dcc67e6d46e115a))

- Add Renovate config for automated dependency updates - ([f7cf294](//commit/f7cf294e99910698bafd734e168d4f303e8c04bf))

- Pin all GitHub Actions to commit SHAs - ([0380ed4](//commit/0380ed4e1b4a2878a30b5b44baa70efc5c2b0983))

- Add govulncheck to lint job and weekly OSV scan workflow - ([1b6ec0e](//commit/1b6ec0eb92e83454a68369f28e25c292aa9b94ad))

