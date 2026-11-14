# AGENTS.md

Kanban single-user estilo Trello/Linear para gerenciar projetos de software. Go + SQLite + HTMX + Tailwind v4, empacotado como desktop com Electron. Sem autenticação; `/` abre direto no dashboard.

## Comece aqui

1. `PLAN.md` — plano mestre por fases (ESTRUTURA, ALPHA, BETA, 1.0.0 RC) com microtarefas `[ ]` e asserts de qualidade. Pegue a próxima tarefa não marcada.
2. Documentação em `docs/`:
   - [docs/architecture.md](docs/architecture.md) — componentes, decisões (ADR resumido), convenções HTTP/HTMX, modelo de dados, contrato Electron ↔ sidecar Go.
   - [docs/tooling.md](docs/tooling.md) — versões fixadas, comandos `bun run *`, lint/format Go, Tailwind, HTMX.
   - [docs/workflow.md](docs/workflow.md) — ciclo de tarefa, git, Conventional Commits, worktrees e paralelização, SemVer, CHANGELOG, release local, Definition of Done.
   - [docs/testing.md](docs/testing.md) — camadas de teste (unit, integração, smoke, E2E web/desktop, agent-browser), regras e metas de cobertura.
   - [docs/agent-skills.md](docs/agent-skills.md) — skills locais em `.claude/skills/` e quando usar.

## Regras essenciais

- Go: app vukem (`github.com/AlexGalhardo/vukem`, estrutura `bootstrap/`, `routes/`, `app/controllers/`, `database/migrations/`, `resources/`). Fora do framework, stdlib primeiro; dependência nova só com aprovação.
- Versões exatas e estáveis (sem `^`, `latest`, `rc`, `beta`).
- Lint/format Go: `golangci-lint` v2 (`gofumpt` + `goimports`). Indentação: tab, largura 4 (`.editorconfig`).
- Conventional Commits em inglês; SemVer a partir de `0.0.1`; `CHANGELOG.md` no formato Keep a Changelog.
- Release local com `gh`. CI em `.github/workflows/ci.yml` (actions fixadas por SHA); confira `gh run list` após o push.
- Paralelize tarefas independentes com git worktrees (ver `docs/workflow.md`).
- Faça push para `origin/main` ao concluir cada grupo de microtarefas.
- htmx 4.0.0: herança só com `:inherited`; erros de htmx respondem com `HX-Reswap: none` + header `X-Toast` (JSON ASCII); nunca coloque um elemento `hx-swap-oob` dentro do conteúdo principal da mesma resposta. Detalhes em `docs/architecture.md`.
- Descrições de card são Markdown renderizado por `internal/markdown` (stdlib, escapa tudo); nunca use `template.HTML` com texto do usuário fora dele.
- Atualize os checklists do `PLAN.md` antes de cada commit/push.
- Rode uma worktree de QA adversarial em paralelo às fases (ver `docs/workflow.md`) e transforme bugs em testes de regressão.
- Rotas novas: registre em `app/controllers/routes.go`; handlers de escrita passam por `k.form(...)`. Views são blocos de `resources/views/partials/` renderizados pela página-host `kanban`. Migrações: SQL em `database/migrations/sql/` + `registry.go`.
- Validação: `bun run lint`, `bun run test`, `bun run test:perf`, `bun run smoke`, `E2E_PORT=<porta livre> bun run e2e` e `bun run e2e:desktop`.
- Ao mudar convenção, comando ou arquitetura: atualize `docs/`, este arquivo e `CLAUDE.md`.
- Após mudanças de código: `graphify update .`.
