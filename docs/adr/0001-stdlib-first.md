# ADR 0001: Stdlib primeiro no backend Go

- Status: aceita
- Data: 2026-10-05

## Contexto

O projeto é didático e pequeno. Cada dependência adiciona superfície de ataque, atualização e aprendizado de API alheia.

## Decisão

Usar só a biblioteca padrão do Go (`net/http` com padrões de rota do Go 1.22+, `html/template`, `log/slog`, `embed`, `database/sql`, `testing`). A única dependência de runtime é o driver SQLite.

## Consequências

Menos código de terceiros e builds reprodutíveis. Roteamento, migrações e validação foram escritos à mão, em poucas dezenas de linhas cada.
