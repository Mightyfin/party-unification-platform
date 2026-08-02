# Party Platform boundaries

| Concern | System of record |
|---|---|
| Authentication, credentials, sessions, MFA | Identity Platform / Keycloak |
| Person, organisation, organisational unit, global party ID | Party Platform |
| Tenant/source alias and relationship | Party Platform |
| Consent grant and permitted purpose | Consent service (future); Party records a purpose reference only |
| KYC evidence and documents | KYC / Document services |
| Wallet and account balances | Wallet/Ledger |
| Credit applications and facilities | Lending/Risk domains |
| Partner product configuration | EFaaS / Embedded Finance product |
| Golden-record stewardship UI | Optional MDM/Pimcore adapter, never runtime critical path |

EFaaS resolves a participant before accepting it. A successful resolution returns a stable `party_id`; EFaaS stores that ID with its own tenant relationship and product state. Repeating the same alias is idempotent. A verified identifier shared across channels can resolve to the same global party, while the aliases remain isolated.

Business roles are not authorization grants. `EMPLOYEE` records an employment/business relationship; Keycloak and the entitlement service independently determine whether that person is an operations agent, credit officer, platform administrator or another privileged user.

Cross-tenant product transitions require the dedicated `party.roles.transition` scope and are atomic. They preserve the source participation as ended history and require new consent and eligibility references before activating the target participation.
