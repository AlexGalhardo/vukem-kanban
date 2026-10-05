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
- App desktop Electron (`desktop/`): sobe o binário Go como sidecar em porta aleatória, abre a janela no endereço do handshake `LISTENING`, encerra o sidecar ao sair e mostra erro se ele morrer ou não iniciar em 15 s.
- Desktop: janela com `contextIsolation`, `sandbox`, sem `nodeIntegration`; navegação para fora do servidor bloqueada e links externos abertos no navegador.
- Desktop: single instance, tamanho/posição da janela persistidos, menu nativo pt-BR (backup, abrir pasta de dados, zoom, Sobre com versão) e ícone do app.
- Desktop: logs do sidecar em `userData/logs/kanban.log` com rotação simples (5 MB); `KANBAN_DATA_DIR` sobrescreve a pasta de dados.
- Empacotamento com electron-builder (`desktop/electron-builder.yml`): instalador NSIS e executável portable para Windows x64.
- E2E desktop (`e2e/desktop/app.spec.ts`): dashboard carrega, `kanban.db` criado, backup, sem `kanban.exe` órfão após fechar e segunda instância.
