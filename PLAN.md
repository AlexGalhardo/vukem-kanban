# PLAN.md — Kanban de Projetos (Go + HTMX + SQLite + Electron)

> Documento principal de planejamento. Toda implementação parte daqui.
> Marque `[x]` somente quando o item e seus **asserts de qualidade** estiverem verificados (comando rodado, resultado conferido).

## 1. Visão geral

Aplicação web/desktop simplificada no estilo Trello/Linear para gerenciar o desenvolvimento dos meus projetos de software.
A raiz (`/`) abre direto no dashboard: barra lateral com os projetos (boards) e o quadro kanban do projeto selecionado.
Sem autenticação, sem páginas extras. Uso single-user, local.

**Fora de escopo (não implementar):** autenticação/usuários, multi-tenant, páginas de marketing/landing, sincronização em nuvem, GitHub Actions.

## 2. Stack e versões fixadas

Versões estáveis verificadas em 2026-10-05. Fixar exatamente (sem `^`/`~`).

| Peça | Escolha | Versão |
| --- | --- | --- |
| Linguagem backend | Go | 1.27.0 |
| HTTP / roteamento | `net/http` (`ServeMux` com padrões `GET /boards/{id}`) | stdlib |
| Templates | `html/template` + `embed` | stdlib |
| Logs | `log/slog` | stdlib |
| Banco | SQLite via `modernc.org/sqlite` (pure Go, sem CGO) + `database/sql` | v1.60.1 |
| Migrações | runner próprio: `embed` + tabela `schema_migrations` | stdlib |
| Lint/format Go | `golangci-lint` v2 (linters + formatters `gofumpt` e `goimports`) | 2.14.0 |
| Frontend | HTMX (arquivo vendorizado, sem CDN — app precisa funcionar offline) | 2.0.11 |
| CSS | Tailwind CSS v4 via `@tailwindcss/cli` | 4.3.3 |
| Desktop | Electron + electron-builder | 44.5.1 / 26.15.3 |
| E2E | `@playwright/test` (web e Electron via `_electron`) | 1.63.0 |
| QA exploratório | `agent-browser` | 0.38.2 |
| Runner JS/tarefas | Bun | 1.4.2 |

## 3. Decisões de arquitetura

- **D1 — Stdlib primeiro (didático).** Única dependência Go de runtime: driver SQLite. Roteamento, templates, logs, testes, embed e migrações com stdlib.
- **D2 — `modernc.org/sqlite` em vez de `mattn/go-sqlite3`.** Sem CGO: cross-compile trivial para Windows/Linux/macOS e build do Electron sem toolchain C.
- **D3 — Binário único.** Templates, CSS gerado e `htmx.min.js` embutidos via `go:embed`. O binário roda sozinho como app web.
- **D4 — Electron como casca fina.** O processo main do Electron sobe o binário Go como sidecar (`--addr 127.0.0.1:0 --db <userData>/kanban.db`), lê a porta impressa no stdout (`LISTENING <addr>`) e abre `BrowserWindow` nessa URL. Ao fechar a janela, encerra o sidecar.
- **D5 — HTMX server-driven.** Handlers retornam fragmentos HTML. Requisição com header `HX-Request` recebe parcial; sem ele, página completa (navegação direta funciona).
- **D6 — Drag-and-drop nativo.** HTML5 DnD + ~50 linhas de JS que chamam `htmx.ajax('POST', '/cards/{id}/move', ...)`. Sem SortableJS. Reordenação por `position INTEGER` reindexada dentro de transação.
- **D7 — Bun como task runner.** `package.json` concentra scripts (`dev`, `build`, `test`, `lint`, `e2e`, `release`). Scripts complexos em `scripts/*.ts` rodados com `bun`. Evita `make` (ausente no Windows).
- **D8 — Release local.** `scripts/release.ts` gera builds, extrai a seção da versão do `CHANGELOG.md` e publica com `gh release create`. Nenhum workflow em `.github/workflows/`.
- **D9 — Sem CSRF/auth.** App local single-user; servidor escuta só em `127.0.0.1`. Validação de entrada e escaping (`html/template`) continuam obrigatórios.

## 4. Estrutura de diretórios alvo

