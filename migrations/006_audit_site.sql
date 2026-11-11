-- +goose Up
ALTER TABLE audit_log
    ADD COLUMN site_id BINARY(16) NULL AFTER user_id,
    ADD KEY idx_audit_site_created (site_id, created_at);

-- +goose Down
ALTER TABLE audit_log
    DROP INDEX idx_audit_site_created,
    DROP COLUMN site_id;
