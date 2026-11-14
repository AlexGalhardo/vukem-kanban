# Security policy

This file covers reporting vulnerabilities in vukem-kanban. The design of the protections (loopback-only server, escaped Markdown, the Electron ↔ Go sidecar contract) lives in [docs/architecture.md](docs/architecture.md) and the [ADRs](docs/adr/README.md).

## Scope and limits

vukem-kanban is a **single-user, local-first kanban**. It has no accounts and no authentication: the server is meant to listen on `127.0.0.1` (web) or on a random loopback port spawned by the desktop app. Reports about the following are in scope:

- Cross-site scripting through card titles, descriptions (Markdown), labels or imported JSON.
- Cross-site request forgery or DNS rebinding against the local server.
- Unsafe JSON import or database restore (path traversal, oversized payloads, SQL injection).
- Electron hardening issues: node integration, context isolation, navigation or `window.open` escapes.
- Supply-chain issues in the build, release or desktop installer workflows.

Exposing the server on a public interface without a reverse proxy and authentication is outside the supported setup.

## Supported versions

| Version | Supported |
| --- | --- |
| Latest `v0` minor (pre-1.0) | Security fixes ship as the next `v0` patch or minor |
| Older `v0` minors | Upgrade to the latest `v0` minor |

## Reporting a vulnerability

**Do not open a public issue.**

Report privately through GitHub: **Security → Report a vulnerability** at <https://github.com/AlexGalhardo/vukem-kanban/security/advisories/new>.

Please include:

- Affected version or commit
- A description of the issue and its impact
- Steps or code to reproduce
- Any suggested fix

## What to expect

| Step | Target time |
| --- | --- |
| Acknowledgement | 3 business days |
| Initial assessment | 7 days |
| Fix released for confirmed issues | 30 days, faster for critical issues |

The project has a single maintainer, so these targets are best effort; a confirmed critical issue takes priority over all other work.

## Disclosure policy

The project follows coordinated vulnerability disclosure: the issue is confirmed, a timeline is agreed with the reporter (90 days at most from the report), the fix is prepared privately, and the advisory and the patched release are published together. Please keep the vulnerability private until the advisory is public. Reporters are credited unless they prefer otherwise.

## Supply chain

Workflows pin every action by full commit SHA and request minimal permissions. Dependencies are scanned with `govulncheck` and `bun audit`, and release images are signed with cosign and carry a provenance attestation.
