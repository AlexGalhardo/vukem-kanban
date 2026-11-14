# PLAN.md — Kanban de Projetos (Go + HTMX + SQLite + Electron)

> Documento principal de planejamento. Toda implementação parte daqui.
> Marque `[x]` somente quando o item e seus **asserts de qualidade** estiverem verificados (comando rodado, resultado conferido).

## 1. Visão geral

Aplicação web/desktop simplificada no estilo Trello/Linear para gerenciar o desenvolvimento dos meus projetos de software.
A raiz (`/`) abre direto no dashboard: barra lateral com os projetos (boards) e o quadro kanban do projeto selecionado.
Sem autenticação, sem páginas extras. Uso single-user, local.

**Fora de escopo (não implementar):** autenticação/usuários, multi-tenant, páginas de marketing/landing, sincronização em nuvem, publicação por GitHub Actions (o CI de validação existe desde a migração para o vukem).

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
| Frontend | htmx 4 (arquivo vendorizado, sem CDN — app precisa funcionar offline) | 4.0.0 |
| CSS | Tailwind CSS v4 via `@tailwindcss/cli` | 4.3.3 |
| Desktop | Electron + electron-builder | 44.5.1 / 26.15.3 |
| E2E | `@playwright/test` (web e Electron via `_electron`) | 1.63.0 |
| QA exploratório | `agent-browser` | 0.38.2 |
| Runner JS/tarefas | Bun | 1.4.2 |

## 3. Decisões de arquitetura

- **D1 — Stdlib primeiro (didático), substituída.** Até a v0.9.0-beta.1 a única dependência Go era o driver SQLite. A partir da migração para o vukem (`docs/audit-2026-11-14.md`), roteamento, middleware, config, migrações e testes de app vêm do framework.
- **D2 — `modernc.org/sqlite` em vez de `mattn/go-sqlite3`.** Sem CGO: cross-compile trivial para Windows/Linux/macOS e build do Electron sem toolchain C.
- **D3 — Binário único.** Templates, CSS gerado e `htmx.min.js` embutidos via `go:embed`. O binário roda sozinho como app web.
- **D4 — Electron como casca fina.** O processo main do Electron sobe o binário Go como sidecar (`--addr 127.0.0.1:0 --db <userData>/kanban.db`), lê a porta impressa no stdout (`LISTENING <addr>`) e abre `BrowserWindow` nessa URL. Ao fechar a janela, encerra o sidecar.
- **D5 — htmx 4 server-driven.** Handlers retornam fragmentos HTML. Requisição com `HX-Request` e `HX-Request-Type: partial` recebe parcial; sem eles (ou `full`, usado por `hx-select` e restauração de histórico), página completa. Erros de htmx voltam com `HX-Reswap: none` + toast via `HX-Trigger` (JSON escapado em ASCII).
- **D6 — Drag-and-drop nativo.** HTML5 DnD + ~50 linhas de JS que chamam `htmx.ajax('POST', '/cards/{id}/move', ...)`. Sem SortableJS. Reordenação por `position INTEGER` reindexada dentro de transação.
- **D7 — Bun como task runner.** `package.json` concentra scripts (`dev`, `build`, `test`, `lint`, `e2e`, `release`). Scripts complexos em `scripts/*.ts` rodados com `bun`. Evita `make` (ausente no Windows).
- **D8 — Release local.** `scripts/release.ts` gera builds, extrai a seção da versão do `CHANGELOG.md` e publica com `gh release create`. O único workflow (`ci.yml`) valida, não publica.
- **D9 — Sem auth; CSRF via stdlib.** App local single-user; servidor escuta só em `127.0.0.1`. `http.CrossOriginProtection` (Go 1.25+) barra POST/PUT/DELETE cross-site de páginas maliciosas abertas no navegador. Validação de entrada e escaping (`html/template`) continuam obrigatórios.

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

