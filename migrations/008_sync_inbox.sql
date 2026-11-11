-- +goose Up
-- Bandeja de entrada idempotente para eventos enviados por las APIs de borde.
CREATE TABLE IF NOT EXISTS sync_inbox (
    id BINARY(16) PRIMARY KEY,
    site_id BINARY(16) NOT NULL,
    aggregate_type VARCHAR(50) NOT NULL,
    aggregate_id BINARY(16) NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    payload MEDIUMTEXT NOT NULL,
    event_created_at TIMESTAMP NOT NULL,
    received_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    processed_at TIMESTAMP NULL,
    attempts INT NOT NULL DEFAULT 0,
    last_error TEXT NULL,
    FOREIGN KEY (site_id) REFERENCES sites(id),
    INDEX idx_sync_inbox_pending (processed_at, received_at),
    INDEX idx_sync_inbox_site (site_id, received_at),
    INDEX idx_sync_inbox_aggregate (aggregate_type, aggregate_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE IF EXISTS sync_inbox;
