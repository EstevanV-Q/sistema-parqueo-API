-- +goose Up
-- details guarda texto operativo, no JSON. El tipo JSON rechazaba los valores
-- planos usados por los eventos de entrada, salida y cancelación.
ALTER TABLE ticket_events
    MODIFY COLUMN details TEXT NULL;

-- +goose Down
UPDATE ticket_events SET details = NULL WHERE details IS NOT NULL AND JSON_VALID(details) = 0;
ALTER TABLE ticket_events
    MODIFY COLUMN details JSON NULL;
