# Party deployment parity gap

The sandbox currently runs `mightyfin/party-platform:local`, originating from
`/srv/mightyfin/releases/20260804-efaas-sandbox/microservices/party-platform`.
This release contains role and relationship lifecycle code and migration 2
absent from repository revision a73c939. The live database has those columns.
Do not replace the running service with a build of a73c939: doing so could
remove existing lifecycle APIs.

An authenticated read-only call for Green returned HTTP 200 with keys `id`,
`party_type`, `display_name`, but no status. Payment Rails bank-owner approval
correctly rejects that incomplete proof. Documents supports both old `id` and
new `party_id` formats, requiring matching IDs if both are present.

Repository lookup now returns both identical ID fields and status, with a
disposable-PostgreSQL test for tenant/environment isolation, inactive aliases,
expired/future role relationships and closed identities. This change is NOT
deployed. It does not certify KYC approval or bank ownership.

Before deployment, reconcile the release's lifecycle implementation, routes,
tests and migration 2 into this repository, preserve existing consumers, run
regression tests and retest the connected owner lookup. Existing local changes
to Dockerfile, README, dependencies and CI were not included in this work.

## Repository reconciliation

Imported the deployed lifecycle domain, role/relationship routes, decoder tests,
validation tests and migration 2. Retained repository `GetForTenant` rather than
reintroducing the release's alias-only lookup. Added the deployed JWT `sub`
mapping and migration-2-compatible role creation to `Resolve`. Migration 1 is
identical to the release after line-ending normalization; database/config
differences were formatting-only. No live migration or deployment was performed.

Existing deployed behavior requiring follow-up is explicitly NOT certified by
this import: Resolve stamps role onboarding/verification values, role creation
and relationship creation require stronger resource-level tenant checks, and
cross-tenant participation transitions require explicit authority testing.
Migration 2 contains historical backfill and destructive Down operations; it
was imported as provenance, not executed or declared safe for a new environment.
Do not treat these role flags as proof of independent KYC or credit approval.
Run lifecycle PostgreSQL regressions and close authorization gaps before release.

Role assignment and relationship creation now require existing active scoped
membership (alias or currently effective role) for every affected party. The
write transaction locks the membership evidence and rejects restricted/closed
identities. A disposable PostgreSQL test proves foreign-tenant role creation and
relationship creation stop before insertion, and checks environment, restricted
identity, inactive alias and future-role rejection. Domain entry points also
validate request type and actor, not only HTTP handlers. Cross-tenant transition
authority and onboarding/verification provenance remain separate open checks.

Cross-programme transitions now require a tenant-free credential with matching
environment, `party-participation-transfer` role, `party.participation.transfer`
scope and the existing `party.roles.transition` scope. The domain command also
requires a transport-derived authority flag; JSON cannot set it. Tests prove
ordinary/tenant/wrong-environment callers and missing roles/scopes fail before
storage. No new role/scope was assigned to live users or clients. This restricts
the previously over-broad path rather than granting new financial authority.
Lifecycle requests also reject tenant/environment headers that contradict token
claims. Full authorised transition PostgreSQL regression and verification-state
provenance still need testing before deployment.

## Database regression and primary-route scope checks

The disposable PostgreSQL lifecycle regression now passes with both embedded
migrations applied inside a rolled-back private schema. It exercises identity
resolution retry, role suspension/reactivation, extra role assignment,
relationship creation/suspension/reactivation/end, rejection of reopening an
ended relationship, and authorised programme transfer. A wrong-party transfer
rolls back the source-role change. Successful operations produce exactly eleven
history records and eleven outbox records; rejected operations add neither.

Primary identity lookup and resolution routes now share the lifecycle scope
validation. Tests exercise the actual HTTP handler and prove that contradictory
tenant/environment claims return 403 before storage; matching scoped callers and
tenant-free service callers still reach input validation. Existing service
delegation is preserved; this is not certification of every delegation policy.

Validation: `go test ./...` with `PARTY_LOOKUP_TEST_DATABASE_URL` pointing to
disposable PostgreSQL passed, as did `go vet ./...`. These changes are not yet
deployed. Verification-state provenance, connected bank-account checks and the
remaining Green financial lifecycle are still open; no Green data was changed.

## Resolution does not manufacture compliance approval

New roles created by identity resolution now start `PENDING / UNVERIFIED`, rather
than the release's unconditional `APPROVED / IDENTITY_VERIFIED`. Matching or
registering an identity does not assess the new tenant/product relationship.
Existing roles and approvals are not backfilled or modified. The PostgreSQL
lifecycle regression asserts these initial values. All Party tests and vet pass;
the eFaaS backend Party client package tests also pass. That client consumes only
the resolution identity/outcome, not role approval flags.

This fixes resolution only. Explicit role assignment and participation transfer
still need verification-evidence provenance controls before they can be treated
as compliance decisions. They remain separate from lender approval and funding.
No deployment or Green financial write is claimed by this change.
