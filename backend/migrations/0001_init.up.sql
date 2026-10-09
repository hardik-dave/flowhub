-- FlowOS Hub initial schema (SPEC.md §3, MySQL 8.0 per the 2026-10-08 amendment).
-- Every query is tenant-scoped; money is integer paise; status changes are audited.

CREATE TABLE tenants (
    id                  BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    slug                VARCHAR(64) NOT NULL UNIQUE,
    name                TEXT NOT NULL,
    contact_person      TEXT,
    contact_no          TEXT,
    start_date          DATE NOT NULL,
    end_date            DATE,
    is_house            BOOLEAN NOT NULL DEFAULT FALSE,
    grace_working_days  INT NOT NULL DEFAULT 5 CHECK (grace_working_days BETWEEN 0 AND 30),
    status              VARCHAR(20) NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','INACTIVE')),
    created_by          BIGINT,
    updated_by          BIGINT,
    created_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE products (
    id          BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    code        VARCHAR(64) NOT NULL UNIQUE,
    name        TEXT NOT NULL,
    key_prefix  VARCHAR(8) NOT NULL,
    status      VARCHAR(20) NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','RETIRED')),
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE tenant_products (
    id          BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    tenant_id   BIGINT NOT NULL,
    product_id  BIGINT NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (tenant_id, product_id),
    FOREIGN KEY (tenant_id) REFERENCES tenants(id),
    FOREIGN KEY (product_id) REFERENCES products(id)
);

CREATE TABLE users (
    id                  BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    username            VARCHAR(64) NOT NULL UNIQUE,
    password_hash       TEXT NOT NULL,
    first_name          TEXT,
    last_name           TEXT,
    contact_no          VARCHAR(20),
    mobile_verified     BOOLEAN NOT NULL DEFAULT FALSE,
    email               VARCHAR(191),
    city                TEXT,
    state               TEXT,
    broker_client_code  TEXT,
    referral_code       TEXT,
    status              VARCHAR(20) NOT NULL DEFAULT 'PENDING_VERIFICATION'
                        CHECK (status IN ('PENDING_VERIFICATION','ACTIVE','DEACTIVATED','BANNED')),
    token_version       INT NOT NULL DEFAULT 1,
    imported            BOOLEAN NOT NULL DEFAULT FALSE,
    import_batch_id     BIGINT,
    created_by          BIGINT,
    updated_by          BIGINT,
    created_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE tenant_users (
    id          BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    tenant_id   BIGINT NOT NULL,
    user_id     BIGINT NOT NULL,
    role        TEXT NOT NULL CHECK (role IN ('SUPERADMIN','ADMIN','EDITOR','ALGO_USER')),
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (tenant_id, user_id),
    FOREIGN KEY (tenant_id) REFERENCES tenants(id),
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE licenses (
    id                       BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    tenant_id                BIGINT NOT NULL,
    user_id                  BIGINT NOT NULL,
    product_id               BIGINT NOT NULL,
    license_key_hash         TEXT NOT NULL,
    license_key_hint         VARCHAR(4) NOT NULL,
    status                   VARCHAR(20) NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','DISABLED')),
    subscription_valid_until DATE,
    entitlements             JSON NOT NULL DEFAULT (CAST('{"max_activations":1,"tier":"RETAIL"}' AS JSON)),
    created_at               DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at               DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE (tenant_id, user_id, product_id),
    FOREIGN KEY (tenant_id) REFERENCES tenants(id),
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (product_id) REFERENCES products(id)
);

CREATE TABLE payments (
    id                  BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    tenant_id           BIGINT NOT NULL,
    user_id             BIGINT NOT NULL,
    license_id          BIGINT NOT NULL,
    amount_minor_units  BIGINT NOT NULL CHECK (amount_minor_units >= 0),
    currency            VARCHAR(3) NOT NULL DEFAULT 'INR',
    method              TEXT NOT NULL CHECK (method IN ('MANUAL','COMPLIMENTARY','GATEWAY')),
    gateway_payment_id  TEXT,
    plan                TEXT NOT NULL CHECK (plan IN ('MONTHLY','QUARTERLY','ANNUAL')),
    paid_at             DATE NOT NULL,
    valid_from          DATE NOT NULL,
    valid_until         DATE NOT NULL,
    recorded_by         BIGINT NOT NULL,
    note                TEXT,
    created_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES tenants(id),
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (license_id) REFERENCES licenses(id)
);

CREATE TABLE otp_codes (
    id          BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id     BIGINT NOT NULL,
    purpose     TEXT NOT NULL CHECK (purpose IN ('REGISTRATION','FIRST_LOGIN','PASSWORD_RESET')),
    code_hash   TEXT NOT NULL,
    expires_at  DATETIME NOT NULL,
    attempts    INT NOT NULL DEFAULT 0,
    consumed_at DATETIME,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE sessions (
    id          BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id     BIGINT NOT NULL,
    audience    TEXT NOT NULL CHECK (audience IN ('DASHBOARD','APP')),
    token_hash  VARCHAR(64) NOT NULL,
    expires_at  DATETIME NOT NULL,
    ip          VARCHAR(45),
    user_agent  TEXT,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE audit_log (
    id              BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    tenant_id       BIGINT NOT NULL,
    actor_user_id   BIGINT,
    action          TEXT NOT NULL,
    subject_user_id BIGINT,
    `before`        JSON,
    `after`         JSON,
    reason          TEXT,
    at              DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE import_batches (
    id          BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    tenant_id   BIGINT NOT NULL,
    filename    TEXT NOT NULL,
    row_count   INT NOT NULL,
    ok_count    INT NOT NULL,
    error_count INT NOT NULL,
    errors      JSON,
    created_by  BIGINT NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES tenants(id)
);

-- Seed (SPEC.md §3): product flowos, house tenant id 1, grant.
INSERT INTO products (code, name, key_prefix) VALUES ('flowos', 'FlowOS', 'FL');

INSERT INTO tenants (id, slug, name, start_date, is_house, status)
VALUES (1, 'flowos', 'FlowOS Direct', CURDATE(), TRUE, 'ACTIVE');

INSERT INTO tenant_products (tenant_id, product_id)
SELECT 1, id FROM products WHERE code = 'flowos';
