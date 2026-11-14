# Testes

Pirâmide: muitos unitários e de integração em Go; poucos E2E cobrindo fluxos reais.

| Camada | Ferramenta | Onde | Comando |
| --- | --- | --- | --- |
| Unitário | `testing` | `*_test.go` ao lado do código | `bun run test` |
| Integração | `testing` + `httptest` + SQLite real em `t.TempDir()` | `internal/store`, `tests/` (app inteiro via `vukemtest`) | `bun run test` |
| Smoke | `scripts/smoke.ts` | binário compilado | `bun run smoke` |
| E2E web | `@playwright/test` | `e2e/web/` | `bun run e2e` |
| E2E desktop | Playwright `_electron` | `e2e/desktop/` | `bun run e2e:desktop` |
| QA exploratório | `agent-browser` | `e2e/agent-browser/*.md` (roteiros) | manual/agente |
| Benchmark | `testing.B` | `*_test.go` | `go test -bench . ./internal/...` |

## Regras

- Go: table-driven tests, `t.Parallel()` quando não houver estado compartilhado, `t.Helper()` em helpers.
- Nada de mock de banco: integração usa SQLite real via `testutil.NewDB(t)` (arquivo temporário, migrações aplicadas).
- Handlers testados com `httptest.NewRecorder`; checar status, `Content-Type` e fragmentos HTML relevantes. Testar com e sem header `HX-Request`.
- Segurança mínima por handler que renderiza entrada do usuário: valor `<script>` deve sair escapado.
- E2E: cada teste sobe servidor com DB temporário próprio (isolamento); asserts sobre estado persistido (recarregar página), não sobre animações.
- Drag-and-drop: cobrir via `dragTo` e via alternativa "Mover para…".
- Testes nunca são pulados ou apagados para ficar verde; corrigir causa raiz.
- Flakiness E2E: rodar 3× seguidas antes de checkpoint de fase.

## Smoke (`scripts/smoke.ts`)

`bun run smoke` builda e então roda `bun scripts/smoke.ts` contra `bin/kanban[.exe]`:

- `--version` imprime o conteúdo de `VERSION`.
- Sobe o binário com `--addr 127.0.0.1:0 --db <tmp>/smoke.db --log-level warn` e lê a porta da linha `LISTENING 127.0.0.1:<porta>` (timeout 10 s).
- `GET /healthz` → 200 `ok`; `GET /` → 200 `text/html` com `<title>`, estado vazio e headers `Content-Security-Policy`/`X-Content-Type-Options`; `GET /static/htmx.min.js` e `/static/app.css` → 200.
- `POST /boards` (`name=smoke`) → 303 para `/boards/<id>`; `GET` do quadro → 200 contendo `smoke`.
- Sempre mata o processo e remove o diretório temporário; em falha imprime `FAIL smoke: <check>: <motivo>` e sai com código 1; em sucesso imprime `PASS` com o tempo de cada check.

Testes dos scripts TypeScript (`scripts/*.test.ts`) rodam com `bun run test:scripts`.

## Metas de cobertura (checkpoint BETA)

- `internal/store` ≥ 80%.
- `app/controllers` (medido com os testes de `tests/` e `-coverpkg=./app/...`) ≥ 70%.
- Medir: `go test -cover ./internal/...`.

## Primeira execução em máquina limpa

```text
bun install
bunx playwright install chromium
bun run build
bun run test && bun run smoke && bun run e2e
```
