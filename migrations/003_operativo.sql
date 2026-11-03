-- +goose Up
CREATE TABLE IF NOT EXISTS customers (
    id BINARY(16) PRIMARY KEY,
    document VARCHAR(50) NOT NULL UNIQUE,
    name VARCHAR(200) NOT NULL,
    phone VARCHAR(30) NULL,
    email VARCHAR(255) NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_customers_document (document)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS vehicles (
    id BINARY(16) PRIMARY KEY,
    plate_normalized VARCHAR(20) NOT NULL,
    plate_raw VARCHAR(50) NOT NULL,
    type_id BINARY(16) NOT NULL,
    customer_id BINARY(16) NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (type_id) REFERENCES vehicle_types(id),
    FOREIGN KEY (customer_id) REFERENCES customers(id),
    UNIQUE KEY uk_vehicles_plate (plate_normalized),
    INDEX idx_vehicles_type (type_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS rate_plans (
    id BINARY(16) PRIMARY KEY,
    site_id BINARY(16) NOT NULL,
    name VARCHAR(100) NOT NULL,
    vehicle_type_id BINARY(16) NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'COP',
    grace_minutes INT NOT NULL DEFAULT 0,
    minimum_cents BIGINT NOT NULL DEFAULT 0,
    fraction_minutes INT NOT NULL DEFAULT 60,
    fraction_cents BIGINT NOT NULL DEFAULT 0,
    daily_cap_cents BIGINT NOT NULL DEFAULT 0,
    lost_ticket_cents BIGINT NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    valid_from TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    valid_until TIMESTAMP NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (site_id) REFERENCES sites(id),
    FOREIGN KEY (vehicle_type_id) REFERENCES vehicle_types(id),
    INDEX idx_rate_plans_site (site_id),
    INDEX idx_rate_plans_type (vehicle_type_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS rate_plan_tiers (
    id BINARY(16) PRIMARY KEY,
    rate_plan_id BINARY(16) NOT NULL,
    from_minute INT NOT NULL,
    to_minute INT NOT NULL,
    price_cents BIGINT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (rate_plan_id) REFERENCES rate_plans(id) ON DELETE CASCADE,
    INDEX idx_tiers_plan (rate_plan_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Tickets
CREATE TABLE IF NOT EXISTS tickets (
    id BINARY(16) PRIMARY KEY,
    site_id BINARY(16) NOT NULL,
    zone_id BINARY(16) NULL,
    space_id BINARY(16) NULL,
    vehicle_id BINARY(16) NULL,
    plate_normalized VARCHAR(20) NOT NULL,
    plate_raw VARCHAR(50) NOT NULL,
    type_id BINARY(16) NOT NULL,
    rate_plan_id BINARY(16) NULL,
    entry_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    exit_at TIMESTAMP NULL,
    status ENUM('open','paid','closed','cancelled','lost') NOT NULL DEFAULT 'open',
    total_cents BIGINT NOT NULL DEFAULT 0,
    paid_cents BIGINT NOT NULL DEFAULT 0,
    is_lost BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    -- Columnas generadas para invariantes de un solo ticket abierto
    open_ticket_space BINARY(16) GENERATED ALWAYS AS (CASE WHEN status = 'open' THEN space_id END) VIRTUAL,
    open_ticket_plate_site VARCHAR(60) GENERATED ALWAYS AS (CASE WHEN status = 'open' THEN CONCAT(plate_normalized, '-', HEX(site_id)) END) VIRTUAL,
    FOREIGN KEY (site_id) REFERENCES sites(id),
    FOREIGN KEY (zone_id) REFERENCES zones(id),
    FOREIGN KEY (space_id) REFERENCES spaces(id),
    FOREIGN KEY (vehicle_id) REFERENCES vehicles(id),
    FOREIGN KEY (type_id) REFERENCES vehicle_types(id),
    FOREIGN KEY (rate_plan_id) REFERENCES rate_plans(id),
    INDEX idx_tickets_site_status (site_id, status),
    INDEX idx_tickets_space (space_id),
    INDEX idx_tickets_plate_site (plate_normalized, site_id),
    INDEX idx_tickets_entry (entry_at),
    UNIQUE KEY uk_open_ticket_space (open_ticket_space),
    UNIQUE KEY uk_open_ticket_plate_site (open_ticket_plate_site)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS ticket_events (
    id BINARY(16) PRIMARY KEY,
    ticket_id BINARY(16) NOT NULL,
    event_type VARCHAR(50) NOT NULL,
    actor_user_id BINARY(16) NULL,
    details JSON NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (ticket_id) REFERENCES tickets(id) ON DELETE CASCADE,
    FOREIGN KEY (actor_user_id) REFERENCES users(id),
    INDEX idx_ticket_events_ticket (ticket_id),
    INDEX idx_ticket_events_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Abonados (simplificado: vehículo con vigencia mensual)
-- Se crea antes de payments porque payments referencia subscriptions.
CREATE TABLE IF NOT EXISTS subscriptions (
    id BINARY(16) PRIMARY KEY,
    customer_id BINARY(16) NOT NULL,
    vehicle_id BINARY(16) NOT NULL,
    site_id BINARY(16) NOT NULL,
    plan_name VARCHAR(100) NOT NULL,
    price_cents BIGINT NOT NULL,
    starts_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    status ENUM('active','expired','cancelled') NOT NULL DEFAULT 'active',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (customer_id) REFERENCES customers(id),
    FOREIGN KEY (vehicle_id) REFERENCES vehicles(id),
    FOREIGN KEY (site_id) REFERENCES sites(id),
    INDEX idx_subscriptions_vehicle (vehicle_id),
    INDEX idx_subscriptions_expires (expires_at),
    INDEX idx_subscriptions_site_status (site_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Pagos
CREATE TABLE IF NOT EXISTS payments (
    id BINARY(16) PRIMARY KEY,
    ticket_id BINARY(16) NULL,
    subscription_id BINARY(16) NULL,
    amount_cents BIGINT NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'COP',
    method ENUM('cash','card','transfer','abonnement','waived') NOT NULL DEFAULT 'cash',
    status ENUM('pending','completed','failed','refunded') NOT NULL DEFAULT 'pending',
    reference VARCHAR(100) NULL,
    cashier_user_id BINARY(16) NULL,
    idempotency_key VARCHAR(128) NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (ticket_id) REFERENCES tickets(id),
    FOREIGN KEY (subscription_id) REFERENCES subscriptions(id),
    FOREIGN KEY (cashier_user_id) REFERENCES users(id),
    INDEX idx_payments_ticket (ticket_id),
    INDEX idx_payments_idempotency (idempotency_key),
    INDEX idx_payments_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Dispositivos
CREATE TABLE IF NOT EXISTS devices (
    id BINARY(16) PRIMARY KEY,
    site_id BINARY(16) NOT NULL,
    name VARCHAR(100) NOT NULL,
    type ENUM('barrier','lpr','rfid','printer') NOT NULL,
    api_key_hash VARCHAR(500) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    last_heartbeat TIMESTAMP NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (site_id) REFERENCES sites(id),
    INDEX idx_devices_site (site_id),
    INDEX idx_devices_api_key (api_key_hash(100))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS device_events (
    id BINARY(16) PRIMARY KEY,
    device_id BINARY(16) NOT NULL,
    event_id VARCHAR(128) NOT NULL,
    event_type VARCHAR(50) NOT NULL,
    plate_normalized VARCHAR(20) NULL,
    command VARCHAR(20) NULL,
    decision ENUM('allow','deny') NOT NULL,
    reason VARCHAR(200) NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (device_id) REFERENCES devices(id),
    INDEX idx_device_events_device (device_id),
    INDEX idx_device_events_event_id (event_id),
    INDEX idx_device_events_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE IF EXISTS device_events;
DROP TABLE IF EXISTS devices;
DROP TABLE IF EXISTS subscriptions;
DROP TABLE IF EXISTS payments;
DROP TABLE IF EXISTS ticket_events;
DROP TABLE IF EXISTS tickets;
DROP TABLE IF EXISTS rate_plan_tiers;
DROP TABLE IF EXISTS rate_plans;
DROP TABLE IF EXISTS vehicles;
DROP TABLE IF EXISTS customers;