```text
cmd/kanban/main.go           # flags, wiring, ListenAndServe
internal/store/              # database/sql, migrações, repositórios (boards, columns, cards, labels)
internal/store/migrations/   # 0001_init.sql, ...
internal/web/                # handlers, roteamento, render
internal/web/templates/      # layout.html, partials/*.html
internal/web/static/         # htmx.min.js, app.js (DnD), app.css (gerado)
web/styles/input.css         # entrada do Tailwind v4
desktop/                     # Electron: main.ts, preload.ts, electron-builder.yml
e2e/                         # Playwright (web/ e desktop/)
scripts/                     # smoke.ts, release.ts, vendor-htmx.ts
```

## 5. Modelo de dados (v1)

- `boards(id, name, description, color, archived_at, created_at, updated_at)` — um board = um projeto de software.
- `columns(id, board_id FK, name, position, wip_limit NULL, created_at)` — colunas padrão ao criar board: Backlog, A fazer, Em progresso, Revisão, Feito.
- `cards(id, column_id FK, title, description, priority 0-4, due_date NULL, position, archived_at, created_at, updated_at)`.
- `labels(id, board_id FK, name, color)` e `card_labels(card_id, label_id)`.
- `ON DELETE CASCADE`, `PRAGMA foreign_keys=ON`, `journal_mode=WAL`, `busy_timeout=5000`.

## 6. Comandos do projeto (contrato)

| Comando | Faz |
| --- | --- |
| `bun run dev` | Tailwind em watch + `go run ./cmd/kanban` |
| `bun run build` | CSS minificado + `go build -o bin/kanban ./cmd/kanban` |
| `bun run lint` | `golangci-lint run ./...` + `golangci-lint fmt --diff` |
| `bun run test` | `go test -race ./...` (unit + integração) |
| `bun run smoke` | builda, sobe binário, checa `/healthz` e `/`, derruba |
| `bun run e2e` | Playwright contra servidor real com DB temporário |
| `bun run e2e:desktop` | Playwright `_electron` contra app empacotado |
| `bun run desktop` | Electron em modo dev |
| `bun run dist` | electron-builder |
| `bun run release <versão>` | build + tag + `gh release create` com notas do CHANGELOG |

## 7. Definition of Done global (vale para toda microtarefa)

- [ ] `bun run lint` sem erros
- [ ] `bun run test` verde (com `-race`)
- [ ] Teste novo ou atualizado cobre o comportamento entregue
- [ ] Nenhum `//nolint`, teste pulado ou erro ignorado sem justificativa escrita
- [ ] `CHANGELOG.md` → seção `[Unreleased]` atualizada
- [ ] Commit em Conventional Commits (`feat:`, `fix:`, `test:`, `chore:`, `docs:`, `refactor:`, `build:`)

---

## FASE 1 — ESTRUTURA (`0.0.x`)

Objetivo: repositório, ferramentas, esqueleto executável em todas as camadas (Go → HTML → Electron → testes → release). Nada de feature ainda. Riscos técnicos (sidecar Electron, embed, release local) validados aqui.

### E1. Skills e contexto do agente

- [x] Skill `graphify` instalada em `.claude/skills/graphify` (`graphify install --project --platform windows`)
- [x] Skill `karpathy-guidelines` em `.claude/skills/karpathy-guidelines`
- [x] Skills `ponytail*` em `.claude/skills/`
- [x] Skills `addyosmani/agent-skills` em `.claude/skills/` + `references/` em `.claude/references/`
- [x] `CLAUDE.md` e `AGENTS.md` do projeto descrevem stack, comandos (seção 6) e convenções + índice `docs/` (architecture, tooling, workflow, testing, agent-skills)
- **Asserts:** `ls .claude/skills` lista todas; links `../../references/*.md` das skills resolvem.

### E2. Repositório e convenções

- [x] `git init`, branch `main`, remote `origin` = `https://github.com/AlexGalhardo/trello-golang-htmx.git`
- [x] `.editorconfig`: `indent_style = tab`, `indent_size = 4`, `tab_width = 4`, `end_of_line = lf`, `charset = utf-8`, `insert_final_newline = true`; exceção `*.{yml,yaml,md}` com espaços (YAML proíbe tab)
- [x] `.gitattributes` com `* text=auto eol=lf`
- [x] `.gitignore` (bin/, dist/, node_modules/, *.db, `internal/web/static/app.css`, graphify-out/, test-results/, playwright-report/)
- [x] `CHANGELOG.md` no formato Keep a Changelog 1.1.0 com `[Unreleased]`
- [ ] `VERSION` contendo `0.0.1` (fonte única da versão; injetada via `-ldflags "-X main.version=..."` e lida pelo `package.json` do desktop)
- [ ] `README.md` curto: o que é, como rodar, comandos
- [x] Commit inicial `chore: bootstrap repository`
- [x] Paralelização com git worktrees documentada e seguida (`docs/workflow.md`)
- **Asserts:** `git log --oneline` mostra commit convencional; editor abre `.go` com tab de largura 4.