- [x] `git init`, branch `main`, remote `origin` = `https://github.com/AlexGalhardo/vukem-kanban.git`
- [x] `.editorconfig`: `indent_style = tab`, `indent_size = 4`, `tab_width = 4`, `end_of_line = lf`, `charset = utf-8`, `insert_final_newline = true`; exceção `*.{yml,yaml,md}` com espaços (YAML proíbe tab)
- [x] `.gitattributes` com `* text=auto eol=lf`
- [x] `.gitignore` (bin/, dist/, node_modules/, *.db, `internal/web/static/app.css`, graphify-out/, test-results/, playwright-report/)
- [x] `CHANGELOG.md` no formato Keep a Changelog 1.1.0 com `[Unreleased]`
- [x] `VERSION` contendo `0.0.1` (fonte única da versão; injetada via `-ldflags "-X main.version=..."` e lida pelo `package.json` do desktop)
- [x] `README.md` curto: o que é, como rodar, comandos
- [x] Commit inicial `chore: bootstrap repository`
- [x] Paralelização com git worktrees documentada e seguida (`docs/workflow.md`)
- **Asserts:** `git log --oneline` mostra commit convencional; editor abre `.go` com tab de largura 4.

### E3. Toolchain Go

- [x] `go mod init github.com/AlexGalhardo/vukem-kanban` com `go 1.27.0`
- [x] `.golangci.yml` (v2): linters `errcheck`, `govet`, `staticcheck`, `unused`, `ineffassign`, `gosec`, `revive`, `bodyclose`, `sqlclosecheck`, `rowserrcheck`, `errorlint`, `misspell`; formatters `gofumpt`, `goimports`
- [x] `cmd/kanban/main.go` mínimo: flags `--addr`, `--db`, `--version`; `GET /healthz` → `200 ok`; imprime `LISTENING <addr>` no stdout; shutdown gracioso com `signal.NotifyContext`
- [x] Teste unitário de `/healthz` com `httptest`
- **Asserts:** `golangci-lint run ./...` limpo; `golangci-lint fmt --diff` vazio; `go test ./...` verde; `go build` gera binário < 20 MB.

### E4. Toolchain frontend

- [x] `package.json` com dependências fixadas (sem `^`): `@tailwindcss/cli`, `tailwindcss`, `@playwright/test`, `electron`, `electron-builder`, `typescript`
- [x] `bun install` e `bun.lock` versionado
- [x] `web/styles/input.css` com `@import "tailwindcss";` + `@source "../../internal/web/templates";` + tokens de tema (`@theme`)
- [x] `scripts/vendor-htmx.ts` copia `node_modules/htmx.org/dist/htmx.min.js` (4.0.0) para `internal/web/static/`
- [x] Scripts `dev`, `build`, `lint`, `test` no `package.json`
- **Asserts:** `bun run build` gera `app.css` minificado contendo só classes usadas; `bin/kanban` serve `/static/htmx.min.js` com `200`.

### E5. Layout base embutido

- [x] `internal/web/templates/layout.html` com `<script src="/static/htmx.min.js">`, CSS, `hx-boost` desativado
- [x] `GET /` renderiza layout com placeholder "Nenhum projeto ainda"
- [x] `go:embed` de `templates/` e `static/`; templates parseados uma vez no boot
- [x] Teste de integração: `GET /` → `200`, `Content-Type: text/html; charset=utf-8`, contém `<title>`
- **Asserts:** binário roda fora da pasta do projeto e ainda serve CSS/JS (prova do embed).

### E6. Esqueleto de testes (todas as camadas)

- [x] Unit/integração: `go test -race ./...`; helpers `newStore(t)`/`newApp(t)` com SQLite em `t.TempDir()`
- [x] Smoke: `scripts/smoke.ts` builda, sobe `bin/kanban --addr 127.0.0.1:0`, lê porta do stdout, checa `/healthz` e `/`, mata processo, exit code ≠ 0 em falha
- [x] E2E web: `playwright.config.ts` com `webServer` subindo o binário com DB temporário; 1 teste "dashboard carrega"
- [x] QA: `e2e/agent-browser/smoke.md` com roteiro reproduzível para `agent-browser` (abrir, snapshot, screenshot)
- **Asserts:** `bun run test`, `bun run smoke`, `bun run e2e` verdes em máquina limpa (após `bun install` + `bunx playwright install chromium`).

