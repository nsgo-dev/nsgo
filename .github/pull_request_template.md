## Summary

<!-- What a user or an app author sees change, and why. Keep it to one change; link the issue it fixes (Fixes #123). -->

## Targets

<!-- Tick what this changes. A change one backend needs usually belongs in the others: make it there, or record the gap in docs/roadmap.md. -->

- [ ] Shared code (`core`, `base`, `ui`, `tsx`, the `nsgo` tool)
- [ ] macOS
- [ ] iOS
- [ ] tvOS
- [ ] watchOS
- [ ] Android
- [ ] Windows
- [ ] Linux
- [ ] Web
- [ ] Mini programs
- [ ] Docs or website only

## Screenshots

<!-- For anything visible, before and after images of the same view at its real size, with no personal data in them. Write "None" when nothing visible changes. -->

## Verification

<!-- Run the checks on the commit you push, and replace every value below, keeping the local-ci line: pr-triage.yml reads the block and labels this pull request. Give each command's exit code as → N; the commands may also be a nested list. A missing or incomplete block, a head that is not this pull request's head, or a pass with a non-zero exit code is labelled local-ci-skipped for a maintainer to audit. -->

<!-- local-ci -->
- head: `SHA the checks ran on`
- base: `the main commit the branch was based on`
- commands: `each command with its exit code, such as go tool nsgo check → 0`
- result: `pass | fail | skipped`
- not covered: `what a local run cannot cover, such as the Android emulator, Windows, Linux or the browser jobs`

## Checklist

- [ ] The title is a Conventional Commit, such as `fix(linux): keep the caret after a paste`; a squash merge uses it as the commit subject.
- [ ] Docs whose facts change are updated, and a capability users can see has a line in a [`changelog.d/`](../changelog.d/README.md) fragment, not in `CHANGELOG.md`.
- [ ] New third-party code, assets and dependencies name their source and license.