### E3. Toolchain Go

- [ ] `go mod init github.com/AlexGalhardo/trello-golang-htmx` com `go 1.27.0`
- [ ] `.golangci.yml` (v2): linters `errcheck`, `govet`, `staticcheck`, `unused`, `ineffassign`, `gosec`, `revive`, `bodyclose`, `sqlclosecheck`, `rowserrcheck`, `errorlint`, `misspell`; formatters `gofumpt`, `goimports`
- [ ] `cmd/kanban/main.go` mínimo: flags `--addr`, `--db`, `--version`; `GET /healthz` → `200 ok`; imprime `LISTENING <addr>` no stdout; shutdown gracioso com `signal.NotifyContext`
- [ ] Teste unitário de `/healthz` com `httptest`
- **Asserts:** `golangci-lint run ./...` limpo; `golangci-lint fmt --diff` vazio; `go test ./...` verde; `go build` gera binário < 20 MB.

### E4. Toolchain frontend

- [ ] `package.json` com dependências fixadas (sem `^`): `@tailwindcss/cli`, `tailwindcss`, `@playwright/test`, `electron`, `electron-builder`, `typescript`
- [ ] `bun install` e `bun.lock` versionado
- [ ] `web/styles/input.css` com `@import "tailwindcss";` + `@source "../../internal/web/templates";` + tokens de tema (`@theme`)
- [ ] `scripts/vendor-htmx.ts` copia `node_modules/htmx.org/dist/htmx.min.js` (2.0.11) para `internal/web/static/`
- [ ] Scripts `dev`, `build`, `lint`, `test` no `package.json`
- **Asserts:** `bun run build` gera `app.css` minificado contendo só classes usadas; `bin/kanban` serve `/static/htmx.min.js` com `200`.

### E5. Layout base embutido

- [ ] `internal/web/templates/layout.html` com `<script src="/static/htmx.min.js">`, CSS, `hx-boost` desativado
- [ ] `GET /` renderiza layout com placeholder "Nenhum projeto ainda"
- [ ] `go:embed` de `templates/` e `static/`; templates parseados uma vez no boot
- [ ] Teste de integração: `GET /` → `200`, `Content-Type: text/html; charset=utf-8`, contém `<title>`
- **Asserts:** binário roda fora da pasta do projeto e ainda serve CSS/JS (prova do embed).

### E6. Esqueleto de testes (todas as camadas)

- [ ] Unit/integração: `go test -race ./...`; helper `testutil.NewDB(t)` com SQLite em `t.TempDir()`
- [x] Smoke: `scripts/smoke.ts` builda, sobe `bin/kanban --addr 127.0.0.1:0`, lê porta do stdout, checa `/healthz` e `/`, mata processo, exit code ≠ 0 em falha
- [ ] E2E web: `playwright.config.ts` com `webServer` subindo o binário com DB temporário; 1 teste "dashboard carrega"
- [ ] QA: `e2e/agent-browser/smoke.md` com roteiro reproduzível para `agent-browser` (abrir, snapshot, screenshot)
- **Asserts:** `bun run test`, `bun run smoke`, `bun run e2e` verdes em máquina limpa (após `bun install` + `bunx playwright install chromium`).

### E7. Spike Electron (risco alto → cedo)

- [ ] `desktop/main.ts`: resolve caminho do binário (dev: `bin/`; empacotado: `process.resourcesPath`), spawn com `--addr 127.0.0.1:0 --db <app.getPath('userData')>/kanban.db`, espera `LISTENING`, abre `BrowserWindow`
- [ ] `contextIsolation: true`, `nodeIntegration: false`, `sandbox: true`; navegação externa bloqueada (`will-navigate`, `setWindowOpenHandler`)
- [ ] Sidecar encerrado em `before-quit`; se o sidecar morrer, janela mostra erro e app sai
- [ ] `electron-builder.yml`: `extraResources` com binário Go; alvo Windows `nsis` + `portable`
- [ ] E2E desktop: Playwright `_electron.launch` abre app e vê o dashboard
- **Asserts:** `bun run dist` gera instalador; app instalado abre, cria `kanban.db` em `userData`; nenhum processo `kanban.exe` órfão após fechar (checar no Gerenciador de Tarefas / `tasklist`).

