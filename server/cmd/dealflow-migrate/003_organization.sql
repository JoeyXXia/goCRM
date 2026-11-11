ALTER TABLE departments
    ADD COLUMN parent_id BIGINT UNSIGNED NULL,
    ADD CONSTRAINT fk_departments_parent FOREIGN KEY (tenant_id, parent_id)
        REFERENCES departments (tenant_id, id);

ALTER TABLE tenant_memberships
    ADD COLUMN permission_version BIGINT UNSIGNED NOT NULL DEFAULT 1;
