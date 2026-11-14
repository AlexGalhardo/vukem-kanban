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

- Remote: `origin` = `https://github.com/AlexGalhardo/vukem-kanban.git`. Faça push para `origin/main` ao fim de cada grupo de microtarefas concluído, para salvar o trabalho.

## Worktrees e paralelização

Use git worktrees para paralelizar trabalho independente entre agentes/sessões quando isso acelerar a entrega.

Quando usar:

- Duas ou mais microtarefas do `PLAN.md` sem dependência entre si e sem arquivos em comum (ex.: testes E2E de uma feature pronta + docs + nova fatia vertical em outro pacote).
- Spikes/experimentos arriscados que não devem sujar a árvore principal.

Quando não usar:

- Migrações de banco (`database/migrations/sql/`): sequenciais, sempre na árvore principal, uma por vez.
- Tarefas que mudam contrato compartilhado (rotas, schema, `layout.html`, `package.json`): defina o contrato primeiro na `main`, depois paralelize.
- Tarefas XS/S que terminam mais rápido do que o custo de criar e mesclar a worktree.

Como:

1. Crie uma worktree por tarefa: `git worktree add ../vukem-kanban-<slug> -b feat/<slug>` (ou ferramenta `EnterWorktree` / `Agent` com `isolation: "worktree"` no Claude Code).
2. Cada worktree roda seus próprios `bun install`, lint e testes. Use porta `0` e DB temporário para não colidir com outras worktrees.
3. Ao concluir: lint + testes verdes na worktree, commit convencional, merge na `main` (preferir `--ff-only` após rebase), rodar a suíte focada de novo na `main`.
4. Remova: `git worktree remove ../vukem-kanban-<slug>` e `git branch -d feat/<slug>`.
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

## Release local

O release é publicado localmente com `gh`; o workflow `.github/workflows/ci.yml` só valida (não publica artefatos).

`bun run release <versão> [--dry-run] [--skip-desktop]` (`scripts/release.ts`; aceita `X.Y.Z` e `X.Y.Z-alpha.N|beta.N|rc.N`):

1. Valida semver, árvore limpa, branch `main` e que a tag `vX.Y.Z` não existe localmente nem em `origin`. A versão deve ser maior que `VERSION`; igual só é aceita na primeira release (quando `v<VERSION>` ainda não tem tag).
2. Atualiza `VERSION` e `package.json` (`"version"`) e move o conteúdo de `[Unreleased]` para `## [X.Y.Z] - AAAA-MM-DD` no `CHANGELOG.md` (falha se `[Unreleased]` estiver vazio), deixando `[Unreleased]` vazio e atualizando os links de comparação do rodapé.
3. Gates (fail fast): `golangci-lint run ./...`, `go test -race ./...`, `bun run smoke`.
4. Builda Go com `CGO_ENABLED=0` em `dist/go/` para `windows/amd64`, `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`. Sem `--skip-desktop` e com `desktop/electron-builder.yml` presente, roda `bun run dist` e coleta os instaladores `.exe` de topo de `dist/desktop/` que contenham a versão no nome (sem o yml, pula com aviso).
5. Gera `dist/SHA256SUMS` (`<hash>  <arquivo>`) e `dist/RELEASE_NOTES.md` (corpo da seção do CHANGELOG + lista de downloads).
6. Commit `chore(release): vX.Y.Z` (VERSION, package.json, CHANGELOG.md), tag anotada `vX.Y.Z`, `git push origin main --follow-tags`.
7. `gh release create vX.Y.Z --title vX.Y.Z --notes-file dist/RELEASE_NOTES.md <artefatos> dist/SHA256SUMS`; versões com `-alpha`/`-beta`/`-rc` usam `--prerelease`.

Se algo falhar antes do commit, `VERSION`, `package.json` e `CHANGELOG.md` são restaurados.

`--dry-run` executa os passos 1–5 (fora de `main` só avisa), restaura os arquivos versionados ao final e apenas imprime os comandos de commit/tag/push/`gh release`. Release é ação externa: confirme com o usuário antes de publicar.

Os helpers puros (semver, rotação do CHANGELOG, links) têm testes em `scripts/release.test.ts`: `bun run test:scripts`.

## Definition of Done

- `bun run lint` limpo.
- `bun run test` verde com `-race`.
- Teste novo/atualizado cobre o comportamento.
- Nenhum `//nolint`, teste pulado ou erro ignorado sem justificativa.
- `CHANGELOG.md` atualizado.
- Checklist detalhado: `.claude/references/definition-of-done.md`.