### E8. Release local `0.0.1`

- [x] `scripts/release.ts <versão>`: valida árvore limpa e semver; atualiza `VERSION`; move `[Unreleased]` → `[x.y.z] - AAAA-MM-DD` no CHANGELOG; roda lint+test+smoke; builda Go para `windows/amd64`, `linux/amd64`, `darwin/arm64` (`CGO_ENABLED=0`) + instalador Electron Windows; gera `SHA256SUMS`; commit `chore(release): vX.Y.Z`; tag anotada `vX.Y.Z`; `git push --follow-tags`; `gh release create vX.Y.Z --notes-file <seção extraída> <artefatos>`
- [x] Sufixo `-rc.N`, `-beta.N`, `-alpha.N` → `gh release create --prerelease`
- [x] `--dry-run` que faz tudo menos push/publicação
- [ ] Release `v0.0.1` publicada
- **Asserts:** página da release mostra notas iguais à seção do CHANGELOG; artefatos baixáveis; `bin/kanban --version` imprime `0.0.1`; `.github/workflows/` não existe.

### Checkpoint ESTRUTURA

- [ ] Todos os comandos da seção 6 existem e funcionam (exceto features)
- [ ] `v0.0.1` publicada no GitHub com binários e instalador
- [ ] Revisão humana antes de seguir para ALPHA

---

## FASE 2 — ALPHA (`0.1.0` … `0.4.x`)

Objetivo: kanban usável no dia a dia. Fatias verticais (schema + repositório + handler + template + testes) por funcionalidade. Release `-alpha.N` ao fim de cada grupo.

### A1. Banco e migrações

- [ ] `store.Open(path)`: aplica PRAGMAs (`foreign_keys`, `journal_mode=WAL`, `busy_timeout`); `SetMaxOpenConns(1)` para escrita segura
- [ ] Runner de migrações: lê `migrations/*.sql` embutido, ordena, aplica cada um em transação, registra em `schema_migrations`; idempotente
- [ ] `0001_init.sql` com tabelas da seção 5 + índices `(board_id, position)`, `(column_id, position)`
- **Asserts:** teste aplica migrações 2× sem erro; teste confirma FK em cascata (apagar board apaga colunas e cards); `PRAGMA foreign_keys` retorna 1.

### A2. Projetos (boards) — criar e listar

- [ ] Repositório `Boards.Create/List/Get` com testes de integração
- [ ] Criar board cria as 5 colunas padrão na mesma transação
- [ ] Sidebar lista boards; form `hx-post="/boards"` insere item na sidebar e abre o board (`HX-Push-Url`)
- [ ] Validação: nome obrigatório, 1–80 chars; erro renderizado inline com `422`
- **Asserts:** E2E "criar projeto → aparece na sidebar → quadro com 5 colunas"; teste de handler cobre `201`/`422`; XSS: nome `<script>` aparece escapado.

### A3. Dashboard como landing

- [ ] `GET /` abre último board visitado (cookie `last_board`) ou o primeiro; sem boards → estado vazio com CTA de criar
- [ ] `GET /boards/{id}` página completa; com `HX-Request` retorna só o quadro
- [ ] `404` amigável para board inexistente
- **Asserts:** navegação direta por URL e via HTMX produzem o mesmo DOM do quadro (teste de integração compara); voltar/avançar do navegador funciona (E2E).

### A4. Cards — criar, editar, excluir

- [ ] Form inline no rodapé da coluna (`hx-post="/columns/{id}/cards"`, `hx-swap="beforeend"`)
- [ ] Editar título inline (`hx-get` → form, `hx-put` → card renderizado)
- [ ] Modal/painel de detalhes: descrição (texto puro, quebras preservadas), prioridade 0–4, data de entrega
- [ ] Excluir com `hx-confirm`
- **Asserts:** E2E cobre criar/editar/excluir; repositório testado; foco do teclado volta ao card após editar.

### A5. Mover cards (drag-and-drop)

