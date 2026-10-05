# ADR 0002: SQLite via modernc.org/sqlite (sem CGO)

- Status: aceita
- Data: 2026-10-05

## Contexto

`mattn/go-sqlite3` exige CGO e um toolchain C para cada plataforma alvo.

## Decisão

Usar `modernc.org/sqlite` (SQLite transpilado para Go), com `journal_mode=WAL`, `foreign_keys=ON`, `busy_timeout=5000` e `SetMaxOpenConns(1)`.

## Consequências

Cross-compile trivial (`CGO_ENABLED=0`) para Windows, Linux e macOS, e o build do Electron não precisa de compilador C. Custo: binário um pouco maior e desempenho um pouco menor que o SQLite nativo, irrelevantes neste uso.
