# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- ADRs em `docs/adr/` (D1–D9) e screenshots no README.
- Orçamentos de performance em teste: 500 cards renderizam em ~14 ms e mover um card leva ~6 ms (metas: 200 ms e 50 ms).
- Teste de compatibilidade: banco e export gerados pela `v0.4.0-alpha.1` abrem, migram e importam sem perda.

### Fixed

- Export JSON gravava as labels com chaves `Name`/`Color`; agora usa `name`/`color` (exports antigos continuam importando).

## [0.4.0-alpha.1] - 2026-10-05

### Added

- E2E desktop do fluxo completo (criar projeto, card, mover, editar, arquivar) dentro da janela Electron.
- Specs de regressão do QA adversarial (`e2e/web/qa-regressions.spec.ts`).

### Changed

- Mover card envia o card de referência (`before_id`) e a resposta traz só as colunas afetadas (menos HTML por arrasto em quadros grandes).
- Mensagens de erro em português com o nome do campo ("Título: obrigatório"); erros de JSON no import sem detalhes internos.
- Navegação de histórico (voltar/avançar) recarrega a página.
- Busca por texto troca a entrada do histórico em vez de empilhar uma por pausa de digitação.
- Contraste dos textos secundários ajustado para WCAG AA nos dois temas.

### Fixed

- Quadro ficava sem colunas após navegar pela sidebar, criar ou salvar um projeto (htmx 4 extraía `#columns` como OOB).
- Toasts com acento chegavam corrompidos ("NÃ£o") e toasts de ações em diálogos não apareciam.
- Filtro de label não recebia labels novas; coluna com cards ocultos pelo filtro não podia ser excluída.
- Arrastar com filtro ativo gravava a posição errada; falha ao mover agora restaura o quadro.
- Criar/excluir card com filtro ativo deixava contadores e resumo inconsistentes.
- Dois backups no mesmo segundo davam erro 500; import "Substituir tudo" agora pede confirmação e faz backup antes.
- Envio duplo (Enter/clique rápido) criava projetos, cards, colunas e labels duplicados.
- Busca não ignorava maiúsculas em letras acentuadas.
- Sidebar em tela estreita não fechava; diálogos não fechavam ao clicar fora; card focado pelo teclado sem foco visível.
- Limite WIP não numérico era zerado em silêncio; título com quebra de linha; corpo grande retornava 400 em vez de 413.
- Import aceitava conteúdo extra após o JSON; rotas inexistentes e IDs inválidos mostravam texto cru em vez da página.
- Limpeza intermitente de diretórios temporários nos testes Go no Windows.

## [0.0.1] - 2026-10-05

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

[Unreleased]: https://github.com/AlexGalhardo/trello-golang-htmx/compare/v0.4.0-alpha.1...HEAD
[0.4.0-alpha.1]: https://github.com/AlexGalhardo/trello-golang-htmx/compare/v0.0.1...v0.4.0-alpha.1
[0.0.1]: https://github.com/AlexGalhardo/trello-golang-htmx/releases/tag/v0.0.1
