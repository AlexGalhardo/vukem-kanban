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
│  vukem (router, middleware, config, migrações)      │
│     ├─ app/controllers (handlers htmx)              │
│     ├─ html/template (embed) → páginas e parciais   │
│     └─ static (embed): htmx.min.js, app.js, app.css │
│  internal/store (database/sql sobre app.DB())       │
└───────────────────────┬─────────────────────────────┘
                        ▼
                 kanban.db (SQLite, WAL)
```

Sem Electron, o mesmo binário funciona como app web: `bin/kanban --addr 127.0.0.1:8080`.

## Decisões (ADR resumido)

| ID | Decisão | Motivo |
| --- | --- | --- |
| D1 | Framework vukem (`github.com/AlexGalhardo/vukem`) sobre `net/http`; SQLite via o driver embutido do vukem (`modernc.org/sqlite`) | Roteamento, middleware, config, migrações e testes padronizados com os demais apps; antes da migração a regra era "só stdlib" |
| D2 | `modernc.org/sqlite` (pure Go) | Sem CGO: cross-compile e build Electron sem toolchain C |
| D3 | Binário único com `go:embed` (templates, CSS, JS) | Distribuição trivial; funciona offline |
| D4 | Electron como casca fina com sidecar Go | Lógica fica toda em Go; desktop só gerencia processo e janela |
| D5 | HTMX server-driven: handlers devolvem fragmentos HTML | Sem SPA, sem JSON API, sem build JS complexo |
| D6 | Drag-and-drop HTML5 nativo + JS mínimo | Evita SortableJS; `position` reindexada em transação |
| D7 | Bun como task runner (`package.json`) | `make` ausente no Windows; Bun já é necessário para Tailwind/Electron/Playwright |
| D8 | Release local via `scripts/release.ts` + `gh`; CI de validação em `.github/workflows/ci.yml` | O release continua local; o CI só valida (não publica) |
| D9 | Sem auth e sem sessões (`Stateless`); CSRF barrado pelo `http.CrossOriginProtection` do vukem; servidor só em `127.0.0.1` | App local single-user, mas páginas maliciosas no navegador poderiam postar no loopback |

ADRs completos irão para `docs/adr/` na fase 1.0.0 RC (ver `PLAN.md` R5).

## Convenções HTTP/htmx 4

- Rotas com padrões do Go 1.22+: `GET /boards/{id}`, `POST /columns/{id}/cards`, `POST /cards/{id}/move`.
- `HX-Request: true` com `HX-Request-Type: partial` → fragmento. Sem htmx, ou `HX-Request-Type: full` (requisições com `hx-select`, restauração de histórico) → página completa com layout. Mesma URL serve os dois.
- htmx 4 tem herança explícita: atributos herdados usam `:inherited` (ex.: `hx-indicator:inherited`, `hx-target:inherited` no form de filtros).
- htmx 4 troca respostas 4xx/5xx no DOM. Por isso `fail()` responde a htmx com `HX-Reswap: none` + toast no header `X-Toast`; só `422` (re-render de formulário com erros) é trocado. A config `noSwap` inclui `5xx` como rede de segurança.
- `HX-Trigger` sempre em JSON escapado para ASCII (`\uXXXX`): o navegador decodifica headers como Latin-1.
- Mutações re-renderizam com o filtro ativo lido de `HX-Current-URL`.
- Fragmentos fora do alvo usam `hx-swap-oob`; um elemento OOB nunca pode estar dentro do conteúdo principal da mesma resposta (o htmx 4 o extrai antes do swap). Flags separadas: `OOB` (sidebar/card) e `ColumnsOOB` (`#columns` + filtro de label).
- Toasts vão no header `X-Toast` (lido em `htmx:after:request`), não em `HX-Trigger`: o evento do `HX-Trigger` é disparado no elemento de origem após o swap e se perde quando o swap remove esse elemento.
- Respostas parciais de navegação incluem `<title>` (template `title-tag`) para o htmx atualizar o `document.title`.
- Edição do card usa trava otimista: o formulário envia `revision`; versão desatualizada → `409` (`store.ErrConflict`).
- Assets estáticos usam `?v=<hash do conteúdo>`; `/static/` tem `Cache-Control: immutable` (`StaticConfig.ImmutablePrefixes`).
- Limite de corpo: `BodyLimit` de 1 MiB no stack do servidor (antes do `MethodOverride`, que lê o formulário) e 20 MiB na rota `/import`; os handlers de escrita passam por `Kanban.form`, que devolve 413 para corpo grande demais.
- Todas as views são blocos de uma página-host (`resources/views/kanban.html`, `layout: none`): o `render` executa vários blocos em sequência no mesmo buffer.
- Foco: o `app.js` lembra o elemento focado antes de cada request e o restaura após o swap (inclusive quando o `htmx:after:settle` cai num nó já removido).
- Eventos JS usam os nomes do htmx 4 (`htmx:after:settle`, `htmx:after:request`, `htmx:response:error`, `htmx:error`); o detalhe vem em `event.detail.ctx`.
- Navegação que muda de board usa `HX-Push-Url`.
- Escaping sempre via `html/template`; nunca `template.HTML` com entrada do usuário.

