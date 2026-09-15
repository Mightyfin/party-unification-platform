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
