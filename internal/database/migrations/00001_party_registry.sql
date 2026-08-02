-- +goose Up
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE parties (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  public_id VARCHAR(64) NOT NULL UNIQUE,
  party_type VARCHAR(32) NOT NULL CHECK (party_type IN ('individual','organization','organizational_unit')),
  display_name VARCHAR(200) NOT NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'active' CHECK (status IN ('provisional','active','restricted','merged','closed')),
  merged_into_id UUID REFERENCES parties(id),
  version BIGINT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK ((status='merged') = (merged_into_id IS NOT NULL))
);

CREATE TABLE party_aliases (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  public_id VARCHAR(64) NOT NULL UNIQUE,
  party_id UUID NOT NULL REFERENCES parties(id),
  tenant_id VARCHAR(64) NOT NULL,
  environment VARCHAR(24) NOT NULL,
  source_system VARCHAR(80) NOT NULL,
  external_reference VARCHAR(120) NOT NULL,
  status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active','suspended','retired')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, environment, source_system, external_reference)
);
CREATE INDEX party_aliases_party_idx ON party_aliases(party_id);

CREATE TABLE party_identifiers (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  party_id UUID NOT NULL REFERENCES parties(id),
  scheme VARCHAR(64) NOT NULL,
  value_token CHAR(64) NOT NULL,
  verification_status VARCHAR(20) NOT NULL CHECK (verification_status IN ('asserted','verified')),
  source_system VARCHAR(80) NOT NULL,
  purpose VARCHAR(120) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (scheme, value_token)
);

CREATE TABLE party_roles (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  party_id UUID NOT NULL REFERENCES parties(id),
  tenant_id VARCHAR(64) NOT NULL,
  environment VARCHAR(24) NOT NULL,
  role_type VARCHAR(80) NOT NULL,
  product VARCHAR(80) NOT NULL,
  status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active','suspended','ended')),
  effective_from TIMESTAMPTZ NOT NULL DEFAULT now(),
  effective_to TIMESTAMPTZ,
  UNIQUE (party_id, tenant_id, environment, role_type, product)
);

CREATE TABLE party_relationships (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  public_id VARCHAR(64) NOT NULL UNIQUE,
  from_party_id UUID NOT NULL REFERENCES parties(id),
  to_party_id UUID NOT NULL REFERENCES parties(id),
  relationship_type VARCHAR(80) NOT NULL,
  tenant_id VARCHAR(64) NOT NULL,
  environment VARCHAR(24) NOT NULL,
  status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active','suspended','ended')),
  effective_from TIMESTAMPTZ NOT NULL DEFAULT now(),
  effective_to TIMESTAMPTZ,
  CHECK (from_party_id <> to_party_id)
);

CREATE TABLE match_reviews (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  public_id VARCHAR(64) NOT NULL UNIQUE,
  tenant_id VARCHAR(64) NOT NULL,
  environment VARCHAR(24) NOT NULL,
  source_system VARCHAR(80) NOT NULL,
  external_reference VARCHAR(120) NOT NULL,
  candidate_party_ids UUID[] NOT NULL,
  reason VARCHAR(200) NOT NULL,
  status VARCHAR(20) NOT NULL DEFAULT 'open' CHECK (status IN ('open','resolved','dismissed')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  resolved_at TIMESTAMPTZ
);

CREATE TABLE party_history (
  sequence BIGSERIAL PRIMARY KEY,
  public_id VARCHAR(64) NOT NULL UNIQUE,
  party_id UUID REFERENCES parties(id),
  action VARCHAR(80) NOT NULL,
  actor_subject VARCHAR(160) NOT NULL,
  tenant_id VARCHAR(64) NOT NULL,
  correlation_id VARCHAR(128) NOT NULL,
  payload JSONB NOT NULL,
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE outbox_events (
  sequence BIGSERIAL PRIMARY KEY,
  public_id VARCHAR(64) NOT NULL UNIQUE,
  aggregate_id VARCHAR(64) NOT NULL,
  event_type VARCHAR(100) NOT NULL,
  tenant_id VARCHAR(64) NOT NULL,
  correlation_id VARCHAR(128) NOT NULL,
  payload JSONB NOT NULL,
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  published_at TIMESTAMPTZ
);
CREATE INDEX outbox_unpublished_idx ON outbox_events(sequence) WHERE published_at IS NULL;

CREATE RULE party_history_no_update AS ON UPDATE TO party_history DO INSTEAD NOTHING;
CREATE RULE party_history_no_delete AS ON DELETE TO party_history DO INSTEAD NOTHING;

-- +goose Down
DROP TABLE IF EXISTS outbox_events, party_history, match_reviews, party_relationships, party_roles, party_identifiers, party_aliases, parties;
