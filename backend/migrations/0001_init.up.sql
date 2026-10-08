-- FlowOS Hub schema v1.1 — generated from SPEC.md §3 (authoritative)

CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE tenants (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  slug                CITEXT UNIQUE NOT NULL,        -- used in signup links & app login
  name                TEXT NOT NULL,
  is_house            BOOLEAN NOT NULL DEFAULT FALSE,
  grace_working_days  INT NOT NULL DEFAULT 5 CHECK (grace_working_days BETWEEN 0 AND 30),
  status              TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','SUSPENDED')),
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE products (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  code        CITEXT UNIQUE NOT NULL,        -- 'flowos'; used in API requests & CSV
  name        TEXT NOT NULL,
  key_prefix  TEXT NOT NULL,                 -- 'FL' → keys look like FL-XXXX-…
  status      TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','RETIRED')),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE tenant_products (                -- which products a tenant may distribute
  tenant_id   UUID NOT NULL REFERENCES tenants(id),
  product_id  UUID NOT NULL REFERENCES products(id),
  PRIMARY KEY (tenant_id, product_id)
);

CREATE TABLE users (
  id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id                UUID NOT NULL REFERENCES tenants(id),
  role                     TEXT NOT NULL CHECK (role IN ('ADMIN','ALGO_USER')),
  username                 CITEXT NOT NULL,
  first_name               TEXT NOT NULL,
  last_name                TEXT NOT NULL,
  password_hash            TEXT NOT NULL,             -- argon2id
  mobile                   TEXT NOT NULL,             -- E.164, e.g. +91XXXXXXXXXX
  mobile_verified          BOOLEAN NOT NULL DEFAULT FALSE,
  email                    CITEXT,                    -- optional
  city                     TEXT,
  state                    TEXT,
  broker_client_code       TEXT,                      -- the user's ID at the broker (optional)
  referral_code            TEXT,                      -- optional, free text V1
  status                   TEXT NOT NULL DEFAULT 'PENDING_VERIFICATION'
                           CHECK (status IN ('PENDING_VERIFICATION','ACTIVE','DEACTIVATED','BANNED')),
  imported                 BOOLEAN NOT NULL DEFAULT FALSE,
  import_batch_id          UUID,
  created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, username),
  UNIQUE (tenant_id, mobile)
);

CREATE TABLE licenses (
  id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id                UUID NOT NULL REFERENCES tenants(id),
  user_id                  UUID NOT NULL REFERENCES users(id),
  product_id               UUID NOT NULL REFERENCES products(id),
  license_key_hash         TEXT NOT NULL,             -- argon2id; key shown ONCE at creation
  license_key_hint         TEXT NOT NULL,             -- last 4 chars, for support convos
  status                   TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','DISABLED')),
                                                      -- DISABLED = this product revoked without banning the user
  subscription_valid_until DATE,                      -- denormalized from payments; NULL = never paid
  entitlements             JSONB NOT NULL DEFAULT '{"max_activations":1,"tier":"RETAIL"}',
  created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (user_id, product_id)
);

CREATE TABLE payments (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id           UUID NOT NULL REFERENCES tenants(id),
  user_id             UUID NOT NULL REFERENCES users(id),      -- kept for query convenience
  license_id          UUID NOT NULL REFERENCES licenses(id),   -- the product-subscription this payment extends
  amount_minor_units  BIGINT NOT NULL CHECK (amount_minor_units >= 0),  -- paise; COMPLIMENTARY = 0
  currency            TEXT NOT NULL DEFAULT 'INR',
  method              TEXT NOT NULL CHECK (method IN ('MANUAL','COMPLIMENTARY','GATEWAY')),
  gateway_payment_id  TEXT,                           -- NULL until gateway exists
  plan                TEXT NOT NULL CHECK (plan IN ('MONTHLY','QUARTERLY','ANNUAL')),
  paid_at             DATE NOT NULL,
  valid_from          DATE NOT NULL,
  valid_until         DATE NOT NULL,                  -- valid_from + plan duration
  recorded_by         UUID NOT NULL REFERENCES users(id),  -- the admin who entered it
  note                TEXT,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- On INSERT (same transaction): licenses.subscription_valid_until =
-- GREATEST(current value, valid_until). Never decreases automatically;
-- an admin correcting a mistake does so via an explicit adjustment
-- payment row or a status change — both audited.

CREATE TABLE otp_codes (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     UUID NOT NULL REFERENCES users(id),
  purpose     TEXT NOT NULL CHECK (purpose IN ('REGISTRATION','FIRST_LOGIN','PASSWORD_RESET')),
  code_hash   TEXT NOT NULL,                          -- 6 digits, hashed; NEVER logged
  expires_at  TIMESTAMPTZ NOT NULL,                   -- now() + 5 minutes
  attempts    INT NOT NULL DEFAULT 0,                 -- max 5, then invalidated
  consumed_at TIMESTAMPTZ
);

CREATE TABLE sessions (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     UUID NOT NULL REFERENCES users(id),
  audience    TEXT NOT NULL CHECK (audience IN ('DASHBOARD','APP')),
  token_hash  TEXT NOT NULL,                          -- random 256-bit, sha256-stored
  expires_at  TIMESTAMPTZ NOT NULL,                   -- dashboard 12h, app 30d
  ip          TEXT,
  user_agent  TEXT,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE audit_log (
  id              BIGSERIAL PRIMARY KEY,
  tenant_id       UUID NOT NULL,
  actor_user_id   UUID,                               -- NULL = system
  action          TEXT NOT NULL,   -- e.g. USER_STATUS_CHANGED, PAYMENT_RECORDED, LICENSE_REGENERATED, USER_IMPORTED, ADMIN_LOGIN
  subject_user_id UUID,
  before          JSONB,
  after           JSONB,
  reason          TEXT,                               -- REQUIRED for status changes
  at              TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- Every mutation of user status, subscription, license, or payment
-- writes an audit row IN THE SAME TRANSACTION. No exceptions.

CREATE TABLE import_batches (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id   UUID NOT NULL REFERENCES tenants(id),
  filename    TEXT NOT NULL,
  row_count   INT NOT NULL,
  ok_count    INT NOT NULL,
  error_count INT NOT NULL,
  errors      JSONB,                                  -- [{row, reason}]
  created_by  UUID NOT NULL REFERENCES users(id),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Seeds (spec §3)
INSERT INTO products (code, name, key_prefix) VALUES ('flowos', 'FlowOS', 'FL');
INSERT INTO tenants (slug, name, is_house) VALUES ('flowos', 'FlowOS Direct', TRUE);
INSERT INTO tenant_products (tenant_id, product_id)
  SELECT t.id, p.id FROM tenants t, products p WHERE t.slug = 'flowos';
-- Platform admin user is created at first boot from env (PLATFORM_ADMIN_*),
-- not seeded here: password hashes don't belong in migrations.
