# Changelog fragments

A pull request with a change users can see records it here, in a file of its own, instead of in `CHANGELOG.md`, so pull requests no longer conflict over the top of `Unreleased`. Name the file after the change in kebab case, such as the branch's name (`keep-caret-after-paste.md`), and put each one-clause bullet, with its reference, under the Keep a Changelog type it is:

```markdown
### Fixed

- Keep the caret after a paste on Linux (#12 by @you).
```

The types are `Breaking`, `Added`, `Changed`, `Deprecated`, `Removed`, `Fixed` and `Security`; a fragment may hold several. A change that makes apps edit their code, `nsgo.yaml` or commands to keep building or behaving as before is also `Breaking`, in a bullet that says what breaks and how to update, which may go on in lines indented by two spaces, such as a code block of the new form:

````markdown
### Breaking

- Rename `core.Old` to `core.New`, which takes its options last (#34 by @you): replace each `core.Old(opts, x)` with

  ```go
  core.New(x, opts)
  ```
````
 `Unreleased` shows no heading per type, only the bullets in that order, then by file name; a release groups them for its notes. `go run ./tools/changelog check` validates the fragments, as `go tool nsgo test` does.

To release, run `go run ./tools/changelog release X.Y.Z` on a `chore/release-X.Y.Z` branch from an up-to-date `main`, which turns `Unreleased` and the fragments into the version's section and deletes them: `### Breaking changes` in full, an empty `### Highlights`, `### Features` and `### Bug fixes` headings, `### Security` in full, and every feature and fix in a closing `<details>` block, `More changes`, under `#### Features` and `#### Bug fixes`; when the Android runtime's Java changed since its last rebuild, it first rebuilds `classes.dex` and `classes.jar` with the `android-runtime.yml` workflow on `main` (`-no-runtime` skips it), so commit them with the changelog. Write three to five highlights in plain words, move the most notable bullets of `More changes` up under `### Features` and `### Bug fixes`, leaving the rest folded at the end, and merge it, then run the Release workflow on main with the version (`gh workflow run release.yml -f version=X.Y.Z`). It runs every check and test first and creates the tag `vX.Y.Z` only once they pass, so a failure leaves the version free; never push the tag yourself, as the Go module proxy can cache a pushed version before any check runs, and a failure then burns it for good. Once `release.yml` has published the release, remove the section from `CHANGELOG.md`, which keeps only what is not released: the notes live on [Releases](https://github.com/nsgo-dev/nsgo/releases). A branch started before fragments moves the bullets it added to `CHANGELOG.md` into one with main's copy of the tool, which the branch lacks: `git show origin/main:tools/changelog/main.go > /tmp/changelog.go && go run /tmp/changelog.go adopt -base origin/main -kind Fixed <name>`; fold the result into the commit that added the bullets (`git commit --amend` on a one-commit branch), since a rebase that still replays that commit's edit to `CHANGELOG.md` still conflicts.
