# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `PLAN.md` com plano de implementação por fases (ESTRUTURA, ALPHA, BETA, 1.0.0 RC).
- Documentação do projeto em `docs/` (arquitetura, tooling, workflow, testes, skills) indexada em `AGENTS.md` e `CLAUDE.md`.
- Skills de agente locais em `.claude/skills/`: graphify, karpathy-guidelines, ponytail, addyosmani/agent-skills.
- `.editorconfig`, `.gitattributes` e `.gitignore`.
- Servidor Go (`cmd/kanban`) só com stdlib + `modernc.org/sqlite`: flags `--addr`, `--db`, `--log-level`, `--version`; handshake `LISTENING <addr>`; shutdown gracioso; `/healthz`.
- Banco SQLite com migrações embutidas (WAL, foreign keys, backup automático antes de migrar).
- Dashboard kanban em `/`: projetos, colunas padrão, cards com prioridade, entrega, descrição e labels; drag-and-drop nativo e "Status" no card como alternativa por teclado.
- UI no estilo Linear com Tailwind CSS v4 e htmx 4.0.0 (vendorizado, offline): tema claro/escuro, command palette (Ctrl+K), atalhos de teclado, toasts.
- Busca e filtros (label, prioridade, vencidos) refletidos na URL; limites WIP por coluna; arquivar/restaurar/excluir projetos.
- Exportar/importar JSON (mesclar ou substituir) e backup do banco (`VACUUM INTO`).
- Segurança: CSP estrita sem scripts inline, proteção CSRF (`http.CrossOriginProtection`), limite de tamanho de body, escaping via `html/template`.
- Testes: unitários/integração Go (`go test -race`), E2E web com Playwright (22 cenários) e logs estruturados `slog` por request.
- `scripts/smoke.ts` (`bun run smoke`): sobe o binário com DB temporário e porta aleatória e valida `--version`, `/healthz`, `/`, estáticos, headers de segurança e criação de quadro.
- `scripts/release.ts` (`bun run release <versão> [--dry-run] [--skip-desktop]`): release local com bump de `VERSION`/`package.json`, rotação do CHANGELOG, gates, binários Go para 5 plataformas, instalador desktop opcional, `SHA256SUMS`, tag e `gh release create`.
- Testes unitários dos helpers de release em `scripts/release.test.ts` (`bun run test:scripts`).
- App desktop Electron (`desktop/`): sobe o binário Go como sidecar em porta aleatória, abre a janela no endereço do handshake `LISTENING`, encerra o sidecar ao sair e mostra erro se ele morrer ou não iniciar em 15 s.
- Desktop: janela com `contextIsolation`, `sandbox`, sem `nodeIntegration`; navegação para fora do servidor bloqueada e links externos abertos no navegador.
- Desktop: single instance, tamanho/posição da janela persistidos, menu nativo pt-BR (backup, abrir pasta de dados, zoom, Sobre com versão) e ícone do app.
- Desktop: logs do sidecar em `userData/logs/kanban.log` com rotação simples (5 MB); `KANBAN_DATA_DIR` sobrescreve a pasta de dados.
- Empacotamento com electron-builder (`desktop/electron-builder.yml`): instalador NSIS e executável portable para Windows x64.
- E2E desktop (`e2e/desktop/app.spec.ts`): dashboard carrega, `kanban.db` criado, backup, sem `kanban.exe` órfão após fechar e segunda instância.
