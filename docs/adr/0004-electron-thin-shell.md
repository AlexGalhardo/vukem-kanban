# ADR 0004: Electron como casca fina com sidecar Go

- Status: aceita
- Data: 2026-10-05

## Contexto

Queríamos um app desktop sem duplicar lógica em JavaScript.

## Decisão

O processo principal do Electron inicia o binário Go com `--addr 127.0.0.1:0`, lê a linha `LISTENING <addr>` e abre a janela nessa URL. Ao sair, encerra o sidecar. A janela usa `contextIsolation`, `sandbox`, sem `nodeIntegration`, e bloqueia navegação externa.

## Consequências

Toda a lógica fica em Go e é testada uma vez. O instalador fica grande (~115 MB, por causa do Chromium), aceitável para uso pessoal.
