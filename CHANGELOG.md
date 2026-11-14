# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **docs:** Rewrite the README in the layout of the other vukem apps (badges, summary, quick start, contributing) and add `LICENSE.md` (MIT), `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md` and `SECURITY.md`
- **docker:** Add a multi-stage non-root image (`HEALTHCHECK`, `/data` volume for SQLite) and `compose.yaml` ([82b30eb](https://github.com/AlexGalhardo/vukem-kanban/commit/82b30ebb855688df41325d8d2f1522c6de8b9513))
- **ci:** Publish the signed image to GHCR and Docker Hub on a version tag (`X.Y.Z`; `X.Y` and `latest` on stable tags) with a smoke test before the push, cosign and attestation; the GitHub and desktop release stays local (`bun run release`) ([3a19cdc](https://github.com/AlexGalhardo/vukem-kanban/commit/3a19cdc9f26a5db3df22867ceb63f84d4d89cbe4), [781ebdd](https://github.com/AlexGalhardo/vukem-kanban/commit/781ebdd37250432c26f4a9f88a29711c810cafac))
- **ci:** Add the validation workflow (Go, golangci-lint, race tests, web e2e, TypeScript and markdownlint) with actions pinned by SHA ([11f61f8](https://github.com/AlexGalhardo/vukem-kanban/commit/11f61f8f894ad9ac4187d76a052d60e2968e63c5))
- **cli:** Add the vukem commands to the binary: `kanban route:list`, `about` and `migrate:status` ([c2d19ef](https://github.com/AlexGalhardo/vukem-kanban/commit/c2d19ef91be344d7ae746c87d73f876d872a775e))
- **docs:** Add the pre-migration audit of routes, store, markdown, desktop and e2e (`docs/audit-2026-11-14.md`) ([b7afb92](https://github.com/AlexGalhardo/vukem-kanban/commit/b7afb92cbb1c13402967f1125d88cdcaca3399b0))

### Changed

- **BREAKING:** **app:** Migrate the app to the vukem v0.43.0 framework: `bootstrap/`, `routes/`, `app/controllers/`, `database/migrations/`, `resources/` and `tests/` structure, with routing, middleware, config, logs and migrations running through vukem. Screens, routes, htmx headers and the desktop contract (`--addr`, `--db`, `--log-level`, `--version`, `LISTENING` line) stay the same. The CSP now uses a nonce, the app sends `Cross-Origin-Opener-Policy: same-origin`, the body limit (1 MiB, 20 MiB on import) uses the framework `BodyLimit` and request logs follow the vukem format (`duration` instead of `duration_ms`, plus `route`, `peer`, `ip`) ([c2d19ef](https://github.com/AlexGalhardo/vukem-kanban/commit/c2d19ef91be344d7ae746c87d73f876d872a775e))
- **store:** Run the SQL migrations through the vukem migrator (`migrations` table), recognise databases from the previous `schema_migrations` table without data loss, keep the `<db>.pre-<migration>.bak` copy before migrating, generate the legacy fixture in the test and tidy `go.mod` ([375fd33](https://github.com/AlexGalhardo/vukem-kanban/commit/375fd3384fb8d71d9e7e389f6b9444855855ea95))
- **docs:** Document the vukem structure, commands and migration, and record the migration outcome and open findings in the audit ([18c913a](https://github.com/AlexGalhardo/vukem-kanban/commit/18c913a3342469df3653bdd8e8820cdc2f975d43), [f4f94db](https://github.com/AlexGalhardo/vukem-kanban/commit/f4f94db9320ee0f8aa8576cc35b52ca1e9753c14))
- **repo:** Rename the project to vukem-kanban: Go module `github.com/AlexGalhardo/vukem-kanban`, images `ghcr.io/alexgalhardo/vukem-kanban` and `alexgalhardo/vukem-kanban` (Docker Hub), repository links, `package.json` name and docs; the desktop `appId` and data directory do not change

### Fixed

- **cmd:** Reject an invalid `--log-level` instead of falling back to info ([7233648](https://github.com/AlexGalhardo/vukem-kanban/commit/7233648765e995af9caf27c68d8011c6c7e44b34))

## [0.9.0-beta.1] - 2026-10-05

### Added

- **cards:** Render Markdown descriptions with an assisted editor: Write/Preview tabs, lists and checklists that continue on Enter, Tab/Shift+Tab, Ctrl+B/Ctrl+I, safe links and the checklist progress on the card (`1/3`), using an in-house Go renderer (`internal/markdown`) with no dependencies ([b62a84e](https://github.com/AlexGalhardo/vukem-kanban/commit/b62a84e66b7d7cd53c2a582752d636d0b5ec6a75))
- **web:** Move the focused card with the keyboard: Alt+Up/Down within a column and Alt+Left/Right between columns, as an accessible alternative to dragging ([3669df8](https://github.com/AlexGalhardo/vukem-kanban/commit/3669df80af51bd0007f32ce547cd18deafad5725))
- **web:** Add a meta description to the page and `node --check` of the JS files to `bun run lint` ([3669df8](https://github.com/AlexGalhardo/vukem-kanban/commit/3669df80af51bd0007f32ce547cd18deafad5725))
- **test:** Add 20 round-2 adversarial QA regression specs (`e2e/web/qa2-*.spec.ts`) ([727f651](https://github.com/AlexGalhardo/vukem-kanban/commit/727f651b0545f0e1142b3997be3bd94efce58e25))
- **test:** Add `bun run test:perf` with performance budgets (500 cards render in about 14 ms and moving a card takes about 6 ms, against targets of 200 ms and 50 ms) and a compatibility test that opens, migrates and imports the database and export made by `v0.4.0-alpha.1` without loss ([17d5f29](https://github.com/AlexGalhardo/vukem-kanban/commit/17d5f29baabf9b6dd2fcbfe461939d50b9f6351b))
- **docs:** Add the ADRs (D1 to D9) in `docs/adr/` and the README screenshots ([17d5f29](https://github.com/AlexGalhardo/vukem-kanban/commit/17d5f29baabf9b6dd2fcbfe461939d50b9f6351b))

### Changed

- **docs:** Document the htmx 4 rules, Markdown and the validation commands in `AGENTS.md` ([0264f1e](https://github.com/AlexGalhardo/vukem-kanban/commit/0264f1ea1b56494e19d630bf8f3daaa81c0d3b4b))
- **docs:** Allow repeated changelog headings across versions in markdownlint ([dcdaaf4](https://github.com/AlexGalhardo/vukem-kanban/commit/dcdaaf4a5d22273913339462fd54505e7c29540b))
- **web:** Hash the static assets content in `?v=` to avoid stale JS and CSS in cache after an update ([3669df8](https://github.com/AlexGalhardo/vukem-kanban/commit/3669df80af51bd0007f32ce547cd18deafad5725))
- **web:** Restore the keyboard focus after saving or closing dialogs, toggling labels, creating labels and editing titles, ask for confirmation when closing a card dialog with unsaved changes and trap focus in the narrow-screen sidebar (`inert` on the content) ([3669df8](https://github.com/AlexGalhardo/vukem-kanban/commit/3669df80af51bd0007f32ce547cd18deafad5725))
- **web:** Update the tab and window title with the open project, and show "Project not found" when it was deleted in another tab ([3669df8](https://github.com/AlexGalhardo/vukem-kanban/commit/3669df80af51bd0007f32ce547cd18deafad5725))
- **web:** Stop the Labels dialog from resending the whole board when opened, keep Ctrl+K from opening the palette over another dialog, close the column menu with Esc, and raise secondary text, delete buttons, focus ring and sidebar link touch targets (24 px) to WCAG AA ([3669df8](https://github.com/AlexGalhardo/vukem-kanban/commit/3669df80af51bd0007f32ce547cd18deafad5725))

### Fixed

- **web:** Stop cutting or losing text typed right after Enter in "+ New card" and "+ New column": the field is cleared on submit and restored on error ([3669df8](https://github.com/AlexGalhardo/vukem-kanban/commit/3669df80af51bd0007f32ce547cd18deafad5725))
- **web:** Re-render the board when editing, renaming or toggling a label with an active filter, and drop the filter (and URL parameter) when the label in use is deleted ([3669df8](https://github.com/AlexGalhardo/vukem-kanban/commit/3669df80af51bd0007f32ce547cd18deafad5725))
- **cards:** Stop a stale card save in a second tab from overwriting the newer version: it returns 409 with a warning (new `revision` column, migration `0002`) ([3669df8](https://github.com/AlexGalhardo/vukem-kanban/commit/3669df80af51bd0007f32ce547cd18deafad5725))
- **web:** Return the card to its original position when dragging with the server down ([3669df8](https://github.com/AlexGalhardo/vukem-kanban/commit/3669df80af51bd0007f32ce547cd18deafad5725))
- **web:** Reject or clean titles and names made only of invisible characters (NUL, zero-width) or invalid UTF-8, and make the project key count letters instead of bytes ("Ação" becomes `AÇÃ`) ([3669df8](https://github.com/AlexGalhardo/vukem-kanban/commit/3669df80af51bd0007f32ce547cd18deafad5725))
- **server:** Return 413 for chunked request bodies over the limit instead of truncated fields, and stop simultaneous backups from colliding on the file name ([3669df8](https://github.com/AlexGalhardo/vukem-kanban/commit/3669df80af51bd0007f32ce547cd18deafad5725))
- **export:** Write label keys as `name`/`color` instead of `Name`/`Color` in the JSON export (old exports still import) ([3669df8](https://github.com/AlexGalhardo/vukem-kanban/commit/3669df80af51bd0007f32ce547cd18deafad5725))

## [0.4.0-alpha.1] - 2026-10-05

### Added

- **test:** Add failing regression specs for the adversarial QA findings (`e2e/web/qa-regressions.spec.ts`) and a desktop e2e for the full flow (create project, card, move, edit, archive) inside the Electron window ([7dcf255](https://github.com/AlexGalhardo/vukem-kanban/commit/7dcf255ce59f7dcbcd191e8edae8edcece04185e))

### Changed

- **web:** Send the reference card (`before_id`) when moving a card and return only the affected columns, with less HTML per drag on large boards ([211878b](https://github.com/AlexGalhardo/vukem-kanban/commit/211878b198874e626327ff5ce84bd03ea56848f3))
- **web:** Show error messages in Portuguese with the field name ("Título: obrigatório"), hide internal details from JSON import errors, reload the page on history back/forward, replace the history entry on text search instead of stacking one per typing pause, and adjust secondary text contrast to WCAG AA in both themes ([211878b](https://github.com/AlexGalhardo/vukem-kanban/commit/211878b198874e626327ff5ce84bd03ea56848f3))

### Fixed

- **web:** Keep the board columns after sidebar navigation or after creating or saving a project (htmx 4 extracted `#columns` as OOB) ([211878b](https://github.com/AlexGalhardo/vukem-kanban/commit/211878b198874e626327ff5ce84bd03ea56848f3))
- **web:** Fix corrupted accented toasts and toasts from dialog actions that never appeared ([211878b](https://github.com/AlexGalhardo/vukem-kanban/commit/211878b198874e626327ff5ce84bd03ea56848f3))
- **web:** Add new labels to the label filter, allow deleting a column whose cards are hidden by the filter, store the right position when dragging with an active filter, restore the board when a move fails, and keep counters and summary consistent when creating or deleting cards with a filter ([211878b](https://github.com/AlexGalhardo/vukem-kanban/commit/211878b198874e626327ff5ce84bd03ea56848f3))
- **web:** Prevent duplicate projects, cards, columns and labels on double submit (Enter or fast click) ([211878b](https://github.com/AlexGalhardo/vukem-kanban/commit/211878b198874e626327ff5ce84bd03ea56848f3))
- **web:** Make the search ignore case on accented letters ([211878b](https://github.com/AlexGalhardo/vukem-kanban/commit/211878b198874e626327ff5ce84bd03ea56848f3))
- **web:** Close the sidebar on narrow screens, close dialogs on outside click and show a visible focus on keyboard-focused cards ([211878b](https://github.com/AlexGalhardo/vukem-kanban/commit/211878b198874e626327ff5ce84bd03ea56848f3))
- **web:** Stop zeroing a non-numeric WIP limit silently, reject titles with line breaks and return 413 instead of 400 for oversized bodies ([211878b](https://github.com/AlexGalhardo/vukem-kanban/commit/211878b198874e626327ff5ce84bd03ea56848f3))
- **import:** Reject extra content after the JSON, ask for confirmation on "Replace all" with a backup first, and fix the 500 error on two backups in the same second ([211878b](https://github.com/AlexGalhardo/vukem-kanban/commit/211878b198874e626327ff5ce84bd03ea56848f3))
- **web:** Render the error page for unknown routes and invalid IDs instead of raw text ([211878b](https://github.com/AlexGalhardo/vukem-kanban/commit/211878b198874e626327ff5ce84bd03ea56848f3))
- **test:** Fix the intermittent cleanup of temporary directories in Go tests on Windows ([211878b](https://github.com/AlexGalhardo/vukem-kanban/commit/211878b198874e626327ff5ce84bd03ea56848f3))

## [0.0.1] - 2026-10-05

### Added

- **server:** Add the Go kanban server (`cmd/kanban`) with stdlib plus `modernc.org/sqlite`, embedded SQLite migrations (WAL, foreign keys, automatic backup before migrating), a Linear-style htmx 4 + Tailwind CSS v4 UI (vendored, offline) with projects, columns, cards, labels, WIP limits, drag-and-drop with a keyboard "Status" fallback, command palette (Ctrl+K), light/dark theme, search and filters kept in the URL, JSON export/import and database backup, strict CSP, CSRF protection and `slog` request logs ([b87c676](https://github.com/AlexGalhardo/vukem-kanban/commit/b87c676bc0a00bf95e742ed57696e72f1a792bfa))
- **desktop:** Add the Electron shell that runs the Go binary as a sidecar on a random port, with `contextIsolation` and `sandbox`, blocked external navigation, single instance, persisted window state, a pt-BR native menu and rotated sidecar logs (`KANBAN_DATA_DIR` overrides the data folder) ([76d496e](https://github.com/AlexGalhardo/vukem-kanban/commit/76d496e11de0d41cde6f41c5b37def7aa8f6a9e4))
- **desktop:** Package the Windows NSIS installer and portable executable with electron-builder ([0e14002](https://github.com/AlexGalhardo/vukem-kanban/commit/0e14002a4d9df46326e283bf5ba84f2a9c9acfa6))
- **release:** Add the local release script (`bun run release <version> [--dry-run] [--skip-desktop]`) with unit-tested helpers: version bump, CHANGELOG rotation, gates, Go binaries for five platforms, optional desktop installer, `SHA256SUMS`, tag and `gh release create` ([4286bd8](https://github.com/AlexGalhardo/vukem-kanban/commit/4286bd88433c9fa1ac0086f1df2da4491d81dc62))
- **smoke:** Add the binary smoke test (`bun run smoke`) that checks `--version`, `/healthz`, `/`, static files, security headers and board creation ([5f1c7b2](https://github.com/AlexGalhardo/vukem-kanban/commit/5f1c7b2a7706d50c988a14ce9d81359d4ab2ab81))
- **test:** Cover web handlers and the server lifecycle with Go integration tests run with `-race`, and add the shared Playwright config and e2e server launcher ([399dc79](https://github.com/AlexGalhardo/vukem-kanban/commit/399dc794e2acd8f7206267bfbb7c35e26d653adc), [94cd065](https://github.com/AlexGalhardo/vukem-kanban/commit/94cd065490c397939f586c2b6fa247aec0df595b))
- **test:** Cover the desktop sidecar lifecycle, log rotation and single instance with e2e specs ([506361c](https://github.com/AlexGalhardo/vukem-kanban/commit/506361c3bda890e1932b8a0b72f124c0168305e0))
- **docs:** Add the README, the agent-browser smoke script and the htmx 4 conventions ([a41c53b](https://github.com/AlexGalhardo/vukem-kanban/commit/a41c53ba0025468d220456976cbfd10366ee279e))
- **docs:** Document the Electron shell, commands and progress ([121948d](https://github.com/AlexGalhardo/vukem-kanban/commit/121948d47bd615f19929dee0c9edf68563cc194e))

### Fixed

- **web:** Adapt the UI to htmx 4 semantics and add the Playwright web e2e suite with 22 scenarios ([e6af116](https://github.com/AlexGalhardo/vukem-kanban/commit/e6af116fc9584c5d947912e2c0a90ec3bea3201f))

[Unreleased]: https://github.com/AlexGalhardo/vukem-kanban/compare/v0.9.0-beta.1...HEAD
[0.9.0-beta.1]: https://github.com/AlexGalhardo/vukem-kanban/compare/v0.4.0-alpha.1...v0.9.0-beta.1
[0.4.0-alpha.1]: https://github.com/AlexGalhardo/vukem-kanban/compare/v0.0.1...v0.4.0-alpha.1
[0.0.1]: https://github.com/AlexGalhardo/vukem-kanban/releases/tag/v0.0.1
