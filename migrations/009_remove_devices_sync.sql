-- +goose Up
-- Se retiran los módulos de dispositivos y sincronización offline.
DROP TABLE IF EXISTS sync_inbox;
DROP TABLE IF EXISTS device_events;
DROP TABLE IF EXISTS devices;

-- +goose Down
-- Para recuperarlas, revertir hasta 008 y volver a aplicar 003/008.
