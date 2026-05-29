# Security Policy

Vertix takes the security of `vertix-testnet-1` and the custom protocol modules (`x/oracle`, `x/rwa`, `x/fees`) seriously. This document describes how to report vulnerabilities responsibly.

## Supported Versions

| Version | Supported |
|---------|-----------|
| Latest `vertix-testnet-1` release tag | ✅ |
| `main` branch (pre-release) | ✅ best-effort |
| Older testnet tags | ❌ |

## How to Report

**Do not open a public GitHub issue for security vulnerabilities.**

Instead:

1. **GitHub Security Advisory (preferred):** [Create a private advisory](https://github.com/vertix-network/vertix/security/advisories/new) on this repository.
2. **Email:** `security@vertix.network` (PGP key published on request).

Include:

- Description of the vulnerability and impact
- Steps to reproduce (testnet-only; see [bug bounty rules](./docs/bug-bounty.md))
- Affected module(s) and commit/tag
- Proof-of-concept if available

## Response Timeline

| Stage | Target SLA |
|-------|------------|
| Initial acknowledgement | 48 hours |
| Severity triage | 5 business days |
| Fix timeline (Critical/High) | Coordinated disclosure; target 30 days |
| Public disclosure | After fix deployed + validators notified |

We commit to **coordinated disclosure**: we will not publicly disclose your report before a fix is available, and we will credit researchers who follow these rules (unless you prefer anonymity).

## Bug Bounty

Scope, severity rubric, reward tiers, and safe-harbor rules are documented in [`docs/bug-bounty.md`](./docs/bug-bounty.md).

## Out of Scope

See the full list in [`docs/bug-bounty.md`](./docs/bug-bounty.md#scope). In summary: standard unmodified SDK modules, testnet infrastructure DoS, and third-party price-provider APIs are out of scope.
