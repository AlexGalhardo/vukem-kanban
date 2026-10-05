# Roteiro agent-browser — smoke visual

Roteiro reproduzível para QA exploratório com o CLI `agent-browser` (skill `agent-browser`). Complementa os E2E do Playwright com inspeção visual e de acessibilidade.

## Preparação

```bash
bun run build
./bin/kanban --addr 127.0.0.1:8790 --db /tmp/qa/kanban.db --log-level debug
```

## Passos

1. `agent-browser open http://127.0.0.1:8790/` → espera o estado vazio ("Nenhum projeto ainda").
2. `agent-browser snapshot -i` → confirma os botões "Criar projeto", "Novo projeto" e "Buscar…" na árvore de acessibilidade.
3. Pressione `n`, preencha "Nome" com `QA smoke`, envie → o título do quadro mostra `QA smoke` e há 5 colunas.
4. Em "Backlog", digite `primeiro card` no campo "+ Novo card" e pressione Enter → card `QAS-1` aparece; contador da coluna = 1.
5. Abra o card, mude "Prioridade" para Urgente e salve com `Ctrl+Enter` → ícone laranja de urgente no card.
6. `agent-browser screenshot qa-light.png`; pressione `t`; `agent-browser screenshot qa-dark.png` → os dois temas legíveis, sem sobreposição.
7. `Ctrl+K`, digite `tema`, Enter → o tema alterna de volta.
8. Verifique o console (`agent-browser console`): nenhum erro de CSP ou exceção JS.
9. Encerre o servidor.

## Critério de aprovação

Todos os passos com o resultado esperado, nenhum erro no console e nenhum `"level":"ERROR"` no log do servidor.
