# ADR 0007: Bun como gerenciador de pacotes e task runner

- Status: aceita
- Data: 2026-10-05

## Contexto

Windows não tem `make`. Bun já é necessário para Tailwind, Playwright e Electron.

## Decisão

Todos os comandos ficam em `package.json` (`bun run dev/build/lint/test/smoke/e2e/dist/release`). Scripts mais complexos ficam em `scripts/*.ts` com TypeScript estrito.

## Consequências

Um único ponto de entrada multiplataforma. Quem só quer o servidor ainda pode usar `go build` direto.
