-- 2026-10-10 · Rename the end-user role ALGO_USER → USER and drop the
-- unused EDITOR role (SPEC §11 bans EDITOR in V1). MySQL names the inline
-- CHECK from 0001 as tenant_users_chk_1. Existing memberships are
-- migrated in place — the data update must happen while no CHECK
-- constrains the column (between the drop and the re-add).
ALTER TABLE tenant_users DROP CHECK tenant_users_chk_1;
UPDATE tenant_users SET role = 'USER' WHERE role = 'ALGO_USER';
ALTER TABLE tenant_users ADD CONSTRAINT tenant_users_chk_role
    CHECK (role IN ('SUPERADMIN','ADMIN','USER'));
