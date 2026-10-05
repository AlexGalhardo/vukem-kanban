# ADR 0003: Binário único com go:embed

- Status: aceita
- Data: 2026-10-05

## Contexto

O app precisa funcionar offline (desktop) e ser simples de distribuir.

## Decisão

Templates, CSS gerado, `htmx.min.js` vendorizado e o JS do app são embutidos com `go:embed`. Assets são servidos com `Cache-Control: immutable` e `?v=<versão>`.

## Consequências

Um único arquivo executável serve o app web completo. O CSS precisa ser gerado (`bun run css`) antes do `go build`.
