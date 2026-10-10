-- Revert 0003: restore the pre-rename role vocabulary.
ALTER TABLE tenant_users DROP CHECK tenant_users_chk_role;
UPDATE tenant_users SET role = 'ALGO_USER' WHERE role = 'USER';
ALTER TABLE tenant_users ADD CONSTRAINT tenant_users_chk_role
    CHECK (role IN ('SUPERADMIN','ADMIN','EDITOR','ALGO_USER'));