### E7. Spike Electron (risco alto → cedo)

- [x] `desktop/main.ts`: resolve caminho do binário (dev: `bin/`; empacotado: `process.resourcesPath`), spawn com `--addr 127.0.0.1:0 --db <app.getPath('userData')>/kanban.db`, espera `LISTENING`, abre `BrowserWindow`
- [x] `contextIsolation: true`, `nodeIntegration: false`, `sandbox: true`; navegação externa bloqueada (`will-navigate`, `setWindowOpenHandler`)
- [x] Sidecar encerrado em `before-quit`; se o sidecar morrer, janela mostra erro e app sai
- [x] `electron-builder.yml`: `extraResources` com binário Go; alvo Windows `nsis` + `portable`
- [x] E2E desktop: Playwright `_electron.launch` abre app e vê o dashboard
- **Asserts:** `bun run dist` gera instalador; app instalado abre, cria `kanban.db` em `userData`; nenhum processo `kanban.exe` órfão após fechar (checar no Gerenciador de Tarefas / `tasklist`).

### E8. Release local `0.0.1`

- [x] `scripts/release.ts <versão>`: valida árvore limpa e semver; atualiza `VERSION`; move `[Unreleased]` → `[x.y.z] - AAAA-MM-DD` no CHANGELOG; roda lint+test+smoke; builda Go para `windows/amd64`, `linux/amd64`, `darwin/arm64` (`CGO_ENABLED=0`) + instalador Electron Windows; gera `SHA256SUMS`; commit `chore(release): vX.Y.Z`; tag anotada `vX.Y.Z`; `git push --follow-tags`; `gh release create vX.Y.Z --notes-file <seção extraída> <artefatos>`
- [x] Sufixo `-rc.N`, `-beta.N`, `-alpha.N` → `gh release create --prerelease`
- [x] `--dry-run` que faz tudo menos push/publicação
- [ ] Release `v0.0.1` publicada
- **Asserts:** página da release mostra notas iguais à seção do CHANGELOG; artefatos baixáveis; `bin/kanban --version` imprime `0.0.1`; a release não é publicada por workflow.

### Checkpoint ESTRUTURA

- [x] Todos os comandos da seção 6 existem e funcionam (exceto features)
- [ ] `v0.0.1` publicada no GitHub com binários e instalador
- [x] Revisão humana antes de seguir para ALPHA — dispensada: o usuário pediu execução contínua até o fim do plano

---

## FASE 2 — ALPHA (`0.1.0` … `0.4.x`)

Objetivo: kanban usável no dia a dia. Fatias verticais (schema + repositório + handler + template + testes) por funcionalidade. Release `-alpha.N` ao fim de cada grupo.

### A1. Banco e migrações

- [x] `store.Open(path)`: aplica PRAGMAs (`foreign_keys`, `journal_mode=WAL`, `busy_timeout`); `SetMaxOpenConns(1)` para escrita segura
- [x] Runner de migrações: lê `migrations/*.sql` embutido, ordena, aplica cada um em transação, registra em `schema_migrations`; idempotente
- [x] `0001_init.sql` com tabelas da seção 5 + índices `(board_id, position)`, `(column_id, position)`
- **Asserts:** teste aplica migrações 2× sem erro; teste confirma FK em cascata (apagar board apaga colunas e cards); `PRAGMA foreign_keys` retorna 1.

### A2. Projetos (boards) — criar e listar