- [ ] `internal/web/static/app.js`: HTML5 DnD entre e dentro de colunas, indicador visual de destino
- [ ] `POST /cards/{id}/move` com `column_id` e `position`; reindexação em transação
- [ ] Alternativa por teclado/menu "Mover para…" (acessibilidade e fallback de teste)
- **Asserts:** teste de repositório: mover para cima/baixo/outra coluna mantém posições contíguas `0..n-1` sem duplicatas; E2E com `dragTo` persiste após reload.

### A6. Colunas — gerenciar

- [ ] Renomear, adicionar, excluir (bloquear exclusão com cards ou mover cards para outra coluna) e reordenar colunas
- [ ] Limite WIP opcional: contador `n/limite` fica vermelho quando excedido
- **Asserts:** testes de integração cobrem cada operação; E2E "renomear coluna persiste".

### A7. Projetos — editar, arquivar, excluir

- [ ] Editar nome/descrição/cor
- [ ] Arquivar (some da sidebar, recuperável em seção "Arquivados" colapsável dentro da própria sidebar — sem página extra)
- [ ] Excluir definitivo com confirmação digitando o nome
- **Asserts:** E2E arquivar → restaurar; exclusão remove dados (verificado em teste de integração).

### Checkpoint ALPHA

- [ ] Fluxo completo: criar projeto → criar cards → mover entre colunas → editar → arquivar — via E2E web **e** desktop
- [ ] Release `v0.4.0-alpha.1` (prerelease) com builds
- [ ] Uso real por 1 semana gerenciando os próprios projetos; bugs viram issues
- [ ] Revisão humana antes de seguir para BETA

---

## FASE 3 — BETA (`0.5.0` … `0.9.x`)

Objetivo: produtividade estilo Linear, robustez de dados e app desktop polido.

### B1. Labels

- [ ] CRUD de labels por board (nome + cor de paleta fixa)
- [ ] Atribuir/remover labels no card; chips no card
- **Asserts:** testes de integração da relação N:N; E2E atribuir label.

### B2. Busca e filtros

- [ ] Busca por título/descrição no board atual (`hx-trigger="input changed delay:250ms"`)
- [ ] Filtros por label, prioridade e vencidos; estado refletido na URL (query string)
- **Asserts:** teste de repositório com dataset fixo; URL com filtros reabre filtrada (E2E).

### B3. Atalhos de teclado (estilo Linear)

- [ ] `c` novo card, `/` busca, `Esc` fecha painel, `?` mostra lista de atalhos (overlay, não página)
- [ ] Atalhos ignorados quando foco está em input/textarea
- **Asserts:** E2E por atalho; navegação 100% por teclado no fluxo principal.

### B4. Tema e UX

- [ ] Modo claro/escuro (segue sistema + toggle persistido em `localStorage`)
- [ ] Estados de carregamento HTMX (`htmx-request` + indicador), toasts de erro em respostas `4xx/5xx`
- [ ] Layout responsivo mínimo (janela estreita do desktop)
- **Asserts:** contraste AA verificado (Lighthouse a11y ≥ 95); screenshot `agent-browser` nos dois temas sem regressão visual grosseira.

### B5. Dados: backup e portabilidade

- [ ] Export de todos os dados em JSON (`GET /export`, download)
- [ ] Import JSON (substituir ou mesclar) com validação e transação única
- [ ] Desktop: menu "Abrir pasta de dados" e "Fazer backup" (`VACUUM INTO`)
- **Asserts:** teste round-trip export → DB vazio → import → export idêntico.

### B6. Desktop polido

- [ ] Ícone do app, nome, `productName`, single-instance lock
- [ ] Menu nativo mínimo (Arquivo, Editar, Exibir, Ajuda → versão)
- [ ] Persistir tamanho/posição da janela
- [ ] Alvos extras: Linux `AppImage` e macOS `dmg` se build local for viável (ver Questões em aberto)
- **Asserts:** E2E desktop: segunda instância foca a primeira; versão no menu = `VERSION`.

### B7. Observabilidade local

- [ ] Logs `slog` JSON com request id, método, rota, status e duração; `--log-level`
- [ ] Desktop grava logs do sidecar em `userData/logs/` com rotação simples
- **Asserts:** teste de middleware verifica campos do log; erro `500` sempre logado com stack resumida.

### Checkpoint BETA

- [ ] Cobertura Go ≥ 80% em `internal/store` e ≥ 70% em `internal/web` (`go test -cover`)
- [ ] Suíte E2E web + desktop verde 3 execuções seguidas (sem flakiness)
- [ ] Release `v0.9.0-beta.1` (prerelease)
- [ ] Revisão humana antes do RC

