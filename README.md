# MightyFin Party Platform

Operational party and relationship authority shared by EFaaS and MightyFin's direct products.

It answers **who is this party?** independently of login identity (Keycloak), product enrolment, wallet ownership, lending facilities, and optional stewardship tools. It owns global party IDs, tenant/source aliases, deterministic identifier matching, roles, relationships, review cases, provenance, immutable history, and outbox events.

## Safety rules

- Tenant external references are aliases, never global identity keys.
- Only exact, normalized identifiers may auto-match. Fuzzy matches require review.
- Raw identifiers are not persisted; the service stores an HMAC token using a deployment secret.
- A party is global, but aliases and product participation remain tenant/environment scoped.
- Merges are never performed automatically.
- Production requires OIDC. `PARTY_AUTH_DISABLED=true` is accepted only in `local` or `sandbox`.

## Local run

```powershell
docker compose up --build
```

API: `http://localhost:18086`. PostgreSQL: `localhost:15436`.

See `docs/BOUNDARIES.md` for ownership and integration rules.

## Canonical business roles

`CUSTOMER`, `PARTNER`, `SUPPLIER`, `EMPLOYEE`, `AGENT`, `INVESTOR_FUNDER`, and `REGULATOR` are effective-dated business roles. A party may hold several concurrently. Staff administration permissions remain in Keycloak and are never inferred from `EMPLOYEE`.

EFaaS participants are represented as `CUSTOMER / EFAAS / PARTNER`. Ecosystem facts such as employment, supply, aggregation, funding and regulation are effective-dated party relationships. Moving from an EFaaS tenant into MightyFin direct Embedded Finance ends only the source participation and creates a new direct participation with independent consent and eligibility evidence.
