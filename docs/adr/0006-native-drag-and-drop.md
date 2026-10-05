# ADR 0006: Drag-and-drop nativo ancorado em card

- Status: aceita
- Data: 2026-10-05

## Contexto

Bibliotecas como SortableJS resolveriam o arrasto, mas seriam mais uma dependência.

## Decisão

Usar a API HTML5 de drag-and-drop. O cliente envia `column_id` e `before_id` (o card visível acima do qual soltou). O servidor reposiciona em transação, mantendo `position` contígua, e responde só com as colunas afetadas.

## Consequências

Ancorar no card em vez de num índice mantém o resultado certo com filtros ativos. Teclado e leitores de tela usam o select "Status" no diálogo do card.