- [x] Repositório `Boards.Create/List/Get` com testes de integração
- [x] Criar board cria as 5 colunas padrão na mesma transação
- [x] Sidebar lista boards; form `hx-post="/boards"` insere item na sidebar e abre o board (`HX-Push-Url`)
- [x] Validação: nome obrigatório, 1–80 chars; erro renderizado inline com `422`
- **Asserts:** E2E "criar projeto → aparece na sidebar → quadro com 5 colunas"; teste de handler cobre `201`/`422`; XSS: nome `<script>` aparece escapado.

### A3. Dashboard como landing

- [x] `GET /` abre último board visitado (cookie `last_board`) ou o primeiro; sem boards → estado vazio com CTA de criar
- [x] `GET /boards/{id}` página completa; com `HX-Request` retorna só o quadro
- [x] `404` amigável para board inexistente
- **Asserts:** navegação direta por URL e via HTMX produzem o mesmo DOM do quadro (teste de integração compara); voltar/avançar do navegador funciona (E2E).

### A4. Cards — criar, editar, excluir

- [x] Form inline no rodapé da coluna (`hx-post="/columns/{id}/cards"`, `hx-swap="beforeend"`)
- [x] Editar título inline (`hx-get` → form, `hx-put` → card renderizado)
- [x] Modal/painel de detalhes: descrição (texto puro, quebras preservadas), prioridade 0–4, data de entrega
- [x] Excluir com `hx-confirm`
- **Asserts:** E2E cobre criar/editar/excluir; repositório testado; foco do teclado volta ao card após editar.

### A5. Mover cards (drag-and-drop)

- [x] `internal/web/static/app.js`: HTML5 DnD entre e dentro de colunas, indicador visual de destino
- [x] `POST /cards/{id}/move` com `column_id` e `before_id` (âncora no card visível, correto com filtro ativo); reindexação em transação; resposta só com as colunas afetadas (OOB)
- [x] Alternativa por teclado/menu "Mover para…" (acessibilidade e fallback de teste)
- **Asserts:** teste de repositório: mover para cima/baixo/outra coluna mantém posições contíguas `0..n-1` sem duplicatas; E2E com `dragTo` persiste após reload.

### A6. Colunas — gerenciar

- [x] Renomear, adicionar, excluir (bloquear exclusão com cards ou mover cards para outra coluna) e reordenar colunas
- [x] Limite WIP opcional: contador `n/limite` fica vermelho quando excedido
- **Asserts:** testes de integração cobrem cada operação; E2E "renomear coluna persiste".

### A7. Projetos — editar, arquivar, excluir

- [x] Editar nome/descrição/cor
- [x] Arquivar (some da sidebar, recuperável em seção "Arquivados" colapsável dentro da própria sidebar — sem página extra)
- [x] Excluir definitivo com confirmação digitando o nome
- **Asserts:** E2E arquivar → restaurar; exclusão remove dados (verificado em teste de integração).

### Checkpoint ALPHA

- [x] Fluxo completo: criar projeto → criar cards → mover entre colunas → editar → arquivar — via E2E web **e** desktop
- [x] Release `v0.4.0-alpha.1` (prerelease) com builds
- [ ] Uso real por 1 semana gerenciando os próprios projetos; bugs viram issues — pendente do usuário; substituído nesta fase por uma rodada de QA adversarial em worktree (26 achados, corrigidos com testes de regressão)
- [x] Revisão humana antes de seguir para BETA — dispensada: o usuário pediu execução contínua até o fim do plano

---

## FASE 3 — BETA (`0.5.0` … `0.9.x`)

Objetivo: produtividade estilo Linear, robustez de dados e app desktop polido.

### B1. Labels

- [x] CRUD de labels por board (nome + cor de paleta fixa)
- [x] Atribuir/remover labels no card; chips no card
- **Asserts:** testes de integração da relação N:N; E2E atribuir label.

### B2. Busca e filtros

- [x] Busca por título/descrição no board atual (`hx-trigger="input changed delay:250ms"`)
- [x] Filtros por label, prioridade e vencidos; estado refletido na URL (query string)
- **Asserts:** teste de repositório com dataset fixo; URL com filtros reabre filtrada (E2E).

