CREATE TABLE IF NOT EXISTS tenants (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    name VARCHAR(128) NOT NULL,
    status VARCHAR(16) NOT NULL,
    UNIQUE KEY uq_tenants_code (code)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS departments (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    tenant_id BIGINT UNSIGNED NOT NULL,
    code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    name VARCHAR(128) NOT NULL,
    UNIQUE KEY uq_departments_tenant_id (tenant_id, id),
    UNIQUE KEY uq_departments_tenant_code (tenant_id, code),
    CONSTRAINT fk_departments_tenant FOREIGN KEY (tenant_id) REFERENCES tenants (id)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS tenant_memberships (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    tenant_id BIGINT UNSIGNED NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    department_id BIGINT UNSIGNED NOT NULL,
    role_key VARCHAR(32) NOT NULL,
    status VARCHAR(16) NOT NULL,
    UNIQUE KEY uq_memberships_tenant_user (tenant_id, user_id),
    CONSTRAINT fk_memberships_tenant FOREIGN KEY (tenant_id) REFERENCES tenants (id),
    CONSTRAINT fk_memberships_user FOREIGN KEY (user_id) REFERENCES sys_users (id),
    CONSTRAINT fk_memberships_department FOREIGN KEY (tenant_id, department_id)
        REFERENCES departments (tenant_id, id)
) ENGINE=InnoDB;