## Layout do código

Segue o padrão de app vukem (`vukem new`).

```text
cmd/kanban/main.go           # vukem.RunMain; flags legadas (--addr, --db, --version, --log-level) viram HOST/PORT/DATABASE_URL/LOG_LEVEL; linha LISTENING
bootstrap/                   # vukem.New: views, rotas, migrações, provider de auto-migração
routes/                      # web.go (middleware + rotas), registry.go
app/controllers/             # Kanban: handlers, render (blocos de partials), toasts, rotas (routes.go)
database/migrations/         # registry.go e sql/NNNN_descricao.sql (embed); roda pelo migrator do vukem
resources/                   # embed de views/ e public/
resources/views/             # kanban.html (página-host) e partials/*.html (layout, blocos, diálogos)
resources/public/static/     # htmx.min.js (vendorizado), app.js, app.css (gerado, ignorado no git)
internal/store/              # repositórios sobre *sql.DB (store.New com app.DB(); store.Open só para testes e ferramentas)
internal/markdown/           # renderer Markdown seguro (subconjunto GFM) das descrições
tests/                       # testes HTTP do app inteiro (vukemtest) e orçamentos de performance
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
- O store usa o pool de escrita do vukem (`app.DB().SQL()`, uma conexão), o que evita `database is locked`.
- `position` sempre contígua `0..n-1` por coluna/board; reordenação em uma transação.
- Migrações só para frente, numeradas (`database/migrations/sql/NNNN_*.sql`), aplicadas pelo migrator do vukem em transação e registradas na tabela `migrations`; o provider `autoMigrate` roda na subida, com cópia `<db>.pre-<id>.bak` antes de migrar um banco que já tem dados. Bancos anteriores ao vukem (tabela `schema_migrations`) são reconhecidos: as migrações cujo efeito já existe são puladas. Nunca editar migração já lançada.

## Contrato Electron ↔ sidecar

1. Electron executa `kanban --addr 127.0.0.1:0 --db <userData>/kanban.db --log-level info` (o `main.go` traduz as flags para `HOST`, `PORT`, `DATABASE_URL` e `LOG_LEVEL`; `APP_ENV` assume `production` e `APP_KEY` uma chave aleatória por processo, pois o app não usa sessões). Binário: `bin/kanban(.exe)` no dev; `process.resourcesPath/bin/` no app empacotado (`extraResources`).
2. Go imprime exatamente `LISTENING 127.0.0.1:<porta>` no stdout quando pronto. Sem essa linha em 15 s → diálogo de erro e o app sai.
3. Electron abre a URL; `will-navigate` bloqueia origens diferentes da do servidor e `setWindowOpenHandler` nega janelas novas (links `http(s)`/`mailto` vão para o navegador via `shell.openExternal`); `contextIsolation`, `sandbox`, sem `nodeIntegration`. O preload só expõe `window.kanbanDesktop = { platform, version }`.
4. `before-quit`/`will-quit` encerram o sidecar. Sidecar morto inesperadamente → Electron mostra erro e sai.
5. stderr do sidecar (logs JSON) é anexado a `<userData>/logs/kanban.log`; se passar de 5 MB na inicialização vira `kanban.log.1` (substitui o anterior).
6. "Arquivo → Fazer backup" chama `POST /backup` com o `fetch` do Node (sem `Origin`, passa pela proteção CSRF) e mostra o caminho retornado.

### Shell desktop (`desktop/main.ts`)

- `KANBAN_DATA_DIR` sobrescreve `userData` (via `app.setPath` antes do `ready`); usado nos testes para DB isolado. O single-instance lock é por `userData`.
- Single instance: a segunda instância sai e a primeira restaura/foca a janela.
- Tamanho, posição e maximizado salvos em `<userData>/window-state.json`; restaurados só se a janela intersectar algum display atual.
- Menu pt-BR: Arquivo (Fazer backup, Abrir pasta de dados, Sair), Editar (roles), Exibir (recarregar, DevTools só fora do pacote, zoom, tela cheia), Ajuda (Sobre com `app.getVersion()` e link do GitHub).
- `bun build` gera `desktop/dist/main.js` e `preload.js` (CJS, ignorados no git). O preload é resolvido via `app.getAppPath()` porque o Bun embute `__dirname` como diretório do fonte.
- `desktop/electron-builder.yml`: Windows `nsis` + `portable` x64 em `dist/desktop/`; seções Linux `AppImage` e macOS `dmg` definidas, mas não buildadas localmente. Ícone `desktop/build/icon.png` (512×512, renderizado de `resources/public/static/favicon.svg`).
