-- FlowHub product catalogue seed (SPEC.md §3 'Seed data'; multi-product v1.1).
-- 0001 seeded only `flowos`; this migration completes the catalogue the
-- dashboard offers when granting tenants and issuing licenses. Idempotent so
-- it is safe on databases where 0001 already ran.

INSERT INTO products (code, name, key_prefix, status) VALUES
    ('flowos',       'FlowOS',        'FL', 'ACTIVE'),
    ('optionalyzer', 'Optionalyzer',  'OP', 'ACTIVE'),
    ('dhansanketai', 'DhanSanket AI', 'DS', 'ACTIVE'),
    ('pashutrack',   'PashuTrack',    'PT', 'ACTIVE'),
    ('teleflow',     'TeleFlow',      'TF', 'ACTIVE')
ON DUPLICATE KEY UPDATE
    name = VALUES(name),
    key_prefix = VALUES(key_prefix),
    status = 'ACTIVE';
