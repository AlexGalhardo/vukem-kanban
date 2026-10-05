# Arquitetura

Kanban single-user, local, estilo Trello/Linear para gerenciar projetos de software.
Um binário Go serve HTML via HTMX; o Electron só embrulha esse servidor numa janela desktop.

## Visão de componentes

```text
┌──────────────── Electron (desktop/) ────────────────┐
│ main.ts: spawn sidecar → lê "LISTENING <addr>"      │
│          → BrowserWindow(http://127.0.0.1:<porta>)  │
└───────────────────────┬─────────────────────────────┘
                        │ HTTP (loopback)
┌───────────────────────▼─────────────────────────────┐
│ bin/kanban (cmd/kanban)                             │
│  net/http ServeMux → internal/web (handlers)        │
│     ├─ html/template (embed) → páginas e parciais   │
│     └─ static (embed): htmx.min.js, app.js, app.css │
│  internal/store (database/sql + modernc sqlite)     │
└───────────────────────┬─────────────────────────────┘
                        ▼
                 kanban.db (SQLite, WAL)
```

Sem Electron, o mesmo binário funciona como app web: `bin/kanban --addr 127.0.0.1:8080`.

## Decisões (ADR resumido)

| ID | Decisão | Motivo |
| --- | --- | --- |
| D1 | Stdlib primeiro; única dependência Go de runtime é o driver SQLite | Projeto didático; menos superfície |
| D2 | `modernc.org/sqlite` (pure Go) | Sem CGO: cross-compile e build Electron sem toolchain C |
| D3 | Binário único com `go:embed` (templates, CSS, JS) | Distribuição trivial; funciona offline |
| D4 | Electron como casca fina com sidecar Go | Lógica fica toda em Go; desktop só gerencia processo e janela |
| D5 | HTMX server-driven: handlers devolvem fragmentos HTML | Sem SPA, sem JSON API, sem build JS complexo |
| D6 | Drag-and-drop HTML5 nativo + JS mínimo | Evita SortableJS; `position` reindexada em transação |
| D7 | Bun como task runner (`package.json`) | `make` ausente no Windows; Bun já é necessário para Tailwind/Electron/Playwright |
| D8 | Release local via `scripts/release.ts` + `gh` | GitHub Actions desativado (billing) |
| D9 | Sem auth/CSRF; servidor só em `127.0.0.1` | App local single-user |

ADRs completos irão para `docs/adr/` na fase 1.0.0 RC (ver `PLAN.md` R5).

## Convenções HTTP/HTMX

- Rotas com padrões do Go 1.22+: `GET /boards/{id}`, `POST /columns/{id}/cards`, `POST /cards/{id}/move`.
- Requisição com header `HX-Request` → parcial. Sem header → página completa com layout. Mesma URL serve os dois.
- Navegação HTMX que muda de board usa `HX-Push-Url`.
- Erro de validação → status `422` + fragmento do formulário com mensagens inline.
- Escaping sempre via `html/template`; nunca `template.HTML` com entrada do usuário.

## Layout do código

```text
cmd/kanban/main.go           # flags (--addr, --db, --version, --log-level), wiring, shutdown gracioso
internal/store/              # Open(), migrações, repositórios
internal/store/migrations/   # NNNN_descricao.sql (embed)
internal/web/                # handlers, middleware, render
internal/web/templates/      # layout.html, partials/*.html
internal/web/static/         # htmx.min.js (vendorizado), app.js, app.css (gerado, ignorado no git)
web/styles/input.css         # entrada Tailwind v4
desktop/                     # Electron main/preload + electron-builder.yml
e2e/                         # Playwright web/ e desktop/, roteiros agent-browser
scripts/                     # smoke.ts, release.ts, vendor-htmx.ts
```

## Modelo de dados

- `boards(id, name, description, color, archived_at, created_at, updated_at)` — board = projeto de software.
- `columns(id, board_id, name, position, wip_limit, created_at)` — padrão: Backlog, A fazer, Em progresso, Revisão, Feito.
- `cards(id, column_id, title, description, priority 0-4, due_date, position, archived_at, created_at, updated_at)`.
- `labels(id, board_id, name, color)`, `card_labels(card_id, label_id)`.

Regras:

- `PRAGMA foreign_keys=ON`, `journal_mode=WAL`, `busy_timeout=5000`; FKs com `ON DELETE CASCADE`.
- `SetMaxOpenConns(1)` evita `database is locked`.
- `position` sempre contígua `0..n-1` por coluna/board; reordenação em uma transação.
- Migrações só para frente, numeradas, aplicadas em transação e registradas em `schema_migrations`. Nunca editar migração já lançada.

## Contrato Electron ↔ sidecar

1. Electron executa `kanban --addr 127.0.0.1:0 --db <userData>/kanban.db`.
2. Go imprime exatamente `LISTENING 127.0.0.1:<porta>` no stdout quando pronto.
3. Electron abre a URL; bloqueia navegação externa; `contextIsolation`, `sandbox`, sem `nodeIntegration`.
4. `before-quit` encerra o sidecar. Sidecar morto → Electron mostra erro e sai.