---

## FASE 4 — 1.0.0 RC (`1.0.0-rc.N` → `1.0.0`)

Objetivo: congelar features, endurecer, documentar e lançar.

### R1. Feature freeze e hardening

- [ ] Nenhuma feature nova; só `fix:`, `perf:`, `docs:`, `test:`
- [ ] `/security-review` + skill `security-and-hardening`: validação de todas as entradas, limites de tamanho de body (`http.MaxBytesReader`), timeouts do `http.Server`, headers (`Content-Security-Policy` sem inline script, `X-Content-Type-Options`)
- [ ] `govulncheck ./...` limpo
- **Asserts:** CSP ativa e app funcional (nenhum erro de CSP no console via Playwright); `govulncheck` sem achados.

### R2. Performance

- [ ] Board com 500 cards renderiza em < 200 ms no servidor (benchmark `testing.B`)
- [ ] Mover card < 50 ms p95 (benchmark)
- [ ] Binário Go < 25 MB; instalador desktop < 120 MB
- **Asserts:** benchmarks registrados no CHANGELOG da versão; nenhuma query N+1 (contagem de queries por request em teste).

### R3. Acessibilidade

- [ ] Skill `frontend-ui-engineering` + checklist `.claude/references/accessibility-checklist.md`
- [ ] Landmarks, labels, foco visível, `aria-live` para toasts, DnD com alternativa por teclado
- **Asserts:** Lighthouse a11y ≥ 95; `@axe-core/playwright` sem violações sérias (dependência nova → pedir aprovação antes).

### R4. Migração e compatibilidade de dados

- [ ] Teste: DB criado pela `v0.4.0-alpha.1` abre e migra na `1.0.0` sem perda
- [ ] Backup automático do DB antes de aplicar migração nova
- **Asserts:** fixture de DB antigo versionada em `internal/store/testdata/`; teste de upgrade verde.

### R5. Documentação

- [ ] `README.md` completo: screenshots, instalação, comandos, arquitetura
- [ ] `docs/adr/` com D1–D9 como ADRs
- [ ] `CLAUDE.md`/`AGENTS.md` atualizados; `graphify update .` rodado
- **Asserts:** pessoa nova clona, roda `bun install && bun run dev` e usa o app seguindo só o README.

### R6. Release candidate e final

- [ ] `v1.0.0-rc.1` (prerelease) → uso real ≥ 1 semana → `rc.N` se houver correções
- [ ] Zero bugs abertos com label `bug` e severidade alta
- [ ] `v1.0.0` publicada com notas completas e todos os artefatos
- **Asserts:** artefatos da release instalam e abrem em Windows limpo (VM/Sandbox do Windows); `SHA256SUMS` confere.

### Checkpoint 1.0.0

- [ ] Todos os checkpoints anteriores marcados
- [ ] Definition of Done (seção 7) cumprida em todas as tarefas

---

## 8. Riscos e mitigações

| Risco | Impacto | Mitigação |
| --- | --- | --- |
| Sidecar Go órfão ou porta ocupada no Electron | Alto | Porta `0` (SO escolhe), handshake `LISTENING`, kill em `before-quit`, teste E2E desktop (E7) |
| Antivírus/SmartScreen bloqueia instalador sem assinatura | Médio | Documentar no README; avaliar assinatura de código depois da 1.0 |
| `go:embed` exige `app.css` existente antes de `go build`/`go test` | Médio | CSS gerado em `static/` antes do build; templates não dependem do conteúdo do CSS nos testes; `.gitkeep` |
| Flakiness em E2E de drag-and-drop | Médio | Alternativa "Mover para…" testável; `dragTo` + asserts por estado persistido, não por animação |
| SQLite com escrita concorrente (`database is locked`) | Baixo | WAL + `busy_timeout` + `SetMaxOpenConns(1)` |
| Build Electron para macOS/Linux a partir do Windows | Médio | Binários Go cross-compilam sempre; instaladores Electron não-Windows via WSL ou ficam para depois |

## 9. Questões em aberto

- Instaladores Electron de Linux/macOS são necessários já na BETA ou só binários Go web bastam?
- Repositório GitHub público ou privado?
- Precisa de comentários/checklists dentro do card (estilo Trello) ou descrição simples basta para a 1.0?
- `@axe-core/playwright` (R3) aprovado como dependência de dev?
