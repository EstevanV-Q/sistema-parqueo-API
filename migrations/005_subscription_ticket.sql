-- +goose Up
ALTER TABLE tickets
    ADD COLUMN subscription_id BINARY(16) NULL AFTER vehicle_id,
    ADD CONSTRAINT fk_tickets_subscription FOREIGN KEY (subscription_id) REFERENCES subscriptions(id) ON DELETE SET NULL,
    ADD INDEX idx_tickets_subscription (subscription_id);

-- +goose Down
ALTER TABLE tickets
    DROP FOREIGN KEY fk_tickets_subscription,
    DROP INDEX idx_tickets_subscription,
    DROP COLUMN subscription_id;

