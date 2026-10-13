# Guides

Worked examples for specific heraut use cases.

| Guide | Use case |
|---|---|
| [Mobile / CI multi-build tagging](mobile-ci-tagging.md) | Per-env tags with CI build IDs (`{env}/{version}+{build}`) |
| [Template customization](template-customization.md) | Overriding changelog/release-notes templates — blocks, global/per-driver/per-env scoping, full template files |
| [Release pipeline and hook positions](release-pipeline-and-hooks.md) | Diagrams of the `release`/`changelog` pipelines showing exactly where each of the six hook points fires |
| [Maintenance branches](maintenance-branches.md) | Hotfixes on long-lived `release/x.y` lines: `versioning.branches`, range caps, forward-merge workflow, fixed-version branches |
| [Release notes for an app store](store-release-notes.md) | `heraut notes`: the notes of one tag as Markdown or plain text, undecorated, for a store submission |
| [End-to-end tests](e2e-tests.md) | Running and setting up the hermetic and forge-sandbox e2e lanes |
