-- +goose Up
CREATE TABLE IF NOT EXISTS sites (
    id BINARY(16) PRIMARY KEY,
    name VARCHAR(200) NOT NULL,
    code VARCHAR(20) NOT NULL UNIQUE,
    timezone VARCHAR(60) NOT NULL DEFAULT 'America/Bogota',
    currency VARCHAR(3) NOT NULL DEFAULT 'COP',
    capacity_total INT NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS zones (
    id BINARY(16) PRIMARY KEY,
    site_id BINARY(16) NOT NULL,
    name VARCHAR(100) NOT NULL,
    type ENUM('car','moto','bike','mixed') NOT NULL DEFAULT 'mixed',
    capacity INT NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (site_id) REFERENCES sites(id) ON DELETE CASCADE,
    INDEX idx_zones_site (site_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS spaces (
    id BINARY(16) PRIMARY KEY,
    zone_id BINARY(16) NOT NULL,
    code VARCHAR(20) NOT NULL,
    type ENUM('car','moto','bike','mixed') NOT NULL DEFAULT 'mixed',
    status ENUM('free','occupied','reserved','disabled') NOT NULL DEFAULT 'free',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (zone_id) REFERENCES zones(id) ON DELETE CASCADE,
    UNIQUE KEY uk_spaces_zone_code (zone_id, code),
    INDEX idx_spaces_zone_status (zone_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS vehicle_types (
    id BINARY(16) PRIMARY KEY,
    name VARCHAR(50) NOT NULL UNIQUE,
    code VARCHAR(10) NOT NULL UNIQUE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE IF EXISTS spaces;
DROP TABLE IF EXISTS zones;
DROP TABLE IF EXISTS vehicle_types;
DROP TABLE IF EXISTS sites;