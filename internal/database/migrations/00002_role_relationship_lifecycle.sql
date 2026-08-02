-- +goose Up
ALTER TABLE party_roles DROP CONSTRAINT party_roles_party_id_tenant_id_environment_role_type_produc_key;
ALTER TABLE party_roles
  ADD COLUMN public_id VARCHAR(64),
  ADD COLUMN classification VARCHAR(80),
  ADD COLUMN channel VARCHAR(40),
  ADD COLUMN onboarding_status VARCHAR(24),
  ADD COLUMN verification_level VARCHAR(24),
  ADD COLUMN consent_reference VARCHAR(120),
  ADD COLUMN eligibility_reference VARCHAR(120),
  ADD COLUMN status_reason VARCHAR(240),
  ADD COLUMN created_by VARCHAR(160),
  ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

UPDATE party_roles SET
  public_id = 'pro_' || encode(gen_random_bytes(16), 'hex'),
  classification = 'EFAAS_PARTICIPANT',
  channel = 'PARTNER',
  onboarding_status = 'APPROVED',
  verification_level = 'PRODUCT_VERIFIED',
  role_type = 'CUSTOMER',
  product = upper(product),
  created_by = 'migration:00002';

ALTER TABLE party_roles
  ALTER COLUMN public_id SET NOT NULL,
  ALTER COLUMN classification SET NOT NULL,
  ALTER COLUMN channel SET NOT NULL,
  ALTER COLUMN onboarding_status SET NOT NULL,
  ALTER COLUMN verification_level SET NOT NULL,
  ALTER COLUMN created_by SET NOT NULL,
  ADD CONSTRAINT party_roles_public_id_unique UNIQUE(public_id),
  ADD CONSTRAINT party_roles_role_type_check CHECK (role_type IN ('CUSTOMER','PARTNER','SUPPLIER','EMPLOYEE','AGENT','INVESTOR_FUNDER','REGULATOR')),
  ADD CONSTRAINT party_roles_channel_check CHECK (channel IN ('DIRECT','PARTNER','INTERNAL','EXTERNAL')),
  ADD CONSTRAINT party_roles_onboarding_check CHECK (onboarding_status IN ('PENDING','UNDER_REVIEW','APPROVED','REJECTED')),
  ADD CONSTRAINT party_roles_verification_check CHECK (verification_level IN ('UNVERIFIED','IDENTITY_VERIFIED','PRODUCT_VERIFIED','ENHANCED_DUE_DILIGENCE')),
  ADD CONSTRAINT party_roles_effective_dates_check CHECK (effective_to IS NULL OR effective_to >= effective_from);

CREATE UNIQUE INDEX party_roles_current_unique
  ON party_roles(party_id,tenant_id,environment,role_type,product,classification)
  WHERE status IN ('active','suspended');
CREATE INDEX party_roles_party_time_idx ON party_roles(party_id,effective_from DESC);

ALTER TABLE party_relationships
  ADD COLUMN classification VARCHAR(80) NOT NULL DEFAULT 'UNSPECIFIED',
  ADD COLUMN status_reason VARCHAR(240),
  ADD COLUMN created_by VARCHAR(160) NOT NULL DEFAULT 'migration:00002',
  ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  ADD CONSTRAINT party_relationships_type_check CHECK (relationship_type IN ('EMPLOYED_BY','SUPPLIES','AGGREGATED_BY','MEMBER_OF','REPRESENTS','FUNDS','REGULATES','SERVES','INTEGRATES_WITH')),
  ADD CONSTRAINT party_relationships_dates_check CHECK (effective_to IS NULL OR effective_to >= effective_from);
CREATE UNIQUE INDEX party_relationships_current_unique
  ON party_relationships(from_party_id,to_party_id,tenant_id,environment,relationship_type,classification)
  WHERE status IN ('active','suspended');
CREATE INDEX party_relationships_from_idx ON party_relationships(from_party_id,effective_from DESC);
CREATE INDEX party_relationships_to_idx ON party_relationships(to_party_id,effective_from DESC);

-- +goose Down
DROP INDEX IF EXISTS party_relationships_to_idx;
DROP INDEX IF EXISTS party_relationships_from_idx;
DROP INDEX IF EXISTS party_relationships_current_unique;
ALTER TABLE party_relationships DROP CONSTRAINT IF EXISTS party_relationships_dates_check, DROP CONSTRAINT IF EXISTS party_relationships_type_check,
  DROP COLUMN IF EXISTS updated_at, DROP COLUMN IF EXISTS created_by, DROP COLUMN IF EXISTS status_reason, DROP COLUMN IF EXISTS classification;
DROP INDEX IF EXISTS party_roles_party_time_idx;
DROP INDEX IF EXISTS party_roles_current_unique;
ALTER TABLE party_roles DROP CONSTRAINT IF EXISTS party_roles_effective_dates_check, DROP CONSTRAINT IF EXISTS party_roles_verification_check,
  DROP CONSTRAINT IF EXISTS party_roles_onboarding_check, DROP CONSTRAINT IF EXISTS party_roles_channel_check,
  DROP CONSTRAINT IF EXISTS party_roles_role_type_check, DROP CONSTRAINT IF EXISTS party_roles_public_id_unique,
  DROP COLUMN IF EXISTS updated_at, DROP COLUMN IF EXISTS created_by, DROP COLUMN IF EXISTS status_reason,
  DROP COLUMN IF EXISTS eligibility_reference, DROP COLUMN IF EXISTS consent_reference, DROP COLUMN IF EXISTS verification_level,
  DROP COLUMN IF EXISTS onboarding_status, DROP COLUMN IF EXISTS channel, DROP COLUMN IF EXISTS classification, DROP COLUMN IF EXISTS public_id;
ALTER TABLE party_roles ADD CONSTRAINT party_roles_party_scope_product_unique UNIQUE(party_id,tenant_id,environment,role_type,product);
