# Workflow

## Ciclo de uma tarefa

1. Escolha a próxima microtarefa não marcada em `PLAN.md`, respeitando a ordem das fases.
2. Implemente em fatia vertical: migração → repositório → handler → template → testes.
3. Valide focado: lint e teste do pacote afetado; E2E do fluxo tocado.
4. Atualize `CHANGELOG.md` (`[Unreleased]`) e, se mudou convenção/comando/arquitetura, os arquivos em `docs/`, `AGENTS.md` e `CLAUDE.md`.
5. Commit em Conventional Commits.
6. Marque `[x]` no `PLAN.md` só depois que os asserts da tarefa passarem.
7. Rode `graphify update .` após mudanças de código.

## Git

- Branch padrão: `main`. Trabalho maior: branch `feat/<slug>` ou `fix/<slug>`, merge local.
- Commits atômicos em Conventional Commits 1.0.0, mensagens em inglês:
  - `feat:` funcionalidade (bump MINOR)
  - `fix:` correção (bump PATCH)
  - `perf:`, `refactor:`, `test:`, `docs:`, `build:`, `chore:`, `style:`
  - Breaking change: `feat!:` ou rodapé `BREAKING CHANGE:` (bump MAJOR após 1.0.0)
- Escopos sugeridos: `store`, `web`, `ui`, `desktop`, `e2e`, `release`, `deps`.
- Exemplo: `feat(web): create card inline from column footer`.

- Remote: `origin` = `https://github.com/AlexGalhardo/trello-golang-htmx.git`. Faça push para `origin/main` ao fim de cada grupo de microtarefas concluído, para salvar o trabalho.

## Worktrees e paralelização

Use git worktrees para paralelizar trabalho independente entre agentes/sessões quando isso acelerar a entrega.

Quando usar:

- Duas ou mais microtarefas do `PLAN.md` sem dependência entre si e sem arquivos em comum (ex.: testes E2E de uma feature pronta + docs + nova fatia vertical em outro pacote).
- Spikes/experimentos arriscados que não devem sujar a árvore principal.

Quando não usar:

- Migrações de banco (`internal/store/migrations/`): sequenciais, sempre na árvore principal, uma por vez.
- Tarefas que mudam contrato compartilhado (rotas, schema, `layout.html`, `package.json`): defina o contrato primeiro na `main`, depois paralelize.
- Tarefas XS/S que terminam mais rápido do que o custo de criar e mesclar a worktree.

Como:

1. Crie uma worktree por tarefa: `git worktree add ../trello-golang-htmx-<slug> -b feat/<slug>` (ou ferramenta `EnterWorktree` / `Agent` com `isolation: "worktree"` no Claude Code).
2. Cada worktree roda seus próprios `bun install`, lint e testes. Use porta `0` e DB temporário para não colidir com outras worktrees.
3. Ao concluir: lint + testes verdes na worktree, commit convencional, merge na `main` (preferir `--ff-only` após rebase), rodar a suíte focada de novo na `main`.
4. Remova: `git worktree remove ../trello-golang-htmx-<slug>` e `git branch -d feat/<slug>`.
5. Marque `[x]` no `PLAN.md` e atualize `CHANGELOG.md` somente na `main`, após o merge (evita conflito nesses arquivos).

### Worktree de QA

Em paralelo ao desenvolvimento, mantenha uma worktree de QA que simula um testador humano adversarial:

- Sobe o próprio binário numa porta dedicada (ex.: `8790`) com DB temporário e usa `agent-browser`/Playwright para clicar em tudo, testar fluxos alternativos, edge cases, fallbacks (servidor fora do ar, abas concorrentes, entradas inválidas) e abuso HTTP direto.
- Não altera código de produto: entrega uma lista de bugs com severidade, passos de reprodução, esperado × obtido e evidência. Specs de reprodução opcionais ficam em `e2e/qa/`.
- O orquestrador corrige na `main` e transforma cada bug relevante em teste de regressão (Go ou E2E).

## Versionamento (SemVer 2.0.0)

- Fonte única: arquivo `VERSION`. Injetada no Go via `-ldflags "-X main.version=$(cat VERSION)"` e no Electron via `package.json` do desktop.
- Começa em `0.0.1`. Fases do `PLAN.md`:
  - ESTRUTURA: `0.0.x`
  - ALPHA: `0.1.0` … `0.4.x`, prereleases `-alpha.N`
  - BETA: `0.5.0` … `0.9.x`, prereleases `-beta.N`
  - RC: `1.0.0-rc.N` → `1.0.0`

## CHANGELOG (Keep a Changelog 1.1.0)

- Seção `[Unreleased]` no topo; categorias: `Added`, `Changed`, `Deprecated`, `Removed`, `Fixed`, `Security`.
- Entradas objetivas, voltadas ao usuário, em português.
- Na release, `[Unreleased]` vira `[x.y.z] - AAAA-MM-DD` e links de comparação no rodapé são atualizados.

## Release local (sem GitHub Actions)

GitHub Actions fica desativado (billing). Nunca criar `.github/workflows/`.

`bun run release <versão>` (`scripts/release.ts`):

1. Valida árvore git limpa e versão semver.
2. Atualiza `VERSION` e move `[Unreleased]` para a nova versão no `CHANGELOG.md`.
3. Roda `lint`, `test`, `smoke`.
4. Builda Go com `CGO_ENABLED=0` para `windows/amd64`, `linux/amd64`, `darwin/arm64` + instalador Electron Windows.
5. Gera `SHA256SUMS`.
6. Commit `chore(release): vX.Y.Z`, tag anotada `vX.Y.Z`, `git push --follow-tags`.
7. `gh release create vX.Y.Z --notes-file <seção do CHANGELOG> <artefatos>`; versões com `-alpha`/`-beta`/`-rc` usam `--prerelease`.

Use `--dry-run` para executar tudo menos push e publicação. Release é ação externa: confirme com o usuário antes de publicar.

## Definition of Done

- `bun run lint` limpo.
- `bun run test` verde com `-race`.
- Teste novo/atualizado cobre o comportamento.
- Nenhum `//nolint`, teste pulado ou erro ignorado sem justificativa.
- `CHANGELOG.md` atualizado.
- Checklist detalhado: `.claude/references/definition-of-done.md`.
