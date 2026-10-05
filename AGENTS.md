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

- Go: stdlib primeiro. Única dependência Go de runtime: `modernc.org/sqlite`. Dependência nova só com aprovação.
- Versões exatas e estáveis (sem `^`, `latest`, `rc`, `beta`).
- Lint/format Go: `golangci-lint` v2 (`gofumpt` + `goimports`). Indentação: tab, largura 4 (`.editorconfig`).
- Conventional Commits em inglês; SemVer a partir de `0.0.1`; `CHANGELOG.md` no formato Keep a Changelog.
- Release local com `gh`; nunca criar `.github/workflows/`.
- Paralelize tarefas independentes com git worktrees (ver `docs/workflow.md`).
- Faça push para `origin/main` ao concluir cada grupo de microtarefas.
- Ao mudar convenção, comando ou arquitetura: atualize `docs/`, este arquivo e `CLAUDE.md`.
- Após mudanças de código: `graphify update .`.
