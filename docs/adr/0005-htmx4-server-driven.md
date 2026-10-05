# ADR 0005: htmx 4 com HTML server-driven

- Status: aceita
- Data: 2026-10-05

## Contexto

Uma SPA exigiria API JSON, estado no cliente e build JS complexo.

## Decisão

Handlers devolvem fragmentos HTML. `HX-Request-Type: partial` recebe fragmento; requisições `full` e navegação direta recebem a página. Erros de htmx voltam com `HX-Reswap: none` e um toast no header `X-Toast` (JSON escapado em ASCII). Elementos OOB nunca ficam dentro do conteúdo principal da mesma resposta.

## Consequências

O frontend é HTML + ~400 linhas de JS. As regras específicas do htmx 4 (herança explícita, swap de respostas de erro, nomes de eventos) estão documentadas em `docs/architecture.md`.