### B3. Atalhos de teclado (estilo Linear)

- [x] `c` novo card, `/` busca, `Esc` fecha painel, `?` mostra lista de atalhos (overlay, não página)
- [x] Atalhos ignorados quando foco está em input/textarea
- **Asserts:** E2E por atalho; navegação 100% por teclado no fluxo principal.

### B4. Tema e UX

- [x] Modo claro/escuro (segue sistema + toggle persistido em `localStorage`)
- [x] Estados de carregamento HTMX (`htmx-request` + indicador), toasts de erro em respostas `4xx/5xx`
- [x] Layout responsivo mínimo (janela estreita do desktop)
- **Asserts:** contraste AA verificado (Lighthouse a11y ≥ 95); screenshot `agent-browser` nos dois temas sem regressão visual grosseira.

### B5. Dados: backup e portabilidade

- [x] Export de todos os dados em JSON (`GET /export`, download)
- [x] Import JSON (substituir ou mesclar) com validação e transação única
- [x] Desktop: menu "Abrir pasta de dados" e "Fazer backup" (`VACUUM INTO`) — itens de menu verificados no E2E desktop; o endpoint `POST /backup` é coberto por testes (o clique no menu abre um diálogo nativo bloqueante, por isso não é automatizado)
- **Asserts:** teste round-trip export → DB vazio → import → export idêntico.

### B6. Desktop polido

- [x] Ícone do app, nome, `productName`, single-instance lock
- [x] Menu nativo mínimo (Arquivo, Editar, Exibir, Ajuda → versão)
- [x] Persistir tamanho/posição da janela
- [ ] Alvos extras: Linux `AppImage` e macOS `dmg` se build local for viável (ver Questões em aberto) — adiado: seções existem no `electron-builder.yml`, mas build local exige Linux/macOS; releases já trazem binários Go dessas plataformas
- **Asserts:** E2E desktop: segunda instância foca a primeira; versão no menu = `VERSION`.

### B7. Observabilidade local

- [x] Logs `slog` JSON com request id, método, rota, status e duração; `--log-level`
- [x] Desktop grava logs do sidecar em `userData/logs/` com rotação simples
- **Asserts:** teste de middleware verifica campos do log; erro `500` sempre logado com stack resumida.

### B8. Markdown nos cards (pedido durante a BETA)

- [x] `internal/markdown`: renderer próprio (stdlib) do subconjunto GFM — títulos, ênfase, riscado, código, blocos de código, citações, listas aninhadas, checklists, links `http/https/mailto` e autolink; todo texto escapado antes de virar HTML
- [x] Diálogo do card com abas "Escrever/Visualizar" (prévia via `POST /markdown/preview`), duplo clique na prévia volta a editar
- [x] Edição assistida: Enter continua listas/checklists, Enter em item vazio encerra a lista, Tab/Shift+Tab indentam, Ctrl+B/Ctrl+I envolvem a seleção
- [x] Progresso do checklist no card (`1/3`, verde quando completo)
- **Asserts:** `go test ./internal/markdown` (91% de cobertura, entradas hostis sem HTML ativo); E2E "markdown description" verde.

### B9. QA round 2 (correções)

- [x] Perda de texto ao digitar logo após Enter (campos de criação rápida)
- [x] Edições com filtro ativo, filtro de label excluída, conflito entre abas (`revision` + 409)
- [x] Foco após ações de teclado, confirmação de alterações não salvas, revert local do arrasto, Alt+setas para mover cards
- [x] Título da página, projeto excluído em outra aba, contraste AA, `inert` na sidebar mobile, Esc no menu da coluna
- [x] Texto invisível/UTF-8 inválido, chave do projeto por runa, hash dos assets no cache
- **Asserts:** 20 specs `e2e/web/qa2-*.spec.ts` verdes; testes Go novos (revisão, sanitização, filtros); suíte E2E 54/54.

### Checkpoint BETA

