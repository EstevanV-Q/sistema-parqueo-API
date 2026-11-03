# Parking Backend

Backend del Sistema de Control de Parqueo/Estacionamiento — Proyecto Universitario.

## Stack

- **Go 1.27** + `chi` (router HTTP)
- **MySQL 8** (base de datos principal)
- **JWT** (access 15 min + refresh rotativo, Argon2id)
- **Docker Compose** + Caddy (despliegue)
- **UUIDv7** (ordenables por tiempo, BINARY(16))
- **Dinero en enteros** (centavos, nunca float)

## Estructura

```
cmd/
  api/          Punto de entrada de la API
  migrate/      Migraciones goose (up/down/status)
  seed/         Datos de demostración
internal/
  auth/         Usuarios, roles, login, refresh, logout, me
  catalog/      Sedes, zonas, espacios, tipos de vehículo
  parking/      Tickets, entrada/salida, motor de tarifas
  parking/pricing/  Cálculo puro de tarifas (gracia, mínimo, fracción, tope)
  billing/      Pagos, recibos internos, idempotencia
  subscriptions/# Abonados/mensualidades (validación por placa)
  vehicles/     Vehículos (placa normalizada, tipo)
  customers/    Clientes (documento, nombre, contacto)
  rateplans/    Tarifas por sede y tipo de vehículo
  devices/      Dispositivos (barrera virtual, eventos HMAC)
  realtime/     Ocupación por WebSocket (salas por sede)
  reporting/    Ocupación, ingresos, caja diaria
  platform/     DB, logger, seguridad, JWT, errores, IDs, contexto
  shared/       Money (centavos), Plate (normalización)
migrations/     Migraciones SQL versionadas (goose)
deploy/         Caddyfile (TLS)
```

## Requisitos previos

- Go 1.27+
- MySQL 8 (o Docker Compose)
- Docker (opcional, para contenedores)

## Ejecución local

```bash
# 1. Copiar variables de entorno
cp .env.example .env   # o usar .env ya creado

# 2. Levantar MySQL
docker compose up -d mysql

# 3. Ejecutar migraciones
go run cmd/migrate/main.go up

# 4. Cargar datos de demostración
go run cmd/seed/main.go

# 5. Ejecutar la API
go run ./cmd/api
```

La API escucha en `http://localhost:8080`.

## Endpoints principales

### Auth (público, excepto register)
- `POST /api/v1/auth/login` — iniciar sesión
- `POST /api/v1/auth/refresh` — renovar access token
- `POST /api/v1/auth/logout` — cerrar sesión
- `GET /api/v1/auth/me` — usuario autenticado
- `POST /api/v1/auth/register` — crear usuario (solo admin)

### Parqueo (tickets)
- `POST /api/v1/tickets` — entrada de vehículo
- `POST /api/v1/tickets/{id}/quote` — cotizar salida
- `POST /api/v1/tickets/{id}/settle` — pagar y salir
- `POST /api/v1/tickets/{id}/cancel` — cancelar ticket
- `GET /api/v1/tickets?site_id=&status=` — listar tickets

### Pagos
- `POST /api/v1/payments` — registrar pago
- `GET /api/v1/payments` — listar pagos
- `GET /api/v1/payments/{id}` — obtener pago

### Abonados
- `POST /api/v1/subscriptions` — crear abono
- `GET /api/v1/subscriptions/validate?vehicle_id=&site_id=` — validar abono

### Catálogo
- `POST /api/v1/catalog/sites` — crear sede
- `GET /api/v1/catalog/sites` — listar sedes
- `POST /api/v1/catalog/zones` — crear zona
- `POST /api/v1/catalog/spaces` — crear espacio
- `POST /api/v1/catalog/vehicle-types` — crear tipo de vehículo
- `POST /api/v1/catalog/rate-plans` — crear tarifa

### Reportes
- `GET /api/v1/reports/occupancy?site_id=` — ocupación en tiempo real
- `GET /api/v1/reports/revenue?site_id=&from=&to=` — ingresos por día
- `GET /api/v1/reports/cash-daily?site_id=&date=` — caja diaria

### Dispositivos (barrera virtual simulada)
- `POST /api/v1/devices` — registrar dispositivo
- `POST /api/v1/devices/events` — evento de barrera/LPR/RFID
- `GET /api/v1/devices` — listar dispositivos

### Tiempo real
- `GET /ws/occupancy?site_id=` — WebSocket de ocupación

### Salud
- `GET /healthz` — liveness
- `GET /readyz` — readiness (incluye DB)

## Datos de demostración (seed)

Después de `go run cmd/seed/main.go`:

| Usuario | Contraseña | Rol |
|---|---|---|
| admin | Admin123! | admin |
| operador | Oper123! | operator |

- **Sede**: Centro Comercial Plaza (PLAZA-01)
- **Espacios**: A-01 a A-10 (zona General - Carros)
- **Vehículo abonado**: XYZ-987 (plan Mensual Carro, 30 días)
- **Dispositivo**: Barrera Principal (API key: `device-api-key-001`)

## Arquitectura

Capas estrictas: `handler → service → repository (interfaz) → MySQL`.
El dominio y el motor de tarifas son puros (sin dependencias de infraestructura).

## Próximos pasos (pendiente)

- Swagger/OpenAPI (`/swagger/*`)
- Tests de integración con MySQL real (testcontainers)
- Frontend (React/Vue/Go templates)
- Simulación offline (cola SQLite + sincronización)
- Auditoría completa (`audit_log`)
- CI (lint + test + build)

## Licencia

Uso académico — Proyecto universitario.