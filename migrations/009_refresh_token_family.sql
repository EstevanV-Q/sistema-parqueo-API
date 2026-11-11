-- +goose Up
ALTER TABLE refresh_tokens
    ADD COLUMN family_id BINARY(16) NULL,
    ADD COLUMN replaced_by BINARY(16) NULL;

UPDATE refresh_tokens SET family_id = id WHERE family_id IS NULL;

ALTER TABLE refresh_tokens
    MODIFY family_id BINARY(16) NOT NULL,
    ADD INDEX idx_refresh_tokens_family (family_id),
    ADD CONSTRAINT fk_refresh_tokens_replaced_by FOREIGN KEY (replaced_by) REFERENCES refresh_tokens(id);

-- +goose Down
ALTER TABLE refresh_tokens
    DROP FOREIGN KEY fk_refresh_tokens_replaced_by,
    DROP INDEX idx_refresh_tokens_family,
    MODIFY family_id BINARY(16) NULL,
    DROP COLUMN family_id,
    DROP COLUMN replaced_by;
