ALTER TABLE crm_customers ADD COLUMN next_followup_at DATETIME(3) NULL,
    ADD KEY idx_customers_followup (tenant_id, next_followup_at, id);

CREATE TABLE IF NOT EXISTS crm_followups (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    tenant_id BIGINT UNSIGNED NOT NULL,
    customer_id BIGINT UNSIGNED NOT NULL,
    actor_id BIGINT UNSIGNED NOT NULL,
    content VARCHAR(1024) NOT NULL,
    created_at DATETIME(3) NOT NULL,
    KEY idx_followups_customer (tenant_id, customer_id, created_at, id),
    CONSTRAINT fk_followups_customer FOREIGN KEY (tenant_id, customer_id) REFERENCES crm_customers (tenant_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS crm_audit_events (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    tenant_id BIGINT UNSIGNED NOT NULL,
    customer_id BIGINT UNSIGNED NOT NULL,
    actor_id BIGINT UNSIGNED NOT NULL,
    action VARCHAR(32) NOT NULL,
    subject_id BIGINT UNSIGNED NOT NULL,
    from_owner_id BIGINT UNSIGNED NOT NULL,
    to_owner_id BIGINT UNSIGNED NOT NULL,
    from_department_id BIGINT UNSIGNED NOT NULL,
    to_department_id BIGINT UNSIGNED NOT NULL,
    request_id VARCHAR(36) NOT NULL,
    created_at DATETIME(3) NOT NULL,
    KEY idx_audit_customer (tenant_id, customer_id, created_at, id),
    CONSTRAINT fk_audit_tenant FOREIGN KEY (tenant_id) REFERENCES tenants (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
