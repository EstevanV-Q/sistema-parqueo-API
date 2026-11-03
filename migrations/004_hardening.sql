-- +goose Up
-- Integridad referencial diferida: users se creó antes que sites.
ALTER TABLE users
    ADD CONSTRAINT fk_users_site FOREIGN KEY (site_id) REFERENCES sites(id) ON DELETE SET NULL;

-- Un evento de dispositivo no puede procesarse dos veces.
ALTER TABLE device_events
    DROP INDEX idx_device_events_event_id,
    ADD UNIQUE KEY uk_device_events_event_id (event_id);

-- El hash del refresh token es único (permite búsqueda y evita duplicados).
ALTER TABLE refresh_tokens
    DROP INDEX idx_refresh_tokens_hash,
    ADD UNIQUE KEY uk_refresh_tokens_hash (token_hash(100));

-- La clave de idempotencia de pagos debe ser única.
ALTER TABLE payments
    DROP INDEX idx_payments_idempotency,
    ADD UNIQUE KEY uk_payments_idempotency (idempotency_key);

-- +goose Down
ALTER TABLE payments
    DROP INDEX uk_payments_idempotency,
    ADD INDEX idx_payments_idempotency (idempotency_key);

ALTER TABLE refresh_tokens
    DROP INDEX uk_refresh_tokens_hash,
    ADD INDEX idx_refresh_tokens_hash (token_hash(100));

ALTER TABLE device_events
    DROP INDEX uk_device_events_event_id,
    ADD INDEX idx_device_events_event_id (event_id);

ALTER TABLE users
    DROP FOREIGN KEY fk_users_site;
