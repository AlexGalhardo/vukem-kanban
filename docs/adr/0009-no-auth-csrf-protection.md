# ADR 0009: Sem autenticação; CSRF via stdlib

- Status: aceita
- Data: 2026-10-05

## Contexto

O app é single-user e local, mas uma página maliciosa aberta no navegador pode enviar POSTs para `127.0.0.1`.

## Decisão

Sem login. O servidor escuta só em `127.0.0.1` e usa `http.CrossOriginProtection` (Go 1.25+), que rejeita requisições não seguras cross-site. A CSP é estrita (sem script ou estilo inline), o tamanho do body é limitado e o escaping fica com `html/template`.

## Consequências

Proteção contra CSRF e XSS sem tokens nem sessão. Qualquer processo local ainda pode chamar a API, aceitável para uma ferramenta pessoal.