- [x] Cobertura Go ≥ 80% em `internal/store` e ≥ 70% em `internal/web` (`go test -cover`) — store ~84%, web ~82%, markdown ~92%
- [x] Suíte E2E web + desktop verde 3 execuções seguidas (sem flakiness) — 34/34 ×3 antes do QA 2; 54/54 após as correções do QA 2
- [x] Release `v0.9.0-beta.1` (prerelease)
- [x] Revisão humana antes do RC — dispensada: o usuário pediu execução contínua; QA adversarial (2 rodadas) e revisão de segurança independente no lugar

---

## FASE 4 — 1.0.0 RC (`1.0.0-rc.N` → `1.0.0`)

Objetivo: congelar features, endurecer, documentar e lançar.

### R1. Feature freeze e hardening

- [ ] Nenhuma feature nova; só `fix:`, `perf:`, `docs:`, `test:`
- [x] `/security-review` + skill `security-and-hardening`: validação de todas as entradas, limites de tamanho de body (`http.MaxBytesReader`), timeouts do `http.Server`, headers (`Content-Security-Policy` sem inline script, `X-Content-Type-Options`) — revisão independente sem achados relevantes; o único (corpo *chunked* acima do limite) foi corrigido com teste
- [x] `govulncheck ./...` limpo
- **Asserts:** CSP ativa e app funcional (nenhum erro de CSP no console via Playwright); `govulncheck` sem achados.

### R2. Performance

- [x] Board com 500 cards renderiza em < 200 ms no servidor (benchmark `testing.B`)
- [x] Mover card < 50 ms p95 (benchmark) — medido: render de 500 cards ~14 ms, mover ~6 ms (`TestPerformanceBudgets`, `BenchmarkRenderBoard500`, `BenchmarkMoveCard500`)
- [x] Binário Go < 25 MB; instalador desktop < 120 MB — medido: 14 MB e 115 MB
- **Asserts:** benchmarks registrados no CHANGELOG da versão; nenhuma query N+1 (`LoadBoard` usa um número fixo de queries; o orçamento de tempo em teste pegaria um N+1).

### R3. Acessibilidade

- [x] Skill `frontend-ui-engineering` + checklist `.claude/references/accessibility-checklist.md`
- [x] Landmarks, labels, foco visível, `aria-live` para toasts, DnD com alternativa por teclado — Lighthouse acessibilidade 97 (alvos de toque da sidebar ajustados para 28 px)
- **Asserts:** Lighthouse a11y ≥ 95 (medido: 97); `@axe-core/playwright` não adotado — exigiria dependência nova; Lighthouse via Chrome DevTools cobre a auditoria.

### R4. Migração e compatibilidade de dados

- [x] Teste: DB criado pela `v0.4.0-alpha.1` abre e migra na `1.0.0` sem perda
- [x] Backup automático do DB antes de aplicar migração nova
- **Asserts:** fixture de DB antigo versionada em `internal/store/testdata/`; teste de upgrade verde.

### R5. Documentação

- [x] `README.md` completo: screenshots, instalação, comandos, arquitetura
- [x] `docs/adr/` com D1–D9 como ADRs
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

## 9. Estado ao fim da sessão de 2026-10-05

- Fases ESTRUTURA, ALPHA e BETA concluídas e publicadas: `v0.0.1`, `v0.4.0-alpha.1` e `v0.9.0-beta.1`.
- Próxima sessão: fase 1.0.0 RC — R1 (freeze), R5 (`graphify update .`), R6 (`v1.0.0-rc.1` → uso real → `v1.0.0`).
- Pendências do usuário: uso real por 1 semana (ALPHA), decidir alvos Linux/macOS do Electron (B6).

## 10. Questões em aberto

- Instaladores Electron de Linux/macOS são necessários já na BETA ou só binários Go web bastam?
- Repositório GitHub público ou privado?
- Precisa de comentários/checklists dentro do card (estilo Trello) ou descrição simples basta para a 1.0?
- `@axe-core/playwright` (R3) aprovado como dependência de dev?
