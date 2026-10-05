# Tooling

## Versões fixadas

Fixar versões exatas (sem `^`/`~`), sempre a última estável. Nunca `latest`, `next`, `rc`, `beta`.

| Ferramenta | Versão | Uso |
| --- | --- | --- |
| Go | 1.27.0 | backend |
| `modernc.org/sqlite` | v1.60.1 | driver SQLite pure Go |
| `golangci-lint` | 2.14.0 | lint + format Go |
| Bun | 1.4.2 | gerenciador de pacotes JS e task runner |
| `htmx.org` | 2.0.11 | vendorizado em `internal/web/static/htmx.min.js` |
| `tailwindcss` / `@tailwindcss/cli` | 4.3.3 | CSS |
| `electron` | 44.5.1 | desktop |
| `electron-builder` | 26.15.3 | instaladores |
| `@playwright/test` | 1.63.0 | E2E web e desktop |
| `agent-browser` | 0.38.2 | QA exploratório por agente |
| `gh` | instalado | releases |

Dependência nova (Go ou JS) só com aprovação do usuário.

## Comandos

Todos via `bun run <script>` (definidos em `package.json`).

| Comando | Faz |
| --- | --- |
| `bun run dev` | Tailwind em watch + `go run ./cmd/kanban` |
| `bun run build` | CSS minificado + `go build -o bin/kanban ./cmd/kanban` |
| `bun run lint` | `golangci-lint run ./...` + `golangci-lint fmt --diff` |
| `bun run fmt` | `golangci-lint fmt` (aplica gofumpt + goimports) |
| `bun run test` | `go test -race ./...` |
| `bun run smoke` | builda, sobe binário, checa `/healthz` e `/`, derruba |
| `bun run e2e` | Playwright web contra servidor real com DB temporário |
| `bun run e2e:desktop` | Playwright `_electron` |
| `bun run desktop` | Electron em modo dev |
| `bun run dist` | electron-builder |
| `bun run release <versão>` | release local completa (ver `docs/workflow.md`) |

Validação focada durante desenvolvimento: `go test ./internal/store/ -run TestNome`, `golangci-lint run ./internal/web/...`.

## Go: lint e formatação

`golangci-lint` v2 é a ferramenta única: linters e formatters no mesmo `.golangci.yml`.

- Formatters: `gofumpt` (superset estrito do `gofmt`) e `goimports`.
- Linters: `errcheck`, `govet`, `staticcheck`, `unused`, `ineffassign`, `gosec`, `revive`, `bodyclose`, `sqlclosecheck`, `rowserrcheck`, `errorlint`, `misspell`.
- `//nolint` só com linter específico e justificativa: `//nolint:gosec // motivo`.
- `govulncheck ./...` antes de releases.

## Tailwind v4

- Configuração CSS-first em `web/styles/input.css` (`@import "tailwindcss"`, `@source`, `@theme`). Sem `tailwind.config.js`.
- Saída: `internal/web/static/app.css` (gerado, no `.gitignore`, embutido no binário). Rode `bun run build` antes de `go build`.

## HTMX

- Arquivo vendorizado (sem CDN): app desktop precisa funcionar offline.
- Atualizar: mudar versão no `package.json`, `bun install`, `bun scripts/vendor-htmx.ts`.

## Editor

`.editorconfig`: tab com largura 4 por padrão; YAML e Markdown com 4 espaços (YAML proíbe tab). Fim de linha LF.

## Ambiente Windows

- Shell principal: PowerShell; Git Bash disponível.
- Graphify CLI em `C:\Users\aleex\.local\bin` (fora do PATH por padrão).
- Binários: `bin/kanban.exe` no Windows.
