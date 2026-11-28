---
title: Making a change
description: What a pull request to NSGo needs, from the same change on every backend to tests, docs and a changelog line.
group: Contributing
order: 82
---

## Before you start

Open an issue or a discussion for anything larger than a fix, so the design is agreed before the code. Questions the design leaves open are listed in [ARCHITECTURE.md](https://github.com/nsgo-dev/nsgo/blob/main/ARCHITECTURE.md#open-questions); their answers are decided there first.

## Every backend

A fix or a feature in one backend usually belongs in the others. Make the same change on every target that needs it, or record what is missing in the [roadmap](https://github.com/nsgo-dev/nsgo/blob/main/docs/roadmap.md). Components look and behave the same on every platform.

## Checks

Use the `nsgo` tool rather than `go vet`, `go test` or `go build`:

```sh
go tool nsgo check   # vet every package
go tool nsgo test    # every package's tests
```

Add a test for what you change: `nsgotest` for what a user sees, a package's own tests for the rest. Run the gallery on the platforms you touched ([Development setup](../contributing.md#the-gallery)).

Format Go files with `gofmt` from the Go version in `go.mod`; `gofmt -l .` lists the files that need it. CI checks every tracked Go file and fails with the names of those that are not formatted.

## The pull request

Title it as a Conventional Commit, such as `fix(linux): keep the caret after a paste`: a squash merge makes the title the commit's subject, and a check fails until it fits. The template asks for the targets the change touches, before and after screenshots of anything visible, and the checks you ran.

Run `go tool nsgo check` and `go tool nsgo test` on the commit you push, then fill the `Local CI` block in the template: that commit, the `main` commit it was based on, each command with its exit code as `→ N` (on one line or as a nested list), the result, and what a local run cannot cover. `pr-triage.yml` labels the pull request from the block, `local-ci-pass`, `local-ci-fail`, or `local-ci-skipped` when the block declares the checks skipped or is missing, incomplete, names another commit or declares `pass` with a non-zero exit code; a bot comment lists what makes a block invalid, and a maintainer audits a `local-ci-skipped` or `local-ci-fail` pull request before merging it. Pull requests from bots such as Dependabot get no label. The declaration is a promise, not evidence: required checks still decide the merge.

## Docs and the changelog

- Update the document whose facts change: a subject in `docs/architecture/` for how NSGo works, a page in `docs/public/` for how apps and components use it ([Docs and the website](docs.md)).
- Keep `README.md` and `README.zh-CN.md` equivalent; everything else is in English, with Chinese pages in `docs/public/zh/`.
- Record a change users can see as one line, with a reference such as `(#12 by @you)`, in a file of its own in `changelog.d/` under its type's heading, such as `### Fixed`, rather than in `CHANGELOG.md`, which a release fills from those files ([format](https://github.com/nsgo-dev/nsgo/blob/main/changelog.d/README.md)).
