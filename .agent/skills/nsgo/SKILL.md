---
name: nsgo
description: Find the NSGo docs while working in this repository. Use when you need what NSGo is, how to build, run, test or ship an app with it, a component's API, or a target's state, and want the first-party page rather than a guess. Holds the index of every page as Markdown, which llms.txt beside it lists.
---

# NSGo docs

`llms.txt` beside this file is <https://nsgo.dev/llms.txt>: the index of every
page of NSGo's documentation as Markdown, one line a page with its URL and a
sentence on what it covers.

To read a page, take its URL from the index and add nothing: the `.md` link is
already the page's Markdown.

```sh
curl -sS https://nsgo.dev/docs/components.md
```

`https://nsgo.dev/llms-full.txt` holds every page at once, which is large:
fetch it only when a task spans many pages.

## Before relying on a signature

NSGo is early, with no stable API. The docs describe the latest release, and
this checkout may be ahead of or behind it, so read the source for the commit
at hand instead of guessing from a signature:

```sh
git grep -n 'func Name' core/     # in this checkout
go list -m -f '{{.Dir}}' github.com/nsgo-dev/nsgo   # an installed module
```

`examples/gallery` demonstrates every component, `examples/hello` is the
smallest app and `examples/tsx` a TSX one. Copy their structure rather than
inventing one. `docs/roadmap.md` says what is missing, by target: check it
before assuming a feature exists everywhere.

## This repository

Rules for changing NSGo itself are in `AGENTS.md`, and how the engine is put
together in `ARCHITECTURE.md`; neither is on the website. `skills/nsgo/` is the
skill apps install with `bunx skills add nsgo-dev/nsgo`, which is for writing
apps, not for changing the engine.