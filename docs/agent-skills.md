# Skills de agente (Claude Code)

Skills instaladas localmente em `.claude/skills/` (escopo do projeto). Referências compartilhadas do `agent-skills` em `.claude/references/` (as skills apontam para `../../references/*.md`).

| Skill | Origem | Quando usar |
| --- | --- | --- |
| `graphify` | [Graphify-Labs/graphify](https://github.com/Graphify-Labs/graphify) | Perguntas sobre o código: `graphify query "<pergunta>"`. Após mudar código: `graphify update .` |
| `karpathy-guidelines` | [multica-ai/andrej-karpathy-skills](https://github.com/multica-ai/andrej-karpathy-skills) | Ao escrever/revisar código: mudanças cirúrgicas, critérios verificáveis |
| `ponytail`, `ponytail-review`, `ponytail-audit`, `ponytail-debt`, `ponytail-gain`, `ponytail-help` | [dietrichgebert/ponytail](https://github.com/dietrichgebert/ponytail) | Combater over-engineering; preferir stdlib |
| `planning-and-task-breakdown`, `incremental-implementation`, `test-driven-development`, `code-review-and-quality`, `security-and-hardening`, `frontend-ui-engineering`, `git-workflow-and-versioning`, `shipping-and-launch`, `documentation-and-adrs`, e demais | [addyosmani/agent-skills](https://github.com/addyosmani/agent-skills) | Ciclo de engenharia; meta-skill `using-agent-skills` |

## Graphify

- CLI instalada via `uv tool install graphifyy` (v0.9.77); executável em `C:\Users\aleex\.local\bin\graphify`.
- Instalação no projeto: `graphify install --project --platform windows` (escreveu `.claude/skills/graphify`, hooks em `.claude/settings.json` e seção no `CLAUDE.md`).
- Saída em `graphify-out/` (no `.gitignore`).

## Atualizar skills

Clonar o repositório de origem e copiar a pasta da skill por cima da existente em `.claude/skills/`. Graphify: `uv tool upgrade graphifyy` e reinstalar com o comando acima.
