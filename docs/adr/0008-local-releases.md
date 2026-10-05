# ADR 0008: Releases locais sem GitHub Actions

- Status: aceita
- Data: 2026-10-05

## Contexto

O GitHub Actions está indisponível por billing.

## Decisão

`bun run release <versão>` valida a árvore, roda lint, testes e smoke, gera binários Go para 5 plataformas e os instaladores Windows, cria `SHA256SUMS`, move o `[Unreleased]` do CHANGELOG, cria a tag e publica com `gh release create` (`--prerelease` para `-alpha`, `-beta` e `-rc`).

## Consequências

Releases reprodutíveis a partir da máquina de desenvolvimento. Instaladores Linux e macOS do Electron não são gerados localmente; os binários Go dessas plataformas são.
