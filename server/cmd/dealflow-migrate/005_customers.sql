CREATE TABLE IF NOT EXISTS crm_customers (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    tenant_id BIGINT UNSIGNED NOT NULL,
    code VARCHAR(38) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    name VARCHAR(128) NOT NULL,
    owner_id BIGINT UNSIGNED NOT NULL,
    department_id BIGINT UNSIGNED NOT NULL,
    industry VARCHAR(64) NOT NULL,
    remark VARCHAR(512) NOT NULL,
    UNIQUE KEY uq_customers_tenant_id (tenant_id, id),
    UNIQUE KEY uq_customers_tenant_code (tenant_id, code),
    KEY idx_customers_owner (tenant_id, owner_id, id),
    KEY idx_customers_department (tenant_id, department_id, id),
    KEY idx_customers_name (tenant_id, name, id),
    CONSTRAINT fk_customers_tenant FOREIGN KEY (tenant_id) REFERENCES tenants (id),
    CONSTRAINT fk_customers_owner FOREIGN KEY (tenant_id, owner_id) REFERENCES tenant_memberships (tenant_id, user_id),
    CONSTRAINT fk_customers_department FOREIGN KEY (tenant_id, department_id) REFERENCES departments (tenant_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS crm_contacts (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    tenant_id BIGINT UNSIGNED NOT NULL,
    customer_id BIGINT UNSIGNED NOT NULL,
    name VARCHAR(128) NOT NULL,
    phone VARCHAR(32) NOT NULL,
    email VARCHAR(254) NOT NULL,
    title VARCHAR(64) NOT NULL,
    KEY idx_contacts_customer (tenant_id, customer_id, id),
    CONSTRAINT fk_contacts_customer FOREIGN KEY (tenant_id, customer_id) REFERENCES crm_customers (tenant_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
