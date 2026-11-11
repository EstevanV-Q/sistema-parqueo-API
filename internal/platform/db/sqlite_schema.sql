-- Esquema local para el API de borde (SQLite).
-- Los IDs se guardan como BLOB de 16 bytes para mantener el mismo contrato que
-- MySQL y permitir que los repositorios compartan las consultas.

CREATE TABLE IF NOT EXISTS users (
    id BLOB PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'operator' CHECK (role IN ('admin','operator','auditor')),
    site_id BLOB NULL,
    active INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_login TIMESTAMP NULL
);
CREATE INDEX IF NOT EXISTS idx_users_site ON users(site_id);

CREATE TABLE IF NOT EXISTS refresh_tokens (
    id BLOB PRIMARY KEY,
    user_id BLOB NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    family_id BLOB NOT NULL,
    replaced_by BLOB NULL,
    expires_at TIMESTAMP NOT NULL,
    revoked_at TIMESTAMP NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (replaced_by) REFERENCES refresh_tokens(id)
);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user ON refresh_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_family ON refresh_tokens(family_id);

CREATE TABLE IF NOT EXISTS idempotency_keys (
    key TEXT PRIMARY KEY,
    user_id BLOB NULL,
    response_body TEXT NULL,
    response_status INTEGER NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_idempotency_expires ON idempotency_keys(expires_at);

CREATE TABLE IF NOT EXISTS audit_log (
    id BLOB PRIMARY KEY,
    user_id BLOB NULL,
    site_id BLOB NULL,
    action TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id BLOB NULL,
    old_values TEXT NULL,
    new_values TEXT NULL,
    ip_address TEXT NULL,
    user_agent TEXT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_audit_user ON audit_log(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_entity ON audit_log(entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_log(created_at);
CREATE INDEX IF NOT EXISTS idx_audit_site ON audit_log(site_id);

CREATE TABLE IF NOT EXISTS sites (
    id BLOB PRIMARY KEY,
    name TEXT NOT NULL,
    code TEXT NOT NULL UNIQUE,
    timezone TEXT NOT NULL DEFAULT 'America/Bogota',
    currency TEXT NOT NULL DEFAULT 'COP',
    capacity_total INTEGER NOT NULL DEFAULT 0,
    is_active INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS zones (
    id BLOB PRIMARY KEY,
    site_id BLOB NOT NULL,
    name TEXT NOT NULL,
    type TEXT NOT NULL DEFAULT 'mixed',
    capacity INTEGER NOT NULL DEFAULT 0,
    is_active INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (site_id) REFERENCES sites(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_zones_site ON zones(site_id);

CREATE TABLE IF NOT EXISTS spaces (
    id BLOB PRIMARY KEY,
    zone_id BLOB NOT NULL,
    code TEXT NOT NULL,
    type TEXT NOT NULL DEFAULT 'mixed',
    status TEXT NOT NULL DEFAULT 'free' CHECK (status IN ('free','occupied','reserved','disabled')),
    is_active INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (zone_id) REFERENCES zones(id) ON DELETE CASCADE,
    UNIQUE (zone_id, code)
);
CREATE INDEX IF NOT EXISTS idx_spaces_zone_status ON spaces(zone_id, status);

CREATE TABLE IF NOT EXISTS vehicle_types (
    id BLOB PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    code TEXT NOT NULL UNIQUE,
    is_active INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS customers (
    id BLOB PRIMARY KEY,
    document TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    phone TEXT NULL,
    email TEXT NULL,
    is_active INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS vehicles (
    id BLOB PRIMARY KEY,
    plate_normalized TEXT NOT NULL UNIQUE,
    plate_raw TEXT NOT NULL,
    type_id BLOB NOT NULL,
    customer_id BLOB NULL,
    is_active INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (type_id) REFERENCES vehicle_types(id),
    FOREIGN KEY (customer_id) REFERENCES customers(id)
);
CREATE INDEX IF NOT EXISTS idx_vehicles_type ON vehicles(type_id);

CREATE TABLE IF NOT EXISTS rate_plans (
    id BLOB PRIMARY KEY,
    site_id BLOB NOT NULL,
    name TEXT NOT NULL,
    vehicle_type_id BLOB NULL,
    currency TEXT NOT NULL DEFAULT 'COP',
    grace_minutes INTEGER NOT NULL DEFAULT 0,
    minimum_cents INTEGER NOT NULL DEFAULT 0,
    fraction_minutes INTEGER NOT NULL DEFAULT 60,
    fraction_cents INTEGER NOT NULL DEFAULT 0,
    daily_cap_cents INTEGER NOT NULL DEFAULT 0,
    lost_ticket_cents INTEGER NOT NULL DEFAULT 0,
    is_active INTEGER NOT NULL DEFAULT 1,
    valid_from TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    valid_until TIMESTAMP NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (site_id) REFERENCES sites(id),
    FOREIGN KEY (vehicle_type_id) REFERENCES vehicle_types(id)
);
CREATE INDEX IF NOT EXISTS idx_rate_plans_site ON rate_plans(site_id);
CREATE INDEX IF NOT EXISTS idx_rate_plans_type ON rate_plans(vehicle_type_id);

CREATE TABLE IF NOT EXISTS rate_plan_tiers (
    id BLOB PRIMARY KEY,
    rate_plan_id BLOB NOT NULL,
    from_minute INTEGER NOT NULL,
    to_minute INTEGER NOT NULL,
    price_cents INTEGER NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (rate_plan_id) REFERENCES rate_plans(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_tiers_plan ON rate_plan_tiers(rate_plan_id);

CREATE TABLE IF NOT EXISTS subscriptions (
    id BLOB PRIMARY KEY,
    customer_id BLOB NOT NULL,
    vehicle_id BLOB NOT NULL,
    site_id BLOB NOT NULL,
    plan_name TEXT NOT NULL,
    price_cents INTEGER NOT NULL,
    starts_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','expired','cancelled')),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (customer_id) REFERENCES customers(id),
    FOREIGN KEY (vehicle_id) REFERENCES vehicles(id),
    FOREIGN KEY (site_id) REFERENCES sites(id)
);
CREATE INDEX IF NOT EXISTS idx_subscriptions_vehicle ON subscriptions(vehicle_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_expires ON subscriptions(expires_at);
CREATE INDEX IF NOT EXISTS idx_subscriptions_site_status ON subscriptions(site_id, status);

CREATE TABLE IF NOT EXISTS tickets (
    id BLOB PRIMARY KEY,
    site_id BLOB NOT NULL,
    zone_id BLOB NULL,
    space_id BLOB NULL,
    vehicle_id BLOB NULL,
    subscription_id BLOB NULL,
    plate_normalized TEXT NOT NULL,
    plate_raw TEXT NOT NULL,
    type_id BLOB NOT NULL,
    rate_plan_id BLOB NULL,
    entry_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    exit_at TIMESTAMP NULL,
    status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','paid','closed','cancelled','lost')),
    total_cents INTEGER NOT NULL DEFAULT 0,
    paid_cents INTEGER NOT NULL DEFAULT 0,
    is_lost INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (site_id) REFERENCES sites(id),
    FOREIGN KEY (zone_id) REFERENCES zones(id),
    FOREIGN KEY (space_id) REFERENCES spaces(id),
    FOREIGN KEY (vehicle_id) REFERENCES vehicles(id),
    FOREIGN KEY (type_id) REFERENCES vehicle_types(id),
    FOREIGN KEY (rate_plan_id) REFERENCES rate_plans(id),
    FOREIGN KEY (subscription_id) REFERENCES subscriptions(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_tickets_site_status ON tickets(site_id, status);
CREATE INDEX IF NOT EXISTS idx_tickets_space ON tickets(space_id);
CREATE INDEX IF NOT EXISTS idx_tickets_plate_site ON tickets(plate_normalized, site_id);
CREATE INDEX IF NOT EXISTS idx_tickets_entry ON tickets(entry_at);
CREATE INDEX IF NOT EXISTS idx_tickets_subscription ON tickets(subscription_id);
CREATE UNIQUE INDEX IF NOT EXISTS uk_open_ticket_space ON tickets(space_id)
    WHERE status = 'open' AND space_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uk_open_ticket_plate_site ON tickets(plate_normalized, site_id)
    WHERE status = 'open';

CREATE TABLE IF NOT EXISTS ticket_events (
    id BLOB PRIMARY KEY,
    ticket_id BLOB NOT NULL,
    event_type TEXT NOT NULL,
    actor_user_id BLOB NULL,
    details TEXT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (ticket_id) REFERENCES tickets(id) ON DELETE CASCADE,
    FOREIGN KEY (actor_user_id) REFERENCES users(id)
);
CREATE INDEX IF NOT EXISTS idx_ticket_events_ticket ON ticket_events(ticket_id);
CREATE INDEX IF NOT EXISTS idx_ticket_events_created ON ticket_events(created_at);

CREATE TABLE IF NOT EXISTS payments (
    id BLOB PRIMARY KEY,
    ticket_id BLOB NULL,
    subscription_id BLOB NULL,
    amount_cents INTEGER NOT NULL,
    currency TEXT NOT NULL DEFAULT 'COP',
    method TEXT NOT NULL DEFAULT 'cash' CHECK (method IN ('cash','card','transfer','abonnement','waived')),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','completed','failed','refunded')),
    reference TEXT NULL,
    cashier_user_id BLOB NULL,
    idempotency_key TEXT NULL UNIQUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (ticket_id) REFERENCES tickets(id),
    FOREIGN KEY (subscription_id) REFERENCES subscriptions(id),
    FOREIGN KEY (cashier_user_id) REFERENCES users(id)
);
CREATE INDEX IF NOT EXISTS idx_payments_ticket ON payments(ticket_id);
CREATE INDEX IF NOT EXISTS idx_payments_created ON payments(created_at);

CREATE TABLE IF NOT EXISTS devices (
    id BLOB PRIMARY KEY,
    site_id BLOB NOT NULL,
    name TEXT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('barrier','lpr','rfid','printer')),
    api_key_hash TEXT NOT NULL,
    is_active INTEGER NOT NULL DEFAULT 1,
    last_heartbeat TIMESTAMP NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (site_id) REFERENCES sites(id)
);
CREATE INDEX IF NOT EXISTS idx_devices_site ON devices(site_id);
CREATE INDEX IF NOT EXISTS idx_devices_api_key ON devices(api_key_hash);

CREATE TABLE IF NOT EXISTS device_events (
    id BLOB PRIMARY KEY,
    device_id BLOB NOT NULL,
    event_id TEXT NOT NULL UNIQUE,
    event_type TEXT NOT NULL,
    plate_normalized TEXT NULL,
    command TEXT NULL,
    decision TEXT NOT NULL CHECK (decision IN ('allow','deny')),
    reason TEXT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (device_id) REFERENCES devices(id)
);
CREATE INDEX IF NOT EXISTS idx_device_events_device ON device_events(device_id);
CREATE INDEX IF NOT EXISTS idx_device_events_created ON device_events(created_at);

-- Outbox transaccional: cada operación local deja un evento para sincronizar.
CREATE TABLE IF NOT EXISTS outbox (
    id BLOB PRIMARY KEY,
    aggregate_type TEXT NOT NULL,
    aggregate_id BLOB NOT NULL,
    event_type TEXT NOT NULL,
    payload TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    attempts INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMP NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','sent','failed')),
    last_error TEXT NULL
);
CREATE INDEX IF NOT EXISTS idx_outbox_pending ON outbox(status, next_attempt_at, created_at);
CREATE INDEX IF NOT EXISTS idx_outbox_aggregate ON outbox(aggregate_type, aggregate_id);

-- Estado de sincronización para cursores de descarga (catálogo, tarifas, etc.).
CREATE TABLE IF NOT EXISTS sync_state (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
