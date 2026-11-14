# Contributing to vukem-kanban

Thanks for helping. This guide covers setup, workflow and the rules every change follows.

## Setup

1. Install Go (see `go.mod`), Bun 1.4+ and `golangci-lint` 2.x.
2. Clone and start the dev server:

   ```sh
   git clone https://github.com/AlexGalhardo/vukem-kanban.git
   cd vukem-kanban
   bun install --frozen-lockfile
   bun run dev          # Tailwind in watch mode + server on http://127.0.0.1:8080
   ```

## Workflow

1. Pick a task from [PLAN.md](PLAN.md), or open an issue first for anything not on it.
2. Branch from `main`: `feat/<short-name>`, `fix/<short-name>`, `docs/<short-name>`.
3. Write the test first, then the code. Keep changes small and focused.
4. Run the checks that cover your change (below), then open a pull request against `main`.
5. The PR title must be a Conventional Commit. `main` accepts changes only through a pull request with green required checks and linear history.

## Checks

```sh
bun run lint         # golangci-lint (gofumpt + goimports) + tsc
bun run test         # go test -race ./...
bun run smoke        # real binary: healthz, pages, static files, headers
bun run e2e          # Playwright, web
bun run e2e:desktop  # Playwright + Electron
```

Run the specific test, lint and typecheck that cover your change; CI runs the full suite.

## Commit messages

[Conventional Commits 1.0](https://www.conventionalcommits.org/en/v1.0.0/):

```text
feat(board): drag cards between columns with the keyboard
fix(store)!: keep card IDs stable after a column is deleted

BREAKING CHANGE: exported JSON now carries the card prefix.
```

Types: `feat`, `fix`, `perf`, `refactor`, `docs`, `test`, `build`, `ci`, `chore`, `style`, `revert`. Scope is the area (`board`, `store`, `cmd`, `desktop`, `docker`, `ci`, `deps`, `docs`).

## Code rules

- Comments explain *why*, never *what*. Fix root causes; never silence an error or skip a test.
- Outside the vukem framework, use the standard library first. No new dependency without approval; pin exact, stable versions.
- Card descriptions render only through `internal/markdown`, which escapes everything; never use `template.HTML` with user text.
- htmx errors answer with `HX-Reswap: none` and an `X-Toast` header (see [docs/architecture.md](docs/architecture.md)).

## Changelog and releases

Release notes follow [Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/) in [CHANGELOG.md](CHANGELOG.md): add entries under `[Unreleased]` in the same change. Versioning follows [SemVer](https://semver.org); tags are `vX.Y.Z` on `main`.

## Code of conduct

This project follows the [Contributor Covenant](CODE_OF_CONDUCT.md). Report security issues privately, as described in [SECURITY.md](SECURITY.md).